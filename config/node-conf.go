package config

import "strings"

const (
	DatabaseFolder   = "/storage"
	SessionTokenName = "X-SESSION-TOKEN"

	apifyAddr  = "https://api.ipify.org?format=txt"
	ipInfoAddr = "https://ipinfo.io/ip"
	seeIpAddr  = "https://api.seeip.org"

	LogFormat = `
	{"time":"${time_datetime_only}",` +
		`"method":"${method}","host":"${host}","uri":"${uri}",` +
		`"status":${status},"error":"${error}}"` + "\n"
)

var IPProviders = []string{
	apifyAddr,
	ipInfoAddr,
	seeIpAddr,
}

type NodeAddress string

const (
	ExternalNodeAddress NodeAddress = "http://127.0.0.1:6969"
	InternalNodeAddress NodeAddress = "http://127.0.0.1:16969"
)

func (n NodeAddress) Protocol() string {
	return strings.Split(string(n), "://")[0]
}

func (n NodeAddress) Host() string {
	addr := strings.TrimPrefix(string(n), "http://")
	host := strings.Split(addr, ":")[1]
	return host
}
func (n NodeAddress) Port() string {
	addr := strings.TrimPrefix(string(n), "http://")
	return strings.Split(addr, ":")[1]
}
func (n NodeAddress) String() string {
	return string(n)
}
func (n NodeAddress) HostPort() string {
	return strings.TrimPrefix(string(n), "http://")
}
