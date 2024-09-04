package database

import (
	"encoding/json"
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database/storage"
)

const IPsRepoName = "IPS"

// IPRepo handles operations related to IP addresses
type IPRepo struct {
	db *storage.DB
}

func NewIPRepo(db *storage.DB) *IPRepo {
	return &IPRepo{db: db}
}

// Create adds a new IP address to the database
func (repo *IPRepo) Create(ip api.IPAddress) error {
	data, err := json.Marshal(ip)
	if err != nil {
		return err
	}

	key, err := storage.NewPrefixBuilder(IPsRepoName).AddIPAddress(ip.Ip).Build()
	if err != nil {
		return err
	}
	return repo.db.Set(key, data)
}

// Get retrieves an IP address by its IP
func (repo *IPRepo) Get(ip string) (*api.IPAddress, error) {
	key, err := storage.NewPrefixBuilder(IPsRepoName).AddIPAddress(ip).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var ipAddress api.IPAddress
	err = json.Unmarshal(data, &ipAddress)
	if err != nil {
		return nil, err
	}
	return &ipAddress, nil
}

// Delete removes an IP address by its IP
func (repo *IPRepo) Delete(ip string) error {
	key, err := storage.NewPrefixBuilder(IPsRepoName).AddIPAddress(ip).Build()
	if err != nil {
		return err
	}
	return repo.db.Delete(key)
}
