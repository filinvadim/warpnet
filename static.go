package warpnet

import (
	"embed"
	"io/fs"
)

//go:embed server/frontend/dist
var static embed.FS

func GetStaticFS() fs.FS {
	return static
}
