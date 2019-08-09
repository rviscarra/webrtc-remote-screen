package encoders

import (
	"bytes"
	"fmt"
	"image"
	"math"

	"github.com/gen2brain/x264-go"
)

//EncoderService creates instances of encoders
type EncoderService struct {
}

//NewEncoderService creates an encoder factory
func NewEncoderService() Service {
	return &EncoderService{}
}

//NewEncoder creates an instance of an encoder of the selected codec
func (*EncoderService) NewEncoder(codec VideoCodec, size image.Point, frameRate int) (Encoder, error) {
	if codec == H264Codec {
		return newH264Encoder(size, frameRate)
	} else {
		return nil, fmt.Errorf("Codec not supported")
	}
}

//H264Encoder h264 encoder
type H264Encoder struct {
	buffer  *bytes.Buffer
	encoder *x264.Encoder
}

func newH264Encoder(size image.Point, frameRate int) (*H264Encoder, error) {
	buffer := bytes.NewBuffer(make([]byte, 0))
	opts := x264.Options{
		Width:     size.X,
		Height:    size.Y,
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

// FindBestSizeForH264Profile finds the best match given the size constraint and H264 profile
func FindBestSizeForH264Profile(profile string, constraints image.Point) (image.Point, error) {
	profileSizes := map[string][]image.Point{
		"3.1": []image.Point{
			image.Point{1280, 720},
			image.Point{720, 576},
			image.Point{720, 480},
		},
	}
	if sizes, exists := profileSizes[profile]; exists {
		minRatioDiff := math.MaxFloat64
		var minRatioSize image.Point
		for _, size := range sizes {
			if size == constraints {
				return size, nil
			}
			lowerRes := size.X < constraints.X && size.Y < constraints.Y
			hRatio := float64(constraints.X) / float64(size.X)
			vRatio := float64(constraints.Y) / float64(size.Y)
			ratioDiff := math.Abs(hRatio - vRatio)
			if lowerRes && (ratioDiff) < 0.0001 {
				return size, nil
			} else if ratioDiff < minRatioDiff {
				minRatioDiff = ratioDiff
				minRatioSize = size
			}
		}
		return minRatioSize, nil
	}
	return image.Point{}, fmt.Errorf("Profile %s not supported", profile)
}
