package encoding

import (
	"bytes"
	"image"

	"github.com/gen2brain/x264-go"
)

//H264EncoderService creates instances of h264 encoder
type H264EncoderService struct {
}

//H264Encoder h264 encoder
type H264Encoder struct {
	buffer  *bytes.Buffer
	encoder *x264.Encoder
}

//NewH264EncoderService creates a new h264 encoder factory
func NewH264EncoderService() Service {
	return &H264EncoderService{}
}

//NewEncoder creates an instances of a h264 encoder
func (*H264EncoderService) NewEncoder(size image.Rectangle, frameRate int) (Encoder, error) {
	buffer := bytes.NewBuffer(make([]byte, 0))
	opts := x264.Options{
		Width:     size.Dx(),
		Height:    size.Dy(),
		FrameRate: frameRate,
		Tune:      "zerolatency",
		Preset:    "veryfast",
		Profile:   "baseline",
		LogLevel:  x264.LogWarning,
	}
	encoder, err := x264.NewEncoder(buffer, &opts)
	if err != nil {
		return nil, err
	}
	return &H264Encoder{
		buffer:  buffer,
		encoder: encoder,
	}, nil
}

//Encode encodes a frame into a h264 payload
func (e *H264Encoder) Encode(frame *image.RGBA) ([]byte, error) {
	err := e.encoder.Encode(frame)
	if err != nil {
		return nil, err
	}
	err = e.encoder.Flush()
	if err != nil {
		return nil, err
	}
	payload := e.buffer.Bytes()
	e.buffer.Reset()
	return payload, nil
}

//Close flushes and closes the inner x264 encoder
func (e *H264Encoder) Close() error {
	return e.encoder.Close()
}
