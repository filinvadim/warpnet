// #nosec
package warpnet

import (
	"embed"
	_ "embed"
)

//go:embed *.go */*.go */*/*.go */*/*/*.go */*/*/*/*.go
var codeBase embed.FS

func GetCodeBase() embed.FS {
	return codeBase
}

//go:embed version
var version []byte

func GetVersion() []byte {
	return version
}
