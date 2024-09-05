package discovery

import (
	"errors"
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database"
	"sync"
	"time"
)

// DiscoveryService manages IP addresses
type DiscoveryService struct {
	ips   map[string]api.IPAddress
	mutex sync.RWMutex
}

// NewDiscoveryService creates a new DiscoveryService instance
func NewDiscoveryService(ipRepo database.IPRepo) *DiscoveryService {
	return &DiscoveryService{
		ips: make(map[string]api.IPAddress),
	}
}

// AddNode adds a new IP address to the service
func (ds *DiscoveryService) AddNode(ipAddress api.IPAddress) error {
	// Validate IP address format
	if ipAddress.Ip == "" {
		return errors.New("invalid IP address")
	}

	// Update the last seen timestamp
	ipAddress.LastSeen = time.Now()

	// Lock and add the IP address to the map
	ds.mutex.Lock()
	ds.ips[ipAddress.Ip] = ipAddress
	ds.mutex.Unlock()

	return nil
}

// GetNodes retrieves the list of all IP addresses
func (ds *DiscoveryService) GetNodes() []api.IPAddress {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	// Convert the map of IP addresses to a slice
	ipList := make([]api.IPAddress, 0, len(ds.ips))
	for _, ip := range ds.ips {
		ipList = append(ipList, ip)
	}
	return ipList
}

// RemoveNode removes an IP address from the service
func (ds *DiscoveryService) RemoveNode(ip string) error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// Check if the IP exists in the map
	if _, exists := ds.ips[ip]; !exists {
		return errors.New("IP address not found")
	}

	// Remove the IP from the map
	delete(ds.ips, ip)
	return nil
}

// UpdateNode updates an existing IP address entry
func (ds *DiscoveryService) UpdateNode(ip string, newInfo api.IPAddress) error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// Check if the IP exists in the map
	if _, exists := ds.ips[ip]; !exists {
		return errors.New("IP address not found")
	}

	// Update the IP information
	newInfo.LastSeen = time.Now()
	ds.ips[ip] = newInfo
	return nil
}
