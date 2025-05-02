package database

import (
	"encoding/hex"
	"errors"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/security"
	"time"
)

const (
	MediaRepoName     = "/MEDIA"
	ImageSubNamespace = "IMAGES"
	VideoSubNamespace = "VIDEOS"
)

var (
	ErrMediaNotFound    = errors.New("media not found")
	ErrMediaRepoNotInit = errors.New("media repo is not initialized")
)

type MediaStorer interface {
	Set(key storage.DatabaseKey, value []byte) error
	Get(key storage.DatabaseKey) ([]byte, error)
	SetWithTTL(key storage.DatabaseKey, value []byte, ttl time.Duration) error
}

type MediaRepo struct {
	db MediaStorer
}

type (
	Base64Image string
	ImageKey    string
)

func NewMediaRepo(db MediaStorer) *MediaRepo {
	return &MediaRepo{db: db}
}

func (repo *MediaRepo) GetImage(userId, key string) (Base64Image, error) {
	if repo == nil {
		return "", ErrMediaRepoNotInit
	}
	if key == "" || userId == "" {
		return "", ErrMediaNotFound
	}

	mediaKey := storage.NewPrefixBuilder(MediaRepoName).
		AddRootID(ImageSubNamespace).
		AddParentId(userId).
		AddId(key).
		Build()

	data, err := repo.db.Get(mediaKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return "", ErrMediaNotFound
	}

	return Base64Image(data), err
}

func (repo *MediaRepo) SetImage(userId string, img Base64Image) (_ ImageKey, err error) {
	if repo == nil {
		return "", ErrMediaRepoNotInit
	}
	if len(img) == 0 || len(userId) == 0 {
		return "", errors.New("no data for image set")
	}
	h := security.ConvertToSHA256([]byte(img))
	key := hex.EncodeToString(h)

	mediaKey := storage.NewPrefixBuilder(MediaRepoName).
		AddRootID(ImageSubNamespace).
		AddParentId(userId).
		AddId(key).
		Build()

	return ImageKey(key), repo.db.Set(mediaKey, []byte(img))
}

func (repo *MediaRepo) SetForeignImageWithTTL(userId, key string, img Base64Image) error {
	if repo == nil {
		return ErrMediaRepoNotInit
	}
	if len(img) == 0 || len(userId) == 0 {
		return errors.New("no data for image set provided")
	}
	if key == "" {
		return errors.New("no key for image set provided")
	}

	mediaKey := storage.NewPrefixBuilder(MediaRepoName).
		AddRootID(ImageSubNamespace).
		AddParentId(userId).
		AddId(key).
		Build()

	week := time.Hour * 24 * 7
	return repo.db.SetWithTTL(mediaKey, []byte(img), week)
}
