package database

import (
	"errors"
	"github.com/filinvadim/warpnet/database/storage"
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
}

type MediaObject struct {
	data []byte
	meta []byte
}

type MediaRepo struct {
	db MediaStorer
}

func NewMediaRepo(db MediaStorer) *MediaRepo {
	return &MediaRepo{db: db}
}

func (repo *MediaRepo) GetImage() ([]byte, error) {
	if repo == nil {
		return nil, ErrMediaRepoNotInit
	}

	mediaKey := storage.NewPrefixBuilder(MediaRepoName).
		AddRootID(ImageSubNamespace).
		Build()

	data, err := repo.db.Get(mediaKey)
	if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		return nil, ErrMediaNotFound
	}

	return data, nil
}

func (repo *MediaRepo) SetImage(img []byte) error {
	if repo == nil {
		return ErrMediaRepoNotInit
	}
	mediaKey := storage.NewPrefixBuilder(MediaRepoName).
		AddRootID(ImageSubNamespace).
		Build()

	return repo.db.Set(mediaKey, img)
}
