package config

import (
	"github.com/Masterminds/semver/v3"
)

type Config struct {
	Version  semver.Version `yaml:"version"`
	Node     Node           `yaml:"node"`
	Server   Server         `yaml:"server"`
	Database Database       `yaml:"database"`
}

type Node struct {
	SeedID        string   `yaml:"seed_id"`
	BootstrapAddr []string `yaml:"bootstrap_addr"`
	ListenAddrs   []string `yaml:"listen_addrs"`
	Logging       Logging  `yaml:"logging"`
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
