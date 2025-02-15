package security

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func hashDirectory(dirPath string) ([]byte, error) {
	hasher := sha256.New()
	files := []string{}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(hasher, f)
		f.Close()
		if err != nil {
			return nil, err
		}
	}

	return hasher.Sum(nil), nil
}

func hashFile(file string) ([]byte, error) {
	hasher := sha256.New()

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	_, err = io.Copy(hasher, f)

	if err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

// TODO access this approach
func GetCodebaseHash() ([]byte, error) {
	cmdHash, err := hashDirectory("./cmd")
	if err != nil {
		return nil, err
	}
	dbHash, err := hashDirectory("./database")
	if err != nil {
		return nil, err
	}
	coreHash, err := hashDirectory("./core")
	if err != nil {
		return nil, err
	}
	secHash, err := hashDirectory("./security")
	if err != nil {
		return nil, err
	}
	serverHash, err := hashDirectory("./server")
	if err != nil {
		return nil, err
	}
	gosumHash, err := hashFile("go.sum")
	if err != nil {
		return nil, err
	}

	orderedHashes := bytes.Join([][]byte{cmdHash, dbHash, coreHash, secHash, serverHash, gosumHash}, []byte{})

	return ConvertToSHA256(orderedHashes), nil
}
