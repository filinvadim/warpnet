package embedded

import (
	"embed"
	"io/fs"
)

//go:embed dist
var static embed.FS

func GetStaticEmbedded() fs.FS {
	return static
}
