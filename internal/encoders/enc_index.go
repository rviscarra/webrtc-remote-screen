package encoders

import (
	"fmt"
	"image"
)

type encoderFactory = func(size image.Point, frameRate int) (Encoder, error)

// Index of supported codecs, each encoder should register itself
// It's implemented this way to support conditional compilation
// of each encoder.
var registeredEncoders = make(map[VideoCodec]encoderFactory, 2)

//EncoderService creates instances of encoders
type EncoderService struct {
}

//NewEncoderService creates an encoder factory
func NewEncoderService() Service {
	return &EncoderService{}
}

//NewEncoder creates an instance of an encoder of the selected codec
func (*EncoderService) NewEncoder(codec VideoCodec, size image.Point, frameRate int) (Encoder, error) {
	factory, found := registeredEncoders[codec]
	if !found {
		return nil, fmt.Errorf("Codec not supported")
	}
	return factory(size, frameRate)
}

//Supports returns a boolean indicating if the codec is supported
func (*EncoderService) Supports(codec VideoCodec) bool {
	_, found := registeredEncoders[codec]
	return found
}
