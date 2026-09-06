package constants

const (
	AppName = "Tuit"
	AppUrl  = "https://github.com/AbeEstrada/tuit"
)

// AppVersion is overridden at build time by the justfile through -ldflags -X.
var AppVersion = "dev"
