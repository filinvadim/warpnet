package security

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
)

const SelfHashConsensusKey = "selfhash"

type FileSystem interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
	Open(name string) (fs.File, error)
}

type SelfHash []byte

func (s SelfHash) Validate(k, v string) error {
	if k != SelfHashConsensusKey {
		return nil
	}

	if v == s.String() {
		return nil
	}
	return errors.New("invalid self hash")
}

func (s SelfHash) String() string {
	return fmt.Sprintf("%x", []byte(s))
}

func walkAndHash(fsys FileSystem, dir string, h io.Writer) error {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", dir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		path := dir + "/" + entry.Name()
		if dir == "." {
			path = entry.Name()
		}

		pathHash := sha256.Sum256([]byte(path))
		h.Write(pathHash[:])

		if entry.IsDir() {
			err := walkAndHash(fsys, path, h)
			if err != nil {
				return fmt.Errorf("walk and hash %s: %w", path, err)
			}
		} else {
			fileHash, err := hashFile(fsys, path)
			if err != nil {
				return fmt.Errorf("file hash %s: %w", path, err)
			}
			h.Write(fileHash)
		}
	}

	return nil
}

func hashFile(fsys FileSystem, path string) ([]byte, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	h := sha256.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func GetCodebaseHash(codebase FileSystem) (SelfHash, error) {
	h := sha256.New()

	err := walkAndHash(codebase, ".", h)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
