package rtc

import (
	"fmt"
	"image"
	"log"
	"math/rand"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/pion/sdp"
	"github.com/pion/webrtc/v2"
	"github.com/rviscarra/x-remote-viewer/internal/encoders"
	"github.com/rviscarra/x-remote-viewer/internal/rdisplay"
)

// RemoteScreenPeerConn is a webrtc.PeerConnection wrapper that implements the
// PeerConnection interface
type RemoteScreenPeerConn struct {
	connection *webrtc.PeerConnection
	stunServer string
	track      *webrtc.Track
	streamer   videoStreamer
	grabber    rdisplay.ScreenGrabber
	encService encoders.Service
}

func findBestCodec(sdp *sdp.SessionDescription, encService encoders.Service, h264Profile string) (*webrtc.RTPCodec, encoders.VideoCodec, error) {
	var h264Codec *webrtc.RTPCodec
	var vp8Codec *webrtc.RTPCodec
	for _, md := range sdp.MediaDescriptions {
		for _, format := range md.MediaName.Formats {
			intPt, err := strconv.Atoi(format)
			payloadType := uint8(intPt)
			sdpCodec, err := sdp.GetCodecForPayloadType(payloadType)
			if err != nil {
				return nil, encoders.NoCodec, fmt.Errorf("Can't find codec for %d", payloadType)
			}

			if sdpCodec.Name == webrtc.H264 && h264Codec == nil {
				packetSupport := strings.Contains(sdpCodec.Fmtp, "packetization-mode=1")
				supportsProfile := strings.Contains(sdpCodec.Fmtp, fmt.Sprintf("profile-level-id=%s", h264Profile))
				if packetSupport && supportsProfile {
					h264Codec = webrtc.NewRTPH264Codec(payloadType, sdpCodec.ClockRate)
					h264Codec.SDPFmtpLine = sdpCodec.Fmtp
				}
			} else if sdpCodec.Name == webrtc.VP8 && vp8Codec == nil {
				vp8Codec = webrtc.NewRTPVP8Codec(payloadType, sdpCodec.ClockRate)
				vp8Codec.SDPFmtpLine = sdpCodec.Fmtp
			}
		}
	}
	if vp8Codec != nil && encService.Supports(encoders.VP8Codec) {
		return vp8Codec, encoders.VP8Codec, nil
	}
	if h264Codec != nil && encService.Supports(encoders.H264Codec) {
		return h264Codec, encoders.H264Codec, nil
	}
	return nil, encoders.NoCodec, fmt.Errorf("Couldn't find a matching codec")
}

func newRemoteScreenPeerConn(stunServer string, grabber rdisplay.ScreenGrabber, encService encoders.Service) *RemoteScreenPeerConn {
	return &RemoteScreenPeerConn{
		stunServer: stunServer,
		grabber:    grabber,
		encService: encService,
	}
}

func getTrackDirection(sdp *sdp.SessionDescription) webrtc.RTPTransceiverDirection {
	for _, mediaDesc := range sdp.MediaDescriptions {
		if mediaDesc.MediaName.Media == "video" {
			if _, recvOnly := mediaDesc.Attribute("recvonly"); recvOnly {
				return webrtc.RTPTransceiverDirectionRecvonly
			} else if _, sendRecv := mediaDesc.Attribute("sendrecv"); sendRecv {
				return webrtc.RTPTransceiverDirectionSendrecv
			}
		}
	}
	return webrtc.RTPTransceiverDirectionInactive
}

// ProcessOffer handles the SDP offer coming from the client,
// return the SDP answer that must be passed back to stablish the WebRTC
// connection.
func (p *RemoteScreenPeerConn) ProcessOffer(strOffer string) (string, error) {
	sdp := sdp.SessionDescription{}
	err := sdp.Unmarshal(strOffer)
	if err != nil {
		return "", err
	}

	webrtcCodec, encCodec, err := findBestCodec(&sdp, p.encService, "42e01f")
	if err != nil {
		return "", err
	}
	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterCodec(webrtcCodec)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

	pcconf := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			webrtc.ICEServer{
				URLs: []string{p.stunServer},
			},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}

	peerConn, err := api.NewPeerConnection(pcconf)
	if err != nil {
		return "", err
	}
	p.connection = peerConn

	peerConn.OnICEConnectionStateChange(func(connState webrtc.ICEConnectionState) {
		if connState == webrtc.ICEConnectionStateConnected {
			p.start()
		}
		if connState == webrtc.ICEConnectionStateDisconnected {
			p.Close()
		}
		log.Printf("Connection state: %s \n", connState.String())
	})

	track, err := peerConn.NewTrack(
		webrtcCodec.PayloadType,
		uint32(rand.Int31()),
		uuid.New().String(),
		fmt.Sprintf("remote-screen"),
	)

	log.Printf("Using codec %s (%d) %s", webrtcCodec.Name, webrtcCodec.PayloadType, webrtcCodec.SDPFmtpLine)

	direction := getTrackDirection(&sdp)

	if direction == webrtc.RTPTransceiverDirectionSendrecv {
		_, err = peerConn.AddTrack(track)
	} else if direction == webrtc.RTPTransceiverDirectionRecvonly {
		_, err = peerConn.AddTransceiverFromTrack(track, webrtc.RtpTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		})
	} else {
		return "", fmt.Errorf("Unsupported transceiver direction")
	}

	offerSdp := webrtc.SessionDescription{
		SDP:  strOffer,
		Type: webrtc.SDPTypeOffer,
	}
	err = peerConn.SetRemoteDescription(offerSdp)
	if err != nil {
		return "", err
	}

	p.track = track

	answer, err := peerConn.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	screen := p.grabber.Screen()
	sourceSize := image.Point{
		screen.Bounds.Dx(),
		screen.Bounds.Dy(),
	}

	encoder, err := p.encService.NewEncoder(encCodec, sourceSize, p.grabber.Fps())
	if err != nil {
		return "", err
	}

	size, err := encoder.VideoSize()
	if err != nil {
		return "", err
	}

	p.streamer = newRTCStreamer(p.track, &p.grabber, &encoder, size)

	err = peerConn.SetLocalDescription(answer)
	if err != nil {
		return "", err
	}
	return answer.SDP, nil
}

func (p *RemoteScreenPeerConn) start() {
	p.streamer.start()
}

// Close Stops the video streamer and closes the WebRTC peer connection
func (p *RemoteScreenPeerConn) Close() error {

	if p.streamer != nil {
		p.streamer.close()
	}

	if p.connection != nil {
		return p.connection.Close()
	}
	return nil
}
