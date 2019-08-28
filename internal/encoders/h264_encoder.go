// +build h264enc

package encoders

import (
	"bytes"
	"fmt"
	"image"
	"math"

	"github.com/gen2brain/x264-go"
)

//H264Encoder h264 encoder
type H264Encoder struct {
	buffer   *bytes.Buffer
	encoder  *x264.Encoder
	realSize image.Point
}

const h264SupportedProfile = "3.1"

func newH264Encoder(size image.Point, frameRate int) (Encoder, error) {
	buffer := bytes.NewBuffer(make([]byte, 0))
	realSize, err := findBestSizeForH264Profile(h264SupportedProfile, size)
	if err != nil {
		return nil, err
	}
	opts := x264.Options{
		Width:     realSize.X,
		Height:    realSize.Y,
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
		buffer:   buffer,
		encoder:  encoder,
		realSize: realSize,
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

//VideoSize returns the size the other side is expecting
func (e *H264Encoder) VideoSize() (image.Point, error) {
	return e.realSize, nil
}

//Close flushes and closes the inner x264 encoder
func (e *H264Encoder) Close() error {
	return e.encoder.Close()
}

//findBestSizeForH264Profile finds the best match given the size constraint and H264 profile
func findBestSizeForH264Profile(profile string, constraints image.Point) (image.Point, error) {
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

func init() {
	registeredEncoders[H264Codec] = newH264Encoder
}
