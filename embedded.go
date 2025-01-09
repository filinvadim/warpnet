package warpnet

import (
	"embed"
	"io/fs"
)

// NOTE this file always must be in a root dir of project

//go:embed server/frontend/dist
var static embed.FS

func GetStaticFS() fs.FS {
	return static
}

//go:embed config.yml
var configFile []byte

func GetConfigFile() []byte {
	return configFile
}
