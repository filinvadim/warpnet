package dhash_table

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"io"
	"strings"
	"time"
)

/*
  Distributed Hash Table (DHT) is a distributed hash table used for decentralized
  data storage and lookup in peer-to-peer (P2P) networks. Instead of storing data on a single server,
  DHT distributes it across multiple nodes.

  DHT solves three main tasks:
  1. Routing — enables efficient lookup of nodes storing specific keys.
  2. Data storage — each node is responsible for a portion of the key space.
  3. Key-based lookup — provides fast access to data without a central server.

  DHT is used in BitTorrent, IPFS, Ethereum, as well as in P2P messengers and other decentralized applications.

  The go-libp2p-kad-dht library is an implementation of Kademlia DHT for libp2p.
  It allows peer-to-peer nodes to exchange data and discover each other without centralized servers.

  Key features of go-libp2p-kad-dht:
  - **Kademlia Algorithm**
    - Implements Kademlia DHT, one of the most widely used algorithms for distributed hash tables.
  - **Node and data lookup in a P2P network**
    - Enables finding nodes and querying them for data by key.
  - **Flexible routing**
    - Optimized for dynamic networks where nodes frequently join and leave.
  - **Support for PubSub and IPFS**
    - Used in IPFS and applicable to P2P messengers and decentralized applications.
  - **Key hashing**
    - Distributes the key space across nodes, ensuring balanced load distribution.

  DHT is well-suited for decentralized applications that require distributed search without a single point of failure,
  P2P networks where nodes frequently connect and disconnect, and data exchange between nodes without a central server.

  The go-libp2p-kad-dht library is useful for finding other nodes in a libp2p network,
  implementing decentralized content lookup (as in IPFS), and enabling efficient routing in a distributed network.
*/

var ErrDHTMisconfigured = errors.New("DHT is misconfigured")

type RoutingStorer interface {
	warpnet.WarpBatching
}

type ProviderStorer interface {
	AddProvider(ctx context.Context, key []byte, prov peer.AddrInfo) error
	GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error)
	io.Closer
}

type DistributedHashTable struct {
	ctx           context.Context
	db            RoutingStorer
	providerStore ProviderStorer
	boostrapNodes []warpnet.PeerAddrInfo
	addF          discovery.DiscoveryHandler
	removeF       discovery.DiscoveryHandler
	dht           *dht.IpfsDHT
	codeHash      []byte
}

func defaultNodeRemovedCallback(id warpnet.WarpPeerID) {
	log.Debugln("dht: node removed", id)
}

func defaultNodeAddedCallback(id warpnet.WarpPeerID) {
	log.Debugln("dht: node added", id)
}

func NewDHTable(
	ctx context.Context,
	db RoutingStorer,
	providerStore ProviderStorer,
	codeHash []byte,
	addF discovery.DiscoveryHandler,
	removeF discovery.DiscoveryHandler,
) *DistributedHashTable {
	bootstrapAddrs, _ := config.ConfigFile.Node.AddrInfos()
	return &DistributedHashTable{
		ctx:           ctx,
		db:            db,
		providerStore: providerStore,
		boostrapNodes: bootstrapAddrs,
		addF:          addF,
		removeF:       removeF,
		codeHash:      codeHash,
	}
}

func (d *DistributedHashTable) StartRouting(n warpnet.P2PNode) (_ warpnet.WarpPeerRouting, err error) {
	infos, err := config.ConfigFile.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	for _, info := range infos {
		fmt.Printf("dht: bootstrap node %s\n", info.String())
	}

	dhTable, err := dht.New(
		d.ctx, n,
		dht.Mode(dht.ModeServer),
		dht.AddressFilter(localHostAddressFilter),
		dht.ProtocolPrefix(protocol.ID("/"+config.ConfigFile.Node.Prefix)),
		dht.Datastore(d.db),
		dht.MaxRecordAge(time.Hour*24*365),
		dht.RoutingTableRefreshPeriod(time.Hour*24),
		dht.RoutingTableRefreshQueryTimeout(time.Hour*24),
		dht.BootstrapPeers(infos...),
		dht.ProviderStore(d.providerStore),
		dht.RoutingTableLatencyTolerance(time.Hour*24),
	)
	if err != nil {
		log.Infof("new dht: %v\n", err)
		return nil, err
	}

	dhTable.RoutingTable().PeerAdded = defaultNodeAddedCallback
	if d.addF != nil {
		dhTable.RoutingTable().PeerAdded = func(id peer.ID) {
			log.Infof("dht: new peer added: %s", id)
			info := peer.AddrInfo{ID: id}
			d.addF(info)
		}
	}
	dhTable.RoutingTable().PeerRemoved = defaultNodeRemovedCallback
	if d.removeF != nil {
		dhTable.RoutingTable().PeerRemoved = func(id peer.ID) {
			info := peer.AddrInfo{ID: id}
			d.removeF(info)
		}
	}

	d.dht = dhTable

	go func() {
		if err := dhTable.Bootstrap(d.ctx); err != nil {
			log.Errorf("dht: bootstrap: %s", err)
		}
		d.correctPeerIdMismatch(d.boostrapNodes)
	}()

	<-dhTable.RefreshRoutingTable()

	log.Infoln("dht: routing started")

	return dhTable, nil
}

func (d *DistributedHashTable) correctPeerIdMismatch(boostrapNodes []warpnet.PeerAddrInfo) {
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second) // common timeout
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	for _, addr := range boostrapNodes {
		addr := addr // this is important!
		g.Go(func() error {
			localCtx, localCancel := context.WithTimeout(ctx, 1*time.Second) // local timeout
			defer localCancel()

			err := d.dht.Ping(localCtx, addr.ID)
			if err == nil {
				return nil
			}
			var pidErr sec.ErrPeerIDMismatch
			if !errors.As(err, &pidErr) {
				return nil
			}

			d.dht.RoutingTable().RemovePeer(pidErr.Expected)
			d.dht.Host().Peerstore().ClearAddrs(pidErr.Expected)
			d.dht.Host().Peerstore().AddAddrs(pidErr.Actual, addr.Addrs, warpnet.PermanentAddrTTL)
			log.Infof("dht: peer id corrected from %s to %s", pidErr.Expected, pidErr.Actual)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Errorf("dht: mismatch: waitgroup: %v", err)
	}
}

func (d *DistributedHashTable) Close() {
	if d == nil || d.dht == nil {
		return
	}
	if err := d.dht.Close(); err != nil {
		log.Errorf("dht: table close: %v\n", err)
	}
	d.dht = nil
	log.Infoln("dht: table closed")
}

func localHostAddressFilter(multiaddrs []multiaddr.Multiaddr) (filtered []multiaddr.Multiaddr) {
	for _, addr := range multiaddrs {
		if addr == nil {
			continue
		}
		if strings.Contains(addr.String(), "localhost") {
			continue
		}
		if strings.HasPrefix(addr.String(), "127.0.0.1") {
			continue
		}

		filtered = append(filtered, addr)
	}
	return filtered
}

//const (
//	requestPrefix        = "request-psk"
//	responsePrefix       = "response-psk"
//	Rejected       int32 = -1
//	Expired        int32 = -2
//)
//
//type pskExchange struct {
//	requestKey  string
//	responseKey string
//}
//
//func (d *DistributedHashTable) RequestPSK() (string, error) {
//	if d == nil || d.dht == nil {
//		return "", nil
//	}
//
//	var (
//		timeout   = 30 * time.Second
//		exchanges []pskExchange
//		ownID     = d.dht.PeerID().String()
//		timer     = time.NewTimer(timeout)
//	)
//	defer timer.Stop()
//
//	defer func() {
//		for _, ex := range exchanges {
//			_ = d.dht.PutValue(d.ctx, ex.requestKey, []byte(string(Expired)))
//			_ = d.dht.PutValue(d.ctx, ex.responseKey, []byte(string(Expired)))
//		}
//	}()
//
//	reHashedCodeHash := security.ConvertToSHA256(d.codeHash)
//
//	ctx, cancel := context.WithTimeout(d.ctx, timeout*time.Duration(len(d.boostrapNodes)))
//	defer cancel()
//
//	for {
//		select {
//		case <-ctx.Done():
//			return "", ctx.Err()
//		case <-timer.C:
//			return "", errors.New("request PSK timed out")
//		default:
//			for _, info := range d.boostrapNodes {
//				if ctx.Err() != nil {
//					return "", ctx.Err()
//				}
//				bootstrapID := info.ID.String()
//				requestKey := buildDHTKey(requestPrefix, bootstrapID, ownID)
//				responseKey := buildDHTKey(responsePrefix, bootstrapID, ownID)
//
//				log.Infof("DHT key components: prefix=%s, bootstrapID=%s, ownID=%s",
//					requestPrefix, bootstrapID, ownID)
//				log.Infof("Generated DHT key: %s", requestKey)
//				err := d.dht.PutValue(ctx, requestKey, reHashedCodeHash)
//				if errors.Is(err, kbucket.ErrLookupFailure) {
//					time.Sleep(time.Millisecond * 100)
//					continue
//				}
//				if err != nil {
//					log.Errorf("dht: psk request put: %v \n", err)
//					return "", err
//				}
//
//				log.Infoln("dht: psk request put successfully")
//
//				exchanges = append(exchanges, pskExchange{requestKey, responseKey})
//
//				value, err := d.dht.GetValue(ctx, responseKey)
//				if err != nil {
//					log.Warnf("dht: request psk from %s: %v\n", bootstrapID, err)
//					continue
//				}
//				if bytes.ContainsRune(value, Rejected) {
//					return "", errors.New("dht: PSK request rejected")
//				}
//				if len(value) == 0 {
//					log.Warnf("dht: request psk from %s: empty psk", value)
//					continue
//				}
//				log.Infoln("dht: psk response collected successfully")
//
//				data, err := security.DecryptAES(value, d.codeHash)
//				if err != nil {
//					return "", fmt.Errorf("decrypt psk: %w", err)
//				}
//				return string(data), nil
//			}
//		}
//	}
//}
//
//func (d *DistributedHashTable) sharePSK(id warpnet.WarpPeerID, currentPSK []byte) {
//	if d == nil || d.dht == nil {
//		return
//	}
//
//	ctx, cancel := context.WithTimeout(d.ctx, time.Minute)
//	defer cancel()
//
//	log.Infof("dht: share PSK called for %s", id)
//	defer log.Infof("dht: share PSK call finished for %s", id)
//
//	var bootstrapID = d.dht.PeerID().String()
//
//	requestKey := buildDHTKey(requestPrefix, bootstrapID, id.String())
//	value, err := d.dht.GetValue(ctx, requestKey)
//	if errors.Is(err, context.DeadlineExceeded) {
//		log.Warnf("dht: PSK collect request timed out")
//		return
//	}
//	if err != nil {
//		log.Errorf("dht: find psk request : %v\n", err)
//		return
//	}
//
//	if bytes.ContainsRune(value, Expired) {
//		log.Infoln("expired psk request")
//		return
//	}
//
//	responseKey := buildDHTKey(responsePrefix, bootstrapID, id.String())
//	existingResp, err := d.dht.GetValue(ctx, responseKey)
//	if err != nil {
//		log.Errorf("dht: find existing psk response : %v\n", err)
//		return
//	}
//	if len(existingResp) > 0 && !bytes.ContainsRune(existingResp, Rejected) && !bytes.ContainsRune(existingResp, Expired) {
//		log.Infoln("dht: found existing psk response", string(existingResp))
//		return // already serviced
//	}
//
//	reHashedCodeHash := security.ConvertToSHA256(d.codeHash)
//
//	if !bytes.Equal(value, reHashedCodeHash) {
//		if err := d.dht.PutValue(ctx, responseKey, []byte(string(Rejected))); err != nil {
//			log.Errorf("dht: respond psk: %v\n", err)
//		}
//		return
//	}
//
//	if currentPSK == nil || d.codeHash == nil {
//		panic("invalid codeHash or psk")
//	}
//
//	ecryptedPSK, err := security.EncryptAES(currentPSK, d.codeHash)
//	if err != nil {
//		log.Errorf("dht: encrypt psk: %v\n", err)
//		if err := d.dht.PutValue(ctx, responseKey, []byte(string(Rejected))); err != nil {
//			log.Errorf("dht: respond psk: %v\n", err)
//		}
//		return
//	}
//
//	if err := d.dht.PutValue(ctx, responseKey, ecryptedPSK); err != nil {
//		log.Errorf("dht: respond psk: %v\n", err)
//	}
//
//	log.Infof("dht: PSK shared for %s", id)
//	return
//}
