// #nosec
package warpnet

import (
	_ "embed"
)

//go:embed config.yml
var configFile []byte

func GetConfigFile() []byte {
	return configFile
}

//go:embed version
var version []byte

func GetVersion() string {
	return string(version)
}
