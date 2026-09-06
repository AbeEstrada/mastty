package utils

import (
	"bytes"
	"container/list"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/constants"
)

const (
	maxRawImages    = 150
	maxScaledImages = 300
	maxImageBytes   = 20 << 20
	downloadWorkers = 4
	downloadTimeout = 15 * time.Second
	failureTTL      = 5 * time.Minute
)

var httpClient = &http.Client{Timeout: downloadTimeout}

// lru is a small least-recently-used cache keyed by string.
type lru[V any] struct {
	capacity int
	ll       *list.List
	items    map[string]*list.Element
	onEvict  func(V)
}

type lruEntry[V any] struct {
	key   string
	value V
}

func newLRU[V any](capacity int, onEvict func(V)) *lru[V] {
	return &lru[V]{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
		onEvict:  onEvict,
	}
}

func (l *lru[V]) get(key string) (V, bool) {
	if el, ok := l.items[key]; ok {
		l.ll.MoveToFront(el)
		return el.Value.(*lruEntry[V]).value, true
	}
	var zero V
	return zero, false
}

func (l *lru[V]) contains(key string) bool {
	_, ok := l.items[key]
	return ok
}

func (l *lru[V]) put(key string, value V) {
	if el, ok := l.items[key]; ok {
		el.Value.(*lruEntry[V]).value = value
		l.ll.MoveToFront(el)
		return
	}
	l.items[key] = l.ll.PushFront(&lruEntry[V]{key: key, value: value})
	for l.ll.Len() > l.capacity {
		oldest := l.ll.Back()
		entry := oldest.Value.(*lruEntry[V])
		l.ll.Remove(oldest)
		delete(l.items, entry.key)
		if l.onEvict != nil {
			l.onEvict(entry.value)
		}
	}
}

// GlobalImageCache downloads images in the background and keeps two bounded
// caches: decoded images by URL, and terminal images by URL, cell size, and
// cell geometry.
type GlobalImageCache struct {
	mu       sync.Mutex
	enabled  bool
	hq       bool // kitty or sixel rather than block characters
	cellPixW int  // terminal cell size in pixels, 0 when unknown
	cellPixH int
	vx       *vaxis.Vaxis
	notify   func()
	raw      *lru[image.Image]
	scaled   *lru[vaxis.Image]
	loading  map[string]bool
	failed   map[string]time.Time
	sem      chan struct{}
}

var ImageCache *GlobalImageCache
var once sync.Once

// InitImageCache sets up the cache. notify is called from download goroutines
// whenever a new image is ready to draw; enabled false turns images off.
func InitImageCache(vx *vaxis.Vaxis, notify func(), enabled bool) {
	once.Do(func() {
		c := &GlobalImageCache{
			vx:      vx,
			notify:  notify,
			loading: make(map[string]bool),
			failed:  make(map[string]time.Time),
			sem:     make(chan struct{}, downloadWorkers),
		}
		c.raw = newLRU[image.Image](maxRawImages, nil)
		c.scaled = newLRU[vaxis.Image](maxScaledImages, func(img vaxis.Image) {
			img.Destroy()
			vx.RemoveImage(img)
		})
		c.enabled = enabled && probeGraphics(vx)
		c.hq = vx.CanDisplayGraphics()
		ImageCache = c
	})
}

// probeGraphics reports whether vaxis can render images at all in this
// terminal (kitty, sixel, or block characters).
func probeGraphics(vx *vaxis.Vaxis) bool {
	img, err := vx.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		log.Printf("images disabled: %v", err)
		return false
	}
	img.Destroy()
	vx.RemoveImage(img)
	return true
}

// Enabled reports whether images can be drawn at all.
func (c *GlobalImageCache) Enabled() bool {
	return c != nil && c.enabled
}

// HighQuality reports whether a pixel graphics protocol (kitty or sixel) is in
// use rather than block characters.
func (c *GlobalImageCache) HighQuality() bool {
	return c != nil && c.hq
}

// SetCellPixelSize records the terminal's cell size in pixels, taken from
// resize events, so images can be scaled to exact cell boxes. Zero means the
// size is unknown.
func (c *GlobalImageCache) SetCellPixelSize(w, h int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.cellPixW, c.cellPixH = w, h
	c.mu.Unlock()
}

// cellGeometryLocked returns the pixel size vaxis assigns to one cell when it
// measures images. Block renderers always use 1x2; kitty and sixel use the
// terminal's reported cell size, which may not be known yet.
func (c *GlobalImageCache) cellGeometryLocked() (w, h int, known bool) {
	if !c.hq {
		return 1, 2, true
	}
	if c.cellPixW > 0 && c.cellPixH > 0 {
		return c.cellPixW, c.cellPixH, true
	}
	return 0, 0, false
}

// fitImage scales img down to fit within maxW x maxH pixels, preserving its
// aspect ratio, and anchors the result at (0,0). Images that already fit are
// returned unchanged.
func fitImage(img image.Image, maxW, maxH int) image.Image {
	if img == nil || maxW <= 0 || maxH <= 0 {
		return img
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return img
	}
	if w <= maxW && h <= maxH && bounds.Min == (image.Point{}) {
		return img
	}
	scale := math.Min(1, math.Min(float64(maxW)/float64(w), float64(maxH)/float64(h)))
	newW := min(maxW, max(1, int(float64(w)*scale)))
	newH := min(maxH, max(1, int(float64(h)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, xdraw.Src, nil)
	return dst
}

// coverBox is the request shape used before images were pre-scaled: a box in
// cells with the image's own aspect ratio that covers width x height. It is
// only used when the cell geometry is unknown.
func coverBox(img image.Image, width, height int) (int, int) {
	bounds := img.Bounds()
	ow, oh := bounds.Dx(), bounds.Dy()
	if ow <= 0 || oh <= 0 {
		return width, height
	}
	scale := math.Max(float64(width)/float64(ow), float64(height)/float64(oh))
	return max(1, int(float64(ow)*scale)), max(1, int(float64(oh)*scale))
}

// Get returns the image at url scaled to fit within width x height cells. When
// the image is not cached yet a download is started and false is returned;
// notify fires once it can be drawn. Get must be called from the main thread.
func (c *GlobalImageCache) Get(url string, width, height int) (vaxis.Image, bool) {
	if !c.Enabled() || url == "" || width <= 0 || height <= 0 {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	cellW, cellH, known := c.cellGeometryLocked()
	cacheKey := fmt.Sprintf("%s|%d|%d|%d|%d", url, width, height, cellW, cellH)
	if img, ok := c.scaled.get(cacheKey); ok {
		return img, true
	}

	rawImg, ok := c.raw.get(url)
	if !ok {
		c.loadLocked(url)
		return nil, false
	}

	var vxImage vaxis.Image
	var err error
	if known {
		// Scale to the exact pixel box ourselves and hand vaxis an image that
		// already fits. Its own scaler skips resizing when both of its scale
		// factors come out equal, which sends the image at full size.
		vxImage, err = c.vx.NewImage(fitImage(rawImg, width*cellW, height*cellH))
		if err == nil {
			vxImage.Resize(width, height)
		}
	} else {
		// Unknown cell geometry on a pixel protocol: request a box shaped like
		// the image, which never trips the equal-factor case for square images.
		vxImage, err = c.vx.NewImage(rawImg)
		if err == nil {
			w, h := coverBox(rawImg, width, height)
			vxImage.Resize(w, h)
		}
	}
	if err != nil {
		log.Printf("Error creating terminal image for %s: %v", url, err)
		return nil, false
	}
	c.scaled.put(cacheKey, vxImage)

	return vxImage, true
}

// LoadAsync starts downloading url in the background if it is not cached,
// already loading, or recently failed.
func (c *GlobalImageCache) LoadAsync(url string) {
	if !c.Enabled() || url == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadLocked(url)
}

func (c *GlobalImageCache) loadLocked(url string) {
	if c.loading[url] || c.raw.contains(url) {
		return
	}
	if failedAt, ok := c.failed[url]; ok && time.Since(failedAt) < failureTTL {
		return
	}
	c.loading[url] = true
	go c.download(url)
}

func (c *GlobalImageCache) download(url string) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	img, err := DownloadImage(url)

	c.mu.Lock()
	delete(c.loading, url)
	if err != nil {
		c.failed[url] = time.Now()
	} else {
		delete(c.failed, url)
		c.raw.put(url, img)
	}
	c.mu.Unlock()

	if err != nil {
		log.Printf("Error downloading image %s: %v", url, err)
		return
	}
	if c.notify != nil {
		c.notify()
	}
}

func DownloadImage(url string) (image.Image, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", constants.AppName+"/"+constants.AppVersion)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get image: bad status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("image exceeds %d bytes", maxImageBytes)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return img, nil
}
