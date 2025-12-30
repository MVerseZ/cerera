package mesh

import (
	"fmt"
	"sync"

	"github.com/cerera/internal/cerera/types"
)

// Route represents a route to a peer
type Route struct {
	Destination types.Address
	NextHop     types.Address
	Hops        int
	LastUpdate  int64 // Unix timestamp
}

// RoutingTable manages routes to peers
type RoutingTable struct {
	mu     sync.RWMutex
	routes map[types.Address]*Route
}

// NewRoutingTable creates a new routing table
func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		routes: make(map[types.Address]*Route),
	}
}

// AddRoute adds or updates a route
func (rt *RoutingTable) AddRoute(dest, nextHop types.Address, hops int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	rt.routes[dest] = &Route{
		Destination: dest,
		NextHop:     nextHop,
		Hops:        hops,
		LastUpdate:  0, // Will be set by caller if needed
	}
}

// GetRoute retrieves a route to a destination
func (rt *RoutingTable) GetRoute(dest types.Address) (*Route, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	route, ok := rt.routes[dest]
	return route, ok
}

// RemoveRoute removes a route
func (rt *RoutingTable) RemoveRoute(dest types.Address) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.routes, dest)
}

// GetAllRoutes returns all routes
func (rt *RoutingTable) GetAllRoutes() []*Route {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	routes := make([]*Route, 0, len(rt.routes))
	for _, route := range rt.routes {
		routes = append(routes, route)
	}
	return routes
}

// Router handles message routing in the mesh network
type Router struct {
	routingTable *RoutingTable
	peerStore   *PeerStore
}

// NewRouter creates a new router
func NewRouter(peerStore *PeerStore) *Router {
	return &Router{
		routingTable: NewRoutingTable(),
		peerStore:    peerStore,
	}
}

// FindRoute finds a route to a destination
func (r *Router) FindRoute(dest types.Address) (*Route, error) {
	// Check if destination is directly connected
	if peer, ok := r.peerStore.Get(dest); ok && peer.IsConnected {
		return &Route{
			Destination: dest,
			NextHop:     dest,
			Hops:        1,
		}, nil
	}
	
	// Check routing table
	if route, ok := r.routingTable.GetRoute(dest); ok {
		return route, nil
	}
	
	return nil, fmt.Errorf("no route to destination: %s", dest.Hex())
}

// UpdateRoute updates a route in the routing table
func (r *Router) UpdateRoute(dest, nextHop types.Address, hops int) {
	r.routingTable.AddRoute(dest, nextHop, hops)
}

