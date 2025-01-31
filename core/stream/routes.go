package stream

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	"strings"
)

const (
	PairPostPrivate WarpRoute = "/private/post/pair/1.0.0"
)

type WarpRoute string

func (r WarpRoute) ProtocolID() protocol.ID {
	return protocol.ID(r)
}

func (r WarpRoute) String() string {
	return string(r)
}

func (r WarpRoute) IsPrivate() bool {
	return strings.Contains(string(r), "private")
}

func (r WarpRoute) IsGet() bool {
	return strings.Contains(string(r), "get")
}

func IsValidRoute(route string) bool { // TODO
	if !strings.HasPrefix(route, "/") {
		return false
	}
	if !(strings.Contains(route, "get") ||
		strings.Contains(route, "delete") ||
		strings.Contains(route, "post")) {
		return false
	}
	if !(strings.Contains(route, "private") ||
		strings.Contains(route, "public")) {
		return false
	}
	return true
}

type WarpRoutes []WarpRoute

func (rs WarpRoutes) FromRoutesToPrIDs() []protocol.ID {
	prIDs := make([]protocol.ID, 0, len(rs))
	for _, r := range rs {
		prIDs = append(prIDs, r.ProtocolID())
	}
	return prIDs
}

func FromPrIDToRoute(prID protocol.ID) WarpRoute {
	return WarpRoute(prID)
}

func FromPrIDToRoutes(prIDs []protocol.ID) WarpRoutes {
	rs := make(WarpRoutes, 0, len(prIDs))
	for _, p := range prIDs {
		rs = append(rs, WarpRoute(p))
	}
	return rs
}
