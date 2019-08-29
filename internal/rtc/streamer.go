package rtc

import (
	"fmt"
	"image"

	"github.com/nfnt/resize"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"
	"github.com/rviscarra/webrtc-remote-screen/internal/encoders"
	"github.com/rviscarra/webrtc-remote-screen/internal/rdisplay"
)

func resizeImage(src *image.RGBA, target image.Point) *image.RGBA {
	return resize.Resize(uint(target.X), uint(target.Y), src, resize.Lanczos3).(*image.RGBA)
}

type rtcStreamer struct {
	track   *webrtc.Track
	stop    chan struct{}
	screen  *rdisplay.ScreenGrabber
	encoder *encoders.Encoder
	size    image.Point
}

func newRTCStreamer(track *webrtc.Track, screen *rdisplay.ScreenGrabber, encoder *encoders.Encoder, size image.Point) videoStreamer {
	return &rtcStreamer{
		track:   track,
		stop:    make(chan struct{}),
		screen:  screen,
		encoder: encoder,
		size:    size,
	}
}

func (s *rtcStreamer) start() {
	go s.startStream()
}

func (s *rtcStreamer) startStream() {
	screen := *s.screen
	screen.Start()
	frames := screen.Frames()
	for {
		select {
		case <-s.stop:
			screen.Stop()
			return
		case frame := <-frames:
			err := s.stream(frame)
			if err != nil {
				fmt.Printf("Streamer: %v\n", err)
				return
			}
		}
	}
}

func (s *rtcStreamer) stream(frame *image.RGBA) error {
	resized := resizeImage(frame, s.size)
	payload, err := (*s.encoder).Encode(resized)
	if err != nil {
		return err
	}
	if payload == nil {
		return nil
	}
	return s.track.WriteSample(media.Sample{
		Data:    payload,
		Samples: 1,
	})
}

func (s *rtcStreamer) close() {
	close(s.stop)
}
