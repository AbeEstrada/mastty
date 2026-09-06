package utils

import (
	"image"
	"testing"
)

func ceilDiv(a, b int) int {
	if a%b != 0 {
		return a/b + 1
	}
	return a / b
}

func TestFitImageSquareAvatarIntoCellBox(t *testing.T) {
	// A 320x320 avatar into 6x3 cells at 16x34 px per cell. vaxis rounds pixel
	// sizes up to whole cells, so the result must measure 6 columns by 3 rows.
	cellW, cellH := 16, 34
	got := fitImage(image.NewRGBA(image.Rect(0, 0, 320, 320)), 6*cellW, 3*cellH)
	b := got.Bounds()
	if b.Dx() != 96 || b.Dy() != 96 {
		t.Fatalf("size = %dx%d, want 96x96", b.Dx(), b.Dy())
	}
	if cols, rows := ceilDiv(b.Dx(), cellW), ceilDiv(b.Dy(), cellH); cols != 6 || rows != 3 {
		t.Fatalf("cells = %dx%d, want 6x3", cols, rows)
	}
}

func TestFitImageKeepsAspect(t *testing.T) {
	// Wide photo into 60 columns by 21 rows at 15x30 px cells (900x630 px).
	got := fitImage(image.NewRGBA(image.Rect(0, 0, 2000, 1296)), 900, 630)
	b := got.Bounds()
	if b.Dx() != 900 || b.Dy() != 583 {
		t.Fatalf("wide size = %dx%d, want 900x583", b.Dx(), b.Dy())
	}

	// Tall image is limited by height.
	got = fitImage(image.NewRGBA(image.Rect(0, 0, 300, 1200)), 96, 102)
	b = got.Bounds()
	if b.Dy() != 102 || b.Dx() < 24 || b.Dx() > 26 {
		t.Fatalf("tall size = %dx%d, want about 25x102", b.Dx(), b.Dy())
	}
}

func TestFitImageNeverExceedsBox(t *testing.T) {
	for _, size := range [][2]int{{320, 320}, {400, 400}, {500, 324}, {1200, 630}, {1, 1}, {7, 3000}} {
		for _, box := range [][2]int{{96, 102}, {900, 630}, {6, 6}, {1, 2}} {
			got := fitImage(image.NewRGBA(image.Rect(0, 0, size[0], size[1])), box[0], box[1])
			b := got.Bounds()
			if b.Dx() > box[0] || b.Dy() > box[1] || b.Dx() < 1 || b.Dy() < 1 {
				t.Errorf("fitImage(%v, %v) = %dx%d exceeds box", size, box, b.Dx(), b.Dy())
			}
		}
	}
}

func TestFitImageAlreadyFits(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 50, 20))
	if got := fitImage(src, 96, 102); got != image.Image(src) {
		t.Fatal("expected the same image back when it already fits")
	}
}

func TestFitImageAnchorsSubImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	sub := src.SubImage(image.Rect(10, 10, 60, 40))
	got := fitImage(sub, 200, 200)
	b := got.Bounds()
	if b.Min != (image.Point{}) || b.Dx() != 50 || b.Dy() != 30 {
		t.Fatalf("sub-image bounds = %v, want 50x30 anchored at 0,0", b)
	}
}

func TestCoverBox(t *testing.T) {
	if w, h := coverBox(image.NewRGBA(image.Rect(0, 0, 320, 320)), 6, 3); w != 6 || h != 6 {
		t.Fatalf("square cover box = %dx%d, want 6x6", w, h)
	}
	if w, h := coverBox(image.NewRGBA(image.Rect(0, 0, 800, 400)), 6, 3); w != 6 || h != 3 {
		t.Fatalf("wide cover box = %dx%d, want 6x3", w, h)
	}
}
