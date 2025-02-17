package dhash_table

import (
	"fmt"
	"github.com/mr-tron/base58/base58"
	mh "github.com/multiformats/go-multihash"
	"log"
	"strings"
	"testing"
)

func TestBuildDHTKey(t *testing.T) {
	key := buildDHTKey("test")
	fmt.Printf("Generated key: %s\n", key)

	// Извлекаем base58-часть
	parts := strings.Split(key, "/pk/")
	if len(parts) != 2 {
		log.Fatal("invalid key format")
	}

	// Декодируем base58
	decoded, err := base58.Decode(parts[1])
	if err != nil {
		log.Fatalf("failed to decode base58: %v", err)
	}

	// Проверяем, что это валидный мультихеш
	_, err = mh.Decode(decoded)
	if err != nil {
		log.Fatalf("invalid multihash: %v", err)
	}
}
