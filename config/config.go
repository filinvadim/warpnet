package config

import (
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version  semver.Version `yaml:"version"`
	Node     Node           `yaml:"node"`
	Database Database       `yaml:"database"`
	Server   Server         `yaml:"server"`
}
type Node struct {
	SeedID      int      `yaml:"seed_id"`
	Bootstrap   []string `yaml:"bootstrap"`
	ListenAddrs []string `yaml:"listen_addrs"`
	Logging     Logging  `yaml:"logging"`
}
type Database struct {
	DirName string `yaml:"dirName"`
}
type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}
type Server struct {
	Host    string  `yaml:"host"`
	Port    int     `yaml:"port"`
	Logging Logging `yaml:"logging"`
}

func (n *Node) AddrInfos() (infos []peer.AddrInfo, err error) {
	for _, addr := range n.Bootstrap {
		ai, err := peer.AddrInfoFromString(addr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *ai)
	}
	return infos, nil
}

func GetConfig() (Config, error) {
	var conf Config
	bt := warpnet.GetConfigFile()
	if err := yaml.Unmarshal(bt, &conf); err != nil {
		return Config{}, err
	}
	return conf, nil
}
