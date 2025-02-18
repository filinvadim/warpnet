package config

import (
	"flag"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strings"
)

var ConfigFile Config

func init() {
	// Определяем флаги командной строки
	pflag.String("database.dir", "storage", "Database directory name")
	pflag.String("server.host", "localhost", "Server host")
	pflag.String("server.port", "4002", "Server port")
	pflag.String("node.host", "0.0.0.0", "Node host")
	pflag.String("node.port", "4001", "Node port")
	pflag.String("node.network.prefix", "testnet", "Private network prefix")
	pflag.String(
		"node.bootstrap",
		"/ip4/67.207.72.168/tcp/4001/p2p/12D3KooWPn8MyXNAgWdGivV6aM34nQt7FZ2Ai4tujS2NcSnhR9fw,"+
			"/ip4/67.207.72.168/tcp/4001/p2p/12D3KooWMp4ddFBPxm3XfvgBgW5dSUKiPWbzDd8f4vDCqu8gepBb,"+
			"/ip4/67.207.72.168/tcp/4001/p2p/12D3KooWC7TjzaPbBp8N5JDvJrTehGixVwWJhceQxbZb4T5zLjzS",
		"Bootstrap nodes multiaddr list, comma separated",
	)
	pflag.String("logging.level", "INFO", "Logging level")
	pflag.String("version", "0.0.0", "App version")

	// Добавляем поддержку стандартных флагов Go
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Привязываем флаги к viper
	_ = viper.BindPFlags(pflag.CommandLine)

	bootstrap := viper.GetString("node.bootstrap")

	ConfigFile = Config{
		Version: semver.MustParse(strings.TrimSpace(viper.GetString("version"))),
		Node: Node{
			Bootstrap: strings.Split(bootstrap, ","),
			Host:      viper.GetString("node.host"),
			Port:      viper.GetString("node.port"),
			Prefix:    viper.GetString("node.network.prefix"),
		},
		Database: Database{viper.GetString("database.dir")},
		Server: Server{
			Host: viper.GetString("server.host"),
			Port: viper.GetString("server.port"),
		},
		Logging: Logging{Level: viper.GetString("logging.level")},
	}

	fmt.Printf("Config: %#v\n", ConfigFile)
}

type Config struct {
	Version  *semver.Version
	Node     Node
	Database Database
	Server   Server
	Logging  Logging
}
type Node struct {
	Bootstrap []string
	Host      string
	Port      string
	Prefix    string
}
type Database struct {
	DirName string
}
type Logging struct {
	Level  string
	Format string
}
type Server struct {
	Host string
	Port string
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
