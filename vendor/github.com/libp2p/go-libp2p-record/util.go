package record

import (
	"strings"
	"fmt"
)

// SplitKey takes a key in the form `/$namespace/$path` and splits it into
// `$namespace` and `$path`.
func SplitKey(key string) (string, string, error) {
	if len(key) == 0 || key[0] != '/' {
		fmt.Println(key, "???????????????")
		return "", "", ErrInvalidRecordType
	}

	key = key[1:]

	i := strings.IndexByte(key, '/')
	if i <= 0 {
		fmt.Println(key, "???????????????", strings.IndexByte(key, '/'))
		return "", "", ErrInvalidRecordType
	}

	return key[:i], key[i+1:], nil
}
