package handler

import (
	"bytes"
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"go.uber.org/zap/buffer"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
)

type MediaNodeInformer interface {
	NodeInfo() warpnet.NodeInfo
}

type MediaStorer interface {
	GetImage() ([]byte, error)
	SetImage(img []byte) error
}

type MediaUserFetcher interface {
	Get(userId string) (user domain.User, err error)
}

func StreamUploadImageHandler(
	info MediaNodeInformer,
	mediaRepo MediaStorer,
	userRepo MediaUserFetcher,
) middleware.WarpHandler {
	return func(input []byte, s warpnet.WarpStream) (any, error) {
		if mediaRepo == nil {
			return nil, nil
		}

		img, _, err := image.Decode(bytes.NewBuffer(input))
		if errors.Is(err, image.ErrFormat) {
			return nil, errors.New("invalid image format: PNG, JPG, JPEG, GIF are only allowed") // TODO
		}
		if err != nil {
			return nil, err
		}

		var buf buffer.Buffer
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100})
		if err != nil {
			return nil, err
		}

		return event.Accepted, nil
	}
}
