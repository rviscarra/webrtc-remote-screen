package rtc

import (
	"fmt"
	"log"
	"time"

	"math/rand"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v2"
	"github.com/rviscarra/x-remote-viewer/internal/encoding"
	"github.com/rviscarra/x-remote-viewer/internal/rdisplay"
)

// PionPeerConnection is a webrtc.PeerConnection wrapper that implements the
// PeerConnection interface
type PionPeerConnection struct {
	pc *webrtc.PeerConnection
}

// PionRtcService is our implementation of the rtc.Service
type PionRtcService struct {
	stunServer      string
	api             *webrtc.API
	videoService    rdisplay.Service
	encodingService encoding.Service
}

// NewPionRtcService creates a new instances of PionRtcService
func NewPionRtcService(stun string, video rdisplay.Service, enc encoding.Service) Service {
	mediaEngine := webrtc.MediaEngine{}
	h264 := webrtc.NewRTPCodec(webrtc.RTPCodecTypeVideo,
		webrtc.H264,
		90000,
		0,
		"level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		webrtc.DefaultPayloadTypeH264,
		&codecs.H264Payloader{})
	mediaEngine.RegisterCodec(h264)
	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
	)
	return &PionRtcService{
		stunServer:      stun,
		api:             api,
		videoService:    video,
		encodingService: enc,
	}
}

// ProcessOffer handles the SDP offer coming from the client,
// return the SDP answer that must be passed back to stablish the WebRTC
// connection.
func (p *PionPeerConnection) ProcessOffer(offer string) (string, error) {
	err := p.pc.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  offer,
		Type: webrtc.SDPTypeOffer,
	})
	if err != nil {
		return "", err
	}

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	err = p.pc.SetLocalDescription(answer)
	if err != nil {
		return "", err
	}
	return answer.SDP, nil
}

// Close just closes the underlying peer connection
func (p *PionPeerConnection) Close() error {
	return p.pc.Close()
}

// CreatePeerConnection creates and configures a new peer connection for
// our purposes, receive one audio track and send data through one DataChannel
func (svc *PionRtcService) CreatePeerConnection(screenIx int) (PeerConnection, error) {
	pcconf := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			webrtc.ICEServer{
				URLs: []string{svc.stunServer},
			},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}
	pc, err := svc.api.NewPeerConnection(pcconf)
	if err != nil {
		return nil, err
	}

	screens, err := svc.videoService.Screens()
	if err != nil {
		return nil, err
	}

	if screenIx < 0 || screenIx > len(screens) {
		screenIx = 0
	}
	screen := screens[screenIx]

	track, err := pc.NewTrack(
		webrtc.DefaultPayloadTypeH264,
		uint32(rand.Int31()),
		uuid.New().String(),
		fmt.Sprintf("screen-%d", screenIx),
	)
	if err != nil {
		return nil, err
	}

	_, err = pc.AddTransceiverFromTrack(track, webrtc.RtpTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	})
	if err != nil {
		return nil, err
	}

	if len(screens) == 0 {
		return nil, fmt.Errorf("No available screens")
	}

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			err := pc.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()},
			})
			if err != nil {
				return
			}
		}
	}()

	grabber, err := svc.videoService.CreateScreenGrabber(screenIx, 20)
	if err != nil {
		return nil, err
	}

	encoder, err := svc.encodingService.NewEncoder(screen.Bounds, 20)
	if err != nil {
		return nil, err
	}
	streamer := newRTCStreamer(track, &grabber, &encoder)

	pc.OnICEConnectionStateChange(func(connState webrtc.ICEConnectionState) {
		if connState == webrtc.ICEConnectionStateConnected {
			streamer.start()
		}
		if connState == webrtc.ICEConnectionStateDisconnected {
			ticker.Stop()
			streamer.close()
			pc.Close()
		}
		log.Printf("Connection state: %s \n", connState.String())
	})

	return &PionPeerConnection{
		pc: pc,
	}, nil
}
