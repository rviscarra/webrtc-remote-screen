package rdisplay

import "image"

// ScreenGrabber TODO
type ScreenGrabber interface {
	Start()
	Frames() <-chan *image.RGBA
	Stop()
}

// Screen TODO
type Screen struct {
	Index  int
	Bounds image.Rectangle
}

// Service TODO
type Service interface {
	CreateScreenGrabber(screen, fps int) (ScreenGrabber, error)
	Screens() ([]Screen, error)
}
