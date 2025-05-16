/*

Warpnet - Decentralized Social Network
Copyright (C) 2025 Vadim Filin, https://github.com/Warp-net,
<github.com.mecdy@passmail.net>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.

WarpNet is provided “as is” without warranty of any kind, either expressed or implied.
Use at your own risk. The maintainers shall not be liable for any damages or data loss
resulting from the use or misuse of this software.
*/

package config

import (
	"flag"
	"fmt"
	"github.com/Masterminds/semver/v3"
	root "github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strings"
)

const (
	testNetNetwork = "testnet"
	warpnetNetwork = "warpnet"
)

const noticeTemplate = " %s version %s. Copyright (C) <%s> <%s>. This program comes with ABSOLUTELY NO WARRANTY; This is free software, and you are welcome to redistribute it under certain conditions.\n\n\n"

var mainnetBootstrapNodes = []string{
	"/ip4/207.154.221.44/tcp/4001/p2p/12D3KooWMKZFrp1BDKg9amtkv5zWnLhuUXN32nhqMvbtMdV2hz7j",
	"/ip4/207.154.221.44/tcp/4002/p2p/12D3KooWSjbYrsVoXzJcEtmgJLMVCbPXMzJmNN1JkEZB9LJ2rnmU",
	"/ip4/207.154.221.44/tcp/4003/p2p/12D3KooWNXSGyfTuYc3JznW48jay73BtQgHszWfPpyF581EWcpGJ",
}

var configSingleton config

func init() {
	pflag.String("database.dir", "storage", "Database directory name")
	pflag.String("server.host", "localhost", "Server host")
	pflag.String("server.port", "4002", "Server port")
	pflag.String("node.host", "0.0.0.0", "Node host")
	pflag.String("node.port", "4001", "Node port")
	pflag.String("node.seed", "", "Bootstrap node seed for deterministic ID generation (random string)")
	pflag.String("node.network", "testnet", "Private network. Use 'testnet' for testing env.")
	pflag.String("node.bootstrap", "", "Bootstrap nodes multiaddr list, comma separated")
	pflag.Bool("node.inmemory", false, "Bootstrap node runs without persistent storage")
	pflag.String("node.metrics.server", "", "Metrics server address")
	pflag.String("logging.level", "info", "Logging level")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	_ = viper.BindPFlags(pflag.CommandLine)

	bootstrapAddrList := make([]string, 0, len(mainnetBootstrapNodes))
	bootstrapAddrs := viper.GetString("node.bootstrap")

	split := strings.Split(bootstrapAddrs, ",")
	if len(split) != 0 {
		bootstrapAddrList = split
	}

	network := strings.TrimSpace(viper.GetString("node.network"))
	if len(bootstrapAddrList) == 0 && network == warpnetNetwork {
		bootstrapAddrList = mainnetBootstrapNodes
	}

	version := root.GetVersion()

	fmt.Printf(noticeTemplate, strings.ToUpper(warpnet.WarpnetName), version, "2025", "Vadim Filin")

	configSingleton = config{
		Version: semver.MustParse(strings.TrimSpace(string(version))),
		Node: node{
			Bootstrap:  bootstrapAddrList,
			Seed:       strings.TrimSpace(viper.GetString("node.seed")),
			Host:       viper.GetString("node.host"),
			Port:       viper.GetString("node.port"),
			Network:    network,
			IsInMemory: viper.GetBool("node.inmemory"),
			Metrics: metrics{
				Server: viper.GetString("node.metrics.server"),
			},
		},
		Database: database{strings.TrimSpace(viper.GetString("database.dir"))},
		Server: server{
			Host: viper.GetString("server.host"),
			Port: viper.GetString("server.port"),
		},
		Logging: logging{Level: strings.TrimSpace(viper.GetString("logging.level"))},
	}
}

func Config() config {
	return configSingleton
}

type config struct {
	Version  *semver.Version
	Node     node
	Database database
	Server   server
	Logging  logging
}
type node struct {
	Bootstrap  []string
	Host       string
	Port       string
	Network    string
	IsInMemory bool
	Metrics    metrics
	Seed       string
}

type metrics struct {
	Server string
}
type database struct {
	DirName string
}
type logging struct {
	Level  string
	Format string
}
type server struct {
	Host string
	Port string
}

func (n node) IsTestnet() bool {
	return n.Network == testNetNetwork
}

func (n node) AddrInfos() (infos []warpnet.PeerAddrInfo, err error) {
	for _, addr := range n.Bootstrap {
		maddr, err := warpnet.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		addrInfo, err := warpnet.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *addrInfo)
	}
	return infos, nil
}
