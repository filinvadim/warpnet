package config

import (
	"github.com/Masterminds/semver/v3"
	"github.com/libp2p/go-libp2p/core/peer"
	"os"
)

type Config struct {
	Version  semver.Version `yaml:"version"`
	Node     Node           `yaml:"node"`
	Server   Server         `yaml:"server"`
	Database Database       `yaml:"database"`
}

type Node struct {
	SeedID         string   `yaml:"seed_id"`
	BootstrapAddrs []string `yaml:"bootstrap_addrs"`
	ListenAddrs    []string `yaml:"listen_addrs"`
	Logging        Logging  `yaml:"logging"`
}

func (n *Node) GetSeedID() string {
	if id := os.Getenv("NODE_ID"); id != "" {
		return id
	}
	return n.SeedID
}

func (n *Node) AddrInfos() (infos []peer.AddrInfo, err error) {
	for _, addr := range n.BootstrapAddrs {
		ai, err := peer.AddrInfoFromString(addr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *ai)
	}
	return infos, nil
}

type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type Server struct {
	Host    string  `yaml:"addr"`
	Port    int     `yaml:"port"`
	Logging Logging `yaml:"logging"`
}

type Database struct {
	Dir string `yaml:"path"`
}
