package rdisplay

import (
	"image"
	"time"

	"github.com/kbinani/screenshot"
)

// XVideoProvider implements the rdisplay.Service interface for XServer
type XVideoProvider struct{}

// XScreenGrabber captures video from a X server
type XScreenGrabber struct {
	fps    int
	bounds image.Rectangle
	frames chan *image.RGBA
	stop   chan struct{}
}

// CreateScreenGrabber Creates an screen capturer for the X server
func (*XVideoProvider) CreateScreenGrabber(screen, fps int) (ScreenGrabber, error) {
	var bounds image.Rectangle
	if screen >= 0 {
		bounds = screenshot.GetDisplayBounds(screen)
	}
	return &XScreenGrabber{
		bounds: bounds,
		fps:    fps,
		frames: make(chan *image.RGBA),
		stop:   make(chan struct{}),
	}, nil
}

// Screens Returns the available screens to capture
func (x *XVideoProvider) Screens() ([]Screen, error) {
	numScreens := screenshot.NumActiveDisplays()
	screens := make([]Screen, numScreens)
	for i := 0; i < numScreens; i++ {
		screens[i] = Screen{
			Index:  i,
			Bounds: screenshot.GetDisplayBounds(i),
		}
	}
	return screens, nil
}

// Frames returns a channel that will receive an image stream
func (g *XScreenGrabber) Frames() <-chan *image.RGBA {
	return g.frames
}

// Start initiates the screen capture loop
func (g *XScreenGrabber) Start() {
	delta := time.Duration(1000/g.fps) * time.Millisecond
	go func() {
		for {
			startedAt := time.Now()
			select {
			case <-g.stop:
				close(g.frames)
				return
			default:
				img, err := screenshot.CaptureRect(g.bounds)
				if err != nil {
					return
				}
				g.frames <- img
				ellapsed := time.Now().Sub(startedAt)
				sleepDuration := delta - ellapsed
				if sleepDuration > 0 {
					time.Sleep(sleepDuration)
				}
			}
		}
	}()
}

// Stop sends a stop signal to the capture loop
func (g *XScreenGrabber) Stop() {
	close(g.stop)
}

// NewVideoProvider returns an X Server-based video provider
func NewVideoProvider() (Service, error) {
	return &XVideoProvider{}, nil
}
