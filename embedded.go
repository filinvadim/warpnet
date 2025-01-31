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

//go:embed warpnet.sig
var signature []byte

func GetSignature() []byte {
	return signature
}

//go:embed public.pem
var publicKey []byte

func GetPublicKey() []byte {
	return publicKey
}
