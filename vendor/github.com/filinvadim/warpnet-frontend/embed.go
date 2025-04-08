package embedded

import (
	"embed"
	"io/fs"
)

// This file embeds the whole frontend to a single Golang binary file. DO NOT REMOVE.

//go:embed dist
var static embed.FS

func GetStaticEmbedded() fs.FS {
	return static
}
