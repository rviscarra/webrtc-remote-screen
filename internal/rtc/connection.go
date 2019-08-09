package rtc

import (
	"fmt"
	"image"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
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
	pliTicker  *time.Ticker
	streamer   videoStreamer
	grabber    rdisplay.ScreenGrabber
	encService encoders.Service
}

const h264SupportedProfile = "3.1"

func findBestCodec(sdp *sdp.SessionDescription, profile string) (*webrtc.RTPCodec, error) {
	for _, md := range sdp.MediaDescriptions {
		for _, format := range md.MediaName.Formats {
			intPt, err := strconv.Atoi(format)
			payloadType := uint8(intPt)
			sdpCodec, err := sdp.GetCodecForPayloadType(payloadType)
			if err != nil {
				return nil, fmt.Errorf("Can't find codec for %d", payloadType)
			}

			if sdpCodec.Name == webrtc.H264 {
				packetSupport := strings.Contains(sdpCodec.Fmtp, "packetization-mode=1")
				supportsProfile := strings.Contains(sdpCodec.Fmtp, fmt.Sprintf("profile-level-id=%s", profile))
				if packetSupport && supportsProfile {
					var codec = webrtc.NewRTPH264Codec(payloadType, sdpCodec.ClockRate)
					codec.SDPFmtpLine = sdpCodec.Fmtp
					return codec, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Couldn't find a matching codec")
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

	codec, err := findBestCodec(&sdp, "42e01f")
	if err != nil {
		return "", err
	}
	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterCodec(codec)

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
		codec.PayloadType,
		uint32(rand.Int31()),
		uuid.New().String(),
		fmt.Sprintf("remote-screen"),
	)

	log.Printf("Using codec %s (%d) %s", codec.Name, codec.PayloadType, codec.SDPFmtpLine)

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
	size, err := encoders.FindBestSizeForH264Profile(h264SupportedProfile, image.Point{
		screen.Bounds.Dx(),
		screen.Bounds.Dy(),
	})
	if err != nil {
		return "", err
	}

	encoder, err := p.encService.NewEncoder(encoders.H264Codec, size, p.grabber.Fps())
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

// TODO: handle this correctly
func (p *RemoteScreenPeerConn) start() {
	p.streamer.start()
	p.startPLILoop()
}

func (p *RemoteScreenPeerConn) startPLILoop() {
	p.pliTicker = time.NewTicker(3 * time.Second)
	go func() {
		for range p.pliTicker.C {
			err := p.connection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{MediaSSRC: p.track.SSRC()},
			})
			if err != nil {
				return
			}
		}
	}()
}

// Close Stops the PLI ticker, the video streamer and closes the WebRTC peer connection
func (p *RemoteScreenPeerConn) Close() error {
	if p.pliTicker != nil {
		p.pliTicker.Stop()
	}

	if p.streamer != nil {
		p.streamer.close()
	}

	return p.connection.Close()
}
