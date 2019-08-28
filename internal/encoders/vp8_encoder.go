// +build vp8enc

package encoders

import (
	"bytes"
	"fmt"
	"image"
	"unsafe"
)

/*
#cgo pkg-config: vpx
#include <stdlib.h>
#include <vpx/vpx_encoder.h>
#include "tools_common.h"

void rgba_to_yuv(uint8_t *destination, uint8_t *rgba, size_t width, size_t height) {
	size_t image_size = width * height;
	size_t upos = image_size;
	size_t vpos = upos + upos / 4;
	size_t i = 0;

	for( size_t line = 0; line < height; ++line ) {
		if( !(line % 2) ) {
			for( size_t x = 0; x < width; x += 2 ) {
				uint8_t r = rgba[4 * i];
				uint8_t g = rgba[4 * i + 1];
				uint8_t b = rgba[4 * i + 2];

				destination[i++] = ((66*r + 129*g + 25*b) >> 8) + 16;

				destination[upos++] = ((-38*r + -74*g + 112*b) >> 8) + 128;
				destination[vpos++] = ((112*r + -94*g + -18*b) >> 8) + 128;

				r = rgba[4 * i];
				g = rgba[4 * i + 1];
				b = rgba[4 * i + 2];

				destination[i++] = ((66*r + 129*g + 25*b) >> 8) + 16;
			}
		} else {
			for( size_t x = 0; x < width; x += 1 ) {
					uint8_t r = rgba[4 * i];
					uint8_t g = rgba[4 * i + 1];
					uint8_t b = rgba[4 * i + 2];

					destination[i++] = ((66*r + 129*g + 25*b) >> 8) + 16;
			}
		}
	}
}

int32_t encode_frame(vpx_codec_ctx_t *ctx, vpx_image_t *img, int32_t framec, int32_t flags,
										 void *rgba, void *yuv_buf, int32_t w, int32_t h, void **encoded_frame) {
	rgba_to_yuv(yuv_buf, rgba, w, h);
	vpx_img_read(img, yuv_buf);
	if (vpx_codec_encode(ctx, img, (vpx_codec_pts_t)framec, 1, flags, VPX_DL_REALTIME) != 0) {
		return 0;
	}
	const vpx_codec_cx_pkt_t *pkt = NULL;
	vpx_codec_iter_t it = NULL;
	while ((pkt = vpx_codec_get_cx_data(ctx, &it)) != NULL) {
		if (pkt->kind == VPX_CODEC_CX_FRAME_PKT) {
			*encoded_frame = pkt->data.frame.buf;
			return pkt->data.frame.sz;
		}
	}
	*encoded_frame = (void *)0xDEADBEEF;
	return 0;
}

vpx_codec_err_t codec_enc_config_default(const VpxInterface *encoder, vpx_codec_enc_cfg_t *cfg) {
	return vpx_codec_enc_config_default(encoder->codec_interface(), cfg, 0);
}

vpx_codec_err_t codec_enc_init(vpx_codec_ctx_t *codec, const VpxInterface *encoder, vpx_codec_enc_cfg_t *cfg) {
	return vpx_codec_enc_init(codec, encoder->codec_interface(), cfg, 0);
}

*/
import "C"

const keyFrameInterval = 10

//VP8Encoder VP8 encoder
type VP8Encoder struct {
	buffer     *bytes.Buffer
	realSize   image.Point
	codecCtx   C.vpx_codec_ctx_t
	vpxImage   C.vpx_image_t
	yuvBuffer  []byte
	frameCount uint
	// vpxCodexIter C.vpx_codec_iter_t
}

func newVP8Encoder(size image.Point, frameRate int) (Encoder, error) {
	buffer := bytes.NewBuffer(make([]byte, 0))
	codecName := C.CString("vp8")
	encoder := C.get_vpx_encoder_by_name(codecName)
	C.free(unsafe.Pointer(codecName))

	var cfg C.vpx_codec_enc_cfg_t
	if C.codec_enc_config_default(encoder, &cfg) != 0 {
		return nil, fmt.Errorf("Can't init default enc. config")
	}
	cfg.g_w = C.uint(size.X)
	cfg.g_h = C.uint(size.Y)
	cfg.g_timebase.num = 1
	cfg.g_timebase.den = C.int(frameRate)
	cfg.rc_target_bitrate = 90000
	cfg.g_error_resilient = 1

	var vpxCodecCtx C.vpx_codec_ctx_t
	if C.codec_enc_init(&vpxCodecCtx, encoder, &cfg) != 0 {
		return nil, fmt.Errorf("Failed to initialize enc ctx")
	}
	var vpxImage C.vpx_image_t
	if C.vpx_img_alloc(&vpxImage, C.VPX_IMG_FMT_I420, C.uint(size.X), C.uint(size.Y), 0) == nil {
		return nil, fmt.Errorf("Can't alloc. vpx image")
	}

	return &VP8Encoder{
		buffer:     buffer,
		realSize:   size,
		codecCtx:   vpxCodecCtx,
		vpxImage:   vpxImage,
		yuvBuffer:  make([]byte, size.X*size.Y*2),
		frameCount: 0,
	}, nil
}

//Encode encodes a frame into a h264 payload
func (e *VP8Encoder) Encode(frame *image.RGBA) ([]byte, error) {

	encodedData := unsafe.Pointer(nil)
	var flags C.int
	if e.frameCount%keyFrameInterval == 0 {
		flags |= C.VPX_EFLAG_FORCE_KF
	}
	frameSize := C.encode_frame(
		&e.codecCtx,
		&e.vpxImage,
		C.int(e.frameCount),
		flags,
		unsafe.Pointer(&frame.Pix[0]),
		unsafe.Pointer(&e.yuvBuffer[0]),
		C.int(e.realSize.X),
		C.int(e.realSize.Y),
		&encodedData,
	)
	e.frameCount++
	if int(frameSize) > 0 {
		encoded := C.GoBytes(encodedData, frameSize)
		return encoded, nil
		return nil, nil
	}
	return nil, nil
}

//Encode encodes a frame into a h264 payload
func (e *VP8Encoder) VideoSize() (image.Point, error) {
	return e.realSize, nil
}

//Close flushes and closes the inner x264 encoder
func (e *VP8Encoder) Close() error {
	C.vpx_img_free(&e.vpxImage)
	C.vpx_codec_destroy(&e.codecCtx)
	return nil
}

func init() {
	registeredEncoders[VP8Codec] = newVP8Encoder
}
