package warpnet

import "embed"

//go:embed server/static
var staticFolder embed.FS

func GetStaticFolder() embed.FS {
	return staticFolder
}
