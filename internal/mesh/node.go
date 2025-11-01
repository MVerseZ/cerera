package mesh

import (
	"net"
)

var (
	NoNetInterfaces  = &nodeError{"no network interfaces found"}
	InvalidIPAddress = &nodeError{"invalid IP address"}
)

type nodeError struct{ msg string }

func (err nodeError) Error() string { return err.msg }

type Node struct {
	NetworkAddress string
	LastSeen       int64
	IsConnected    bool
}

func getLocalIPv4Address() (string, error) {
	interfaces, err := net.InterfaceAddrs()
	if err != nil {
		return "", NoNetInterfaces
	}
	for _, iface := range interfaces {
		ip, _, err := net.ParseCIDR(iface.String())
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return "", InvalidIPAddress
}

func NewNode() *Node {
	var localNetworkAddress string
	localNetworkAddress, err := getLocalIPv4Address()
	if err != nil {
		localNetworkAddress = "127.0.0.1"
	}
	return &Node{
		NetworkAddress: localNetworkAddress,
		LastSeen:       0,
		IsConnected:    false,
	}
}
