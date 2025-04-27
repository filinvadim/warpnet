package handler

import (
	"bytes"
	"errors"
	"github.com/dsoprea/go-exif/v3"
	jis "github.com/dsoprea/go-jpeg-image-structure/v2"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/security"
	log "github.com/sirupsen/logrus"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
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
	ifdPath  = "IFD/Exif"
	warpMeta = "WarpMeta"

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

		img, _, err := image.Decode(bytes.NewBuffer([]byte(ev.Image)))
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

		c := security.NewArgon2Cipher(&security.DefaultArgon2Params)

		password := security.GenerateWeakPassword()
		defer func() {
			for i := range password { // avoid RAM snapshot attack
				password[i] = 0
			}
		}()

		encryptedMeta, err := c.EncryptA2id(metaBytes, password)
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
		return nil, err
	}

	sl, ok := intfc.(*jis.SegmentList)
	if !ok {
		return nil, errors.New("amend: invalid exif type: not a segment list")
	}

	// Update the tag.
	rootIb, err := sl.ConstructExifBuilder()
	if err != nil {
		return nil, err
	}

	ifdIb, err := exif.GetOrCreateIbFromRootIb(rootIb, ifdPath)
	if err != nil {
		return nil, err
	}

	err = ifdIb.SetStandardWithName(warpMeta, string(metadata))
	if err != nil {
		return nil, err
	}

	// Update the exif segment.
	err = sl.SetExif(rootIb)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = sl.Write(buf)
	if err != nil {
		return nil, err
	}

	bt := buf.Bytes()

	return bt, validateExif(bt)
}

func validateExif(data []byte) error {
	parser := jis.NewJpegMediaParser()

	intfc, err := parser.ParseBytes(data)
	if err != nil {
		return err
	}

	sl, ok := intfc.(*jis.SegmentList)
	if !ok {
		return errors.New("validate: invalid exif type: not a segment list")
	}

	_, _, exifTags, err := sl.DumpExif()
	if err != nil {
		return err
	}

	var isFound bool
	for _, et := range exifTags {
		if et.IfdPath == ifdPath && et.TagName == warpMeta {
			log.Infof("EXIF tag value: %s \n", et.FormattedFirst)
			isFound = true
			break
		}
	}
	if !isFound {
		return errors.New("invalid exif: meta tag not found")
	}
	return nil
}
