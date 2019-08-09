package encoders

import (
	"image"
	"io"
)

// Service creates encoder instances
type Service interface {
	NewEncoder(codec VideoCodec, size image.Point, frameRate int) (Encoder, error)
}

// Encoder takes an image/frame and encodes it
type Encoder interface {
	io.Closer
	Encode(*image.RGBA) ([]byte, error)
}

//VideoCodec can be either h264 or vp8
type VideoCodec = int

const (
	//H264Codec h264
	H264Codec VideoCodec = iota
	//VP8Codec vp8
	VP8Codec
)
