package security

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"io"
	"io/fs"
	"sort"
	"strconv"
)

type FileSystem interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
	Open(name string) (fs.File, error)
}

type PSK []byte

func (s PSK) String() string {
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

func getCodebaseHash(codebase FileSystem) ([]byte, error) {
	h := sha256.New()

	err := walkAndHash(codebase, ".", h)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

const spbFounding = -((int64(133129) << 16) + 51200)

func generateAnchoredEntropy() []byte {
	spbFoundingStr := strconv.FormatInt(spbFounding, 10)
	input := []byte(spbFoundingStr)
	for i := 0; i < 1000; i++ {
		sum := sha256.Sum256(input)
		input = sum[:]
	}
	return input
}

// GeneratePSK is not secure TODO
func GeneratePSK(codebase FileSystem, v *semver.Version) (PSK, error) {
	if config.ConfigFile.Node.Prefix == "testnet" {
		return []byte{}, nil
	}
	if codebase == nil || v == nil {
		return nil, errors.New("psk: codebase or version required")
	}
	codeHash, err := getCodebaseHash(codebase)
	if err != nil {
		return nil, err
	}
	entropy := generateAnchoredEntropy()
	majorStr := strconv.FormatInt(int64(v.Major()), 10)

	seed := append([]byte(config.ConfigFile.Node.Prefix), codeHash...)
	seed = append(seed, []byte(majorStr)...)
	seed = append(seed, entropy...)
	return ConvertToSHA256(seed), nil
}
