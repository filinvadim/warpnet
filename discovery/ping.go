package discovery

import (
	"context"
	"errors"
	"fmt"
	genClient "github.com/filinvadim/dWighter/api/client"
	"github.com/filinvadim/dWighter/api/server"
	"github.com/filinvadim/dWighter/client"
	"github.com/filinvadim/dWighter/database"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var PresetNodeAddress = "127.0.0.1:6969"

type DiscoveryLogger interface {
	Errorf(format string, args ...interface{})
}

type (
	PingIP   = string
	PingPort = string
)

// DiscoveryService handles the discovery and health checking of nodes
type DiscoveryService struct {
	nodeRepo *database.NodeRepo
	cache    map[PingIP]PingPort
	mx       *sync.RWMutex
	cli      *client.Client

	l DiscoveryLogger
}

func NewDiscoveryService(
	cli *client.Client,
	nodeRepo *database.NodeRepo,
	l DiscoveryLogger,
) (*DiscoveryService, error) {
	nodes, err := nodeRepo.List()
	if err != nil {
		return nil, err
	}
	cache := make(map[PingIP]PingPort)
	for _, n := range nodes {
		cache[n.Ip] = n.Port
	}

	splitted := strings.Split(PresetNodeAddress, ":")
	if len(splitted) == 2 {
		cache[splitted[0]] = splitted[1]
	}
	ds := &DiscoveryService{
		nodeRepo: nodeRepo,
		cache:    cache,
		cli:      cli,
		l:        l,
		mx:       new(sync.RWMutex),
	}
	return ds, nil
}

// StartDiscovery initiates the process of periodically pinging nodes
func (ds *DiscoveryService) StartDiscovery() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ds.pingAllNodes(); err != nil {
				ds.l.Errorf("discovery: %v", err)
			}
		}
	}
}

// pingAllNodes pings all nodes in the database and updates the cache
func (ds *DiscoveryService) pingAllNodes() error {
	ds.mx.RLock()
	defer ds.mx.RUnlock()

	var g, _ = errgroup.WithContext(context.Background())
	for ip, port := range ds.cache {
		g.Go(func() error {
			return ds.pingNode(ip, port)
		})
	}
	return g.Wait()
}

// pingNode sends a ping request to a single node and updates the cache
func (ds *DiscoveryService) pingNode(ip, port string) error {
	resp, err := ds.cli.Ping(ip + ":" + port)
	if err != nil {
		ds.removeNodeFromCacheAndDB(ip)
		return fmt.Errorf("pinging node %s: %v", ip, err)
	}
	if resp == nil {
		return errors.New("empty response")
	}

	ds.updateCache(*resp)

	n, err := ds.nodeRepo.GetByIP(ip)
	if err != nil && !errors.Is(err, database.ErrNodeNotFound) {
		return err
	}
	if errors.Is(err, database.ErrNodeNotFound) {
		n = new(server.Node)
	}
	n.LastSeen = time.Now()
	n.Latency = &resp.Latency

	_, err = ds.nodeRepo.Create(*n)
	return err
}

// updateCache updates the cache with the ping response from a node
func (ds *DiscoveryService) updateCache(pingResponse genClient.PingResponse) {
	ds.mx.Lock()
	defer ds.mx.Unlock()
	ds.cache[pingResponse.Ip] = pingResponse.Port
}

// removeNodeFromCacheAndDB removes a node from both the cache and the database
func (ds *DiscoveryService) removeNodeFromCacheAndDB(ip string) {
	ds.mx.Lock()
	defer ds.mx.Unlock()

	delete(ds.cache, ip)
	err := ds.nodeRepo.DeleteByIP(ip)
	if err != nil {
		ds.l.Errorf("Error deleting node %s from database: %v", ip, err)
	}
}

func getPublicIP() {
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		fmt.Println("Ошибка при запросе IP адреса:", err)
		return
	}
	defer resp.Body.Close()

	// Читаем ответ
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Ошибка при чтении ответа:", err)
		return
	}

	// Выводим внешний IP адрес
	fmt.Printf("Ваш внешний IP: %s\n", string(ip))
}
