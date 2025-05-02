package relay

import (
	"context"
	"errors"
	"github.com/filinvadim/warpnet/core/warpnet"
	golog "github.com/ipfs/go-log/v2"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

/*
  The libp2p Relay v2 library is an improved version of the connection relay mechanism in libp2p,
  designed for scenarios where nodes cannot establish direct connections due to NAT, firewalls,
  or other network restrictions.

  In a standard P2P network, nodes are expected to connect directly to each other. However, if one
  or both nodes are behind NAT or a firewall, direct connections may be impossible. In such cases,
  Relay v2 enables traffic to be relayed through intermediary nodes (relay nodes), allowing communication
  even in restricted network environments.

  ### **Key Features of libp2p Relay v2:**
  - **Automatic Relay Discovery and Usage**
    - If a node cannot establish a direct connection, it automatically finds a relay node.
  - **Operating Modes:**
    - **Active relay:** A node can act as a relay for others.
    - **Passive relay:** A node uses relay services without providing its own resources.
  - **Hole Punching**
    - Uses NAT traversal techniques (e.g., **DCUtR – Direct Connection Upgrade through Relay**)
      to attempt a direct connection before falling back to a relay.
  - **More Efficient Routing**
    - Relay v2 selects low-latency routes instead of simply forwarding all traffic through a single node.
  - **Bandwidth Optimization**

  ### **When is Relay v2 Needed?**
  - Nodes do not have a public IP address and are behind NAT.
  - A connection is required between network participants who cannot connect directly.
  - Reducing relay server load is important (compared to Relay v1).
  - **DCUtR is used** to attempt NAT traversal before falling back to a relay.
*/

func NewRelay(node warpnet.P2PNode) (*relayv2.Relay, error) {
	golog.SetLogLevel("autorelay", "INFO")
	relay, err := relayv2.New(
		node,
		relayv2.WithLimit(&relayv2.RelayLimit{
			Duration: 5 * time.Minute,
			Data:     1 << 19, // 512kb
		}),
	)
	return relay, err
}

// TODO
func waitRelayReadiness(node warpnet.P2PNode) error {
	ctx, cancelF := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelF()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, addr := range node.Addrs() {
				if strings.Contains(addr.String(), "p2p-circuit") {
					log.Infoln("relay: is active, address:", addr)
					return nil
				}
			}
		case <-ctx.Done():
			return errors.New("relay readiness timeout")
		}
	}
}
