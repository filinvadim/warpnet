// #nosec
package warpnet

import (
	"embed"
	_ "embed"
)

//go:embed config.yml
var configFile []byte

func GetConfigFile() []byte {
	return configFile
}

//go:embed *.go */*.go */*/*.go */*/*/*.go */*/*/*/*.go
var codeBase embed.FS

func GetCodeBase() embed.FS {
	return codeBase
}
