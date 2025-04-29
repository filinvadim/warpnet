package handler

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	jis "github.com/dsoprea/go-jpeg-image-structure/v2"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/security"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"strings"
)

/*

	The system embeds encrypted metadata (node and user information) into the EXIF segment of media files
	during upload.
	A weak password is randomly generated for each file, used for encryption via Argon2id + AES-256-GCM,
	and immediately discarded.
	The password is never stored or logged.
	Decryption is only possible through brute-force attacks, requiring massive computational resources.
	Ordinary users cannot recover the metadata; only powerful entities (e.g., government data centers) can.
	EXIF metadata acts as proof of ownership and responsibility without revealing sensitive data.
	Salt and nonce are public and embedded with the media file.
	Security relies entirely on computational difficulty, not on secrecy of the password.

*/

const (
	imageDescriptionTag = "ImageDescription"

	nodeMetaKey = "node"
	userMetaKey = "user"
	macMetaKey  = "MAC"
)

type MediaNodeInformer interface {
	NodeInfo() warpnet.NodeInfo
}

type MediaStorer interface {
	GetImage(key string) ([]byte, error)
	SetImage(key string, img []byte) error
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

		var ev event.UploadImageEvent
		if err := json.JSON.Unmarshal(input, &ev); err != nil {
			return nil, err
		}

		parts := strings.SplitN(ev.Image, ",", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid base64 image data")
		}

		imgBytes, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}

		img, _, err := image.Decode(bytes.NewReader(imgBytes))
		if errors.Is(err, image.ErrFormat) {
			return nil, errors.New("invalid image format: PNG, JPG, JPEG, GIF are only allowed") // TODO add more types
		}
		if err != nil {
			return nil, err
		}

		var imageBuf bytes.Buffer
		err = jpeg.Encode(&imageBuf, img, &jpeg.Options{Quality: 100})
		if err != nil {
			return nil, err
		}

		nodeInfo := info.NodeInfo()
		ownerUser, err := userRepo.Get(nodeInfo.OwnerId)
		if err != nil {
			return nil, err
		}

		metaData := map[string]any{
			nodeMetaKey: nodeInfo, userMetaKey: ownerUser, macMetaKey: warpnet.GetMacAddr(),
		}
		metaBytes, err := json.JSON.Marshal(metaData)
		if err != nil {
			return nil, err
		}

		encryptedMeta, err := security.EncryptAES(metaBytes, nil) // unknown password
		if err != nil {
			return nil, err
		}

		amendedImg, err := amendExifMetadata(imageBuf.Bytes(), encryptedMeta)
		if err != nil {
			return nil, err
		}

		if err := mediaRepo.SetImage(ev.Key, amendedImg); err != nil {
			return nil, err
		}

		return event.Accepted, nil
	}
}

func amendExifMetadata(imageBytes, metadata []byte) ([]byte, error) {
	parser := jis.NewJpegMediaParser()

	intfc, err := parser.ParseBytes(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("media: parse bytes: %w", err)
	}

	sl, ok := intfc.(*jis.SegmentList)
	if !ok {
		return nil, errors.New("amend: invalid exif type: not a segment list")
	}

	ifdMapping, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, fmt.Errorf("media: new IFD mapping: %w", err)
	}

	ti := exif.NewTagIndex()

	err = exif.LoadStandardTags(ti)
	if err != nil {
		return nil, fmt.Errorf("media: load standard tags: %w", err)
	}

	identity := exifcommon.NewIfdIdentity(
		exifcommon.IfdStandardIfdIdentity.IfdTag(),
		exifcommon.IfdIdentityPart{
			Name:  exifcommon.IfdStandardIfdIdentity.Name(),
			Index: exifcommon.IfdStandardIfdIdentity.Index(),
		},
	)

	rootIb := exif.NewIfdBuilder(ifdMapping, ti, identity, exifcommon.EncodeDefaultByteOrder)

	encodedMetadata := base64.StdEncoding.EncodeToString(metadata)

	err = rootIb.SetStandardWithName(imageDescriptionTag, encodedMetadata)
	if err != nil {
		return nil, fmt.Errorf("media: add standard tag: %w", err)
	}

	err = sl.SetExif(rootIb)
	if err != nil {
		return nil, fmt.Errorf("media: set EXIF: %w", err)
	}

	buf := new(bytes.Buffer)
	err = sl.Write(buf)
	if err != nil {
		return nil, fmt.Errorf("media: write bytes: %w", err)
	}

	return buf.Bytes(), nil
}
