package mesh

import (
	"bytes"
	"math/big"
	"net"
	"strconv"
)

var (
	NoNetInterfaces  = &nodeError{"no network interfaces found"}
	InvalidIPAddress = &nodeError{"invalid IP address"}
)

type nodeError struct{ msg string }

func (err nodeError) Error() string { return err.msg }

type node struct {
	*NetworkNode
}

// nodeList is used in order to sort a list of arbitrary nodes against a
// comparator. These nodes are sorted by xor distance
type shortList struct {
	// Nodes are a list of nodes to be compared
	Nodes []*NetworkNode

	// Comparator is the ID to compare to
	Comparator []byte
}

type NetworkNode struct {
	// ID is a 20 byte unique identifier
	ID []byte

	// IP is the IPv4 address of the node
	IP net.IP

	// Port is the port of the node
	Port int
}

// NewNetworkNode creates a new NetworkNode for bootstrapping
func NewNetworkNode(ip string, port string) *NetworkNode {
	p, _ := strconv.Atoi(port)
	return &NetworkNode{
		IP:   net.ParseIP(ip),
		Port: p,
	}
}

func newNode(networkNode *NetworkNode) *node {
	n := &node{}
	n.NetworkNode = networkNode
	return n
}

func (n *shortList) Len() int {
	return len(n.Nodes)
}

func (n *shortList) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
}

func (n *shortList) Less(i, j int) bool {
	iDist := getDistance(n.Nodes[i].ID, n.Comparator)
	jDist := getDistance(n.Nodes[j].ID, n.Comparator)

	if iDist.Cmp(jDist) == -1 {
		return true
	}

	return false
}

func (n *shortList) RemoveNode(node *NetworkNode) {
	for i := 0; i < n.Len(); i++ {
		if bytes.Compare(n.Nodes[i].ID, node.ID) == 0 {
			n.Nodes = append(n.Nodes[:i], n.Nodes[i+1:]...)
			return
		}
	}
}

func (n *shortList) AppendUniqueNetworkNodes(nodes []*NetworkNode) {
	for _, vv := range nodes {
		exists := false
		for _, v := range n.Nodes {
			if bytes.Compare(v.ID, vv.ID) == 0 {
				exists = true
				break
			}
		}
		if !exists {
			n.Nodes = append(n.Nodes, vv)
		}
	}
}

func (n *shortList) AppendUnique(nodes []*node) {
	for _, vv := range nodes {
		exists := false
		for _, v := range n.Nodes {
			if bytes.Compare(v.ID, vv.ID) == 0 {
				exists = true
				break
			}
		}
		if !exists {
			n.Nodes = append(n.Nodes, vv.NetworkNode)
		}
	}
}

func getDistance(id1 []byte, id2 []byte) *big.Int {
	buf1 := new(big.Int).SetBytes(id1)
	buf2 := new(big.Int).SetBytes(id2)
	result := new(big.Int).Xor(buf1, buf2)
	return result
}
