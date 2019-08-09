package rtc

import (
	"fmt"
	"image"

	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"
	"github.com/rviscarra/x-remote-viewer/internal/encoding"
	"github.com/rviscarra/x-remote-viewer/internal/rdisplay"
)

type rtcStreamer struct {
	track   *webrtc.Track
	stop    chan struct{}
	screen  *rdisplay.ScreenGrabber
	encoder *encoding.Encoder
}

func newRTCStreamer(track *webrtc.Track, screen *rdisplay.ScreenGrabber, encoder *encoding.Encoder) videoStreamer {
	return &rtcStreamer{
		track:   track,
		stop:    make(chan struct{}),
		screen:  screen,
		encoder: encoder,
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
	payload, err := (*s.encoder).Encode(frame)
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
