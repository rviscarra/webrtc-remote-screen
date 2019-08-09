package rtc

import (
	"fmt"

	"github.com/rviscarra/x-remote-viewer/internal/encoders"
	"github.com/rviscarra/x-remote-viewer/internal/rdisplay"
)

// RemoteScreenService is our implementation of the rtc.Service
type RemoteScreenService struct {
	stunServer      string
	videoService    rdisplay.Service
	encodingService encoders.Service
}

// NewRemoteScreenService creates a new instances of RemoteScreenService
func NewRemoteScreenService(stun string, video rdisplay.Service, enc encoders.Service) Service {
	return &RemoteScreenService{
		stunServer:      stun,
		videoService:    video,
		encodingService: enc,
	}
}

func hasElement(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// CreateRemoteScreenConnection creates and configures a new peer connection
// that will stream the selected screen
func (svc *RemoteScreenService) CreateRemoteScreenConnection(screenIx int, fps int) (RemoteScreenConnection, error) {
	screens, err := svc.videoService.Screens()
	if err != nil {
		return nil, err
	}

	if screenIx < 0 || screenIx > len(screens) {
		screenIx = 0
	}
	screen := screens[screenIx]
	screenGrabber, err := svc.videoService.CreateScreenGrabber(screen, fps)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	if len(screens) == 0 {
		return nil, fmt.Errorf("No available screens")
	}

	rtcPeer := newRemoteScreenPeerConn(svc.stunServer, screenGrabber, svc.encodingService)
	return rtcPeer, nil
}
