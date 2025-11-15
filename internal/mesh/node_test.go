package mesh

import (
	"bytes"
	"math/big"
	"net"
	"testing"
)

func TestNewNetworkNode(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		port     string
		expected *NetworkNode
	}{
		{
			name: "valid IPv4 and port",
			ip:   "192.168.1.1",
			port: "8080",
			expected: &NetworkNode{
				IP:   net.ParseIP("192.168.1.1"),
				Port: 8080,
			},
		},
		{
			name: "localhost",
			ip:   "127.0.0.1",
			port: "3000",
			expected: &NetworkNode{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 3000,
			},
		},
		{
			name: "invalid port string",
			ip:   "192.168.1.1",
			port: "invalid",
			expected: &NetworkNode{
				IP:   net.ParseIP("192.168.1.1"),
				Port: 0, // Atoi returns 0 on error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := NewNetworkNode(tt.ip, tt.port)
			if !node.IP.Equal(tt.expected.IP) {
				t.Errorf("Expected IP %v, got %v", tt.expected.IP, node.IP)
			}
			if node.Port != tt.expected.Port {
				t.Errorf("Expected Port %d, got %d", tt.expected.Port, node.Port)
			}
		})
	}
}

func TestGetDistance(t *testing.T) {
	tests := []struct {
		name     string
		id1      []byte
		id2      []byte
		expected *big.Int
	}{
		{
			name:     "same IDs",
			id1:      []byte{1, 2, 3, 4},
			id2:      []byte{1, 2, 3, 4},
			expected: big.NewInt(0),
		},
		{
			name:     "different IDs",
			id1:      []byte{0, 0, 0, 1},
			id2:      []byte{0, 0, 0, 2},
			expected: big.NewInt(3), // 1 XOR 2 = 3
		},
		{
			name:     "completely different",
			id1:      []byte{255, 255, 255, 255},
			id2:      []byte{0, 0, 0, 0},
			expected: big.NewInt(0xFFFFFFFF),
		},
		{
			name:     "empty IDs",
			id1:      []byte{},
			id2:      []byte{},
			expected: big.NewInt(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDistance(tt.id1, tt.id2)
			if result.Cmp(tt.expected) != 0 {
				t.Errorf("Expected distance %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestShortList_Len(t *testing.T) {
	sl := &shortList{
		Nodes: []*NetworkNode{
			{ID: []byte{1}},
			{ID: []byte{2}},
			{ID: []byte{3}},
		},
		Comparator: []byte{0},
	}

	if sl.Len() != 3 {
		t.Errorf("Expected length 3, got %d", sl.Len())
	}

	sl.Nodes = []*NetworkNode{}
	if sl.Len() != 0 {
		t.Errorf("Expected length 0, got %d", sl.Len())
	}
}

func TestShortList_Swap(t *testing.T) {
	sl := &shortList{
		Nodes: []*NetworkNode{
			{ID: []byte{1}},
			{ID: []byte{2}},
			{ID: []byte{3}},
		},
		Comparator: []byte{0},
	}

	originalFirst := sl.Nodes[0]
	originalSecond := sl.Nodes[1]

	sl.Swap(0, 1)

	if !bytes.Equal(sl.Nodes[0].ID, originalSecond.ID) {
		t.Errorf("Swap failed: first element should be %v, got %v", originalSecond.ID, sl.Nodes[0].ID)
	}
	if !bytes.Equal(sl.Nodes[1].ID, originalFirst.ID) {
		t.Errorf("Swap failed: second element should be %v, got %v", originalFirst.ID, sl.Nodes[1].ID)
	}
}

func TestShortList_Less(t *testing.T) {
	comparator := []byte{5, 5, 5, 5}

	// Node closer to comparator
	node1 := &NetworkNode{ID: []byte{5, 5, 5, 4}} // distance = 1
	// Node farther from comparator
	node2 := &NetworkNode{ID: []byte{0, 0, 0, 0}} // distance = 5^4 = 625

	sl := &shortList{
		Nodes:      []*NetworkNode{node1, node2},
		Comparator: comparator,
	}

	if !sl.Less(0, 1) {
		t.Error("Expected node1 to be less than node2 (closer to comparator)")
	}

	if sl.Less(1, 0) {
		t.Error("Expected node2 to NOT be less than node1")
	}
}

func TestShortList_RemoveNode(t *testing.T) {
	node1 := &NetworkNode{ID: []byte{1}}
	node2 := &NetworkNode{ID: []byte{2}}
	node3 := &NetworkNode{ID: []byte{3}}

	sl := &shortList{
		Nodes:      []*NetworkNode{node1, node2, node3},
		Comparator: []byte{0},
	}

	// Remove middle node
	sl.RemoveNode(node2)

	if sl.Len() != 2 {
		t.Errorf("Expected length 2 after removal, got %d", sl.Len())
	}

	if bytes.Equal(sl.Nodes[0].ID, node2.ID) || bytes.Equal(sl.Nodes[1].ID, node2.ID) {
		t.Error("Node2 should have been removed")
	}

	// Remove first node
	sl.RemoveNode(node1)
	if sl.Len() != 1 {
		t.Errorf("Expected length 1 after second removal, got %d", sl.Len())
	}

	// Remove last node
	sl.RemoveNode(node3)
	if sl.Len() != 0 {
		t.Errorf("Expected length 0 after third removal, got %d", sl.Len())
	}

	// Try to remove non-existent node
	sl.RemoveNode(&NetworkNode{ID: []byte{99}})
	if sl.Len() != 0 {
		t.Errorf("Expected length to remain 0, got %d", sl.Len())
	}
}

func TestShortList_AppendUniqueNetworkNodes(t *testing.T) {
	existingNode := &NetworkNode{ID: []byte{1}}
	newNode1 := &NetworkNode{ID: []byte{2}}
	newNode2 := &NetworkNode{ID: []byte{3}}
	duplicateNode := &NetworkNode{ID: []byte{1}} // Same ID as existingNode

	sl := &shortList{
		Nodes:      []*NetworkNode{existingNode},
		Comparator: []byte{0},
	}

	// Append unique nodes
	sl.AppendUniqueNetworkNodes([]*NetworkNode{newNode1, newNode2})

	if sl.Len() != 3 {
		t.Errorf("Expected length 3, got %d", sl.Len())
	}

	// Try to append duplicate
	sl.AppendUniqueNetworkNodes([]*NetworkNode{duplicateNode})

	if sl.Len() != 3 {
		t.Errorf("Expected length to remain 3 (duplicate not added), got %d", sl.Len())
	}

	// Append empty slice
	originalLen := sl.Len()
	sl.AppendUniqueNetworkNodes([]*NetworkNode{})
	if sl.Len() != originalLen {
		t.Errorf("Expected length to remain %d, got %d", originalLen, sl.Len())
	}
}

func TestShortList_AppendUnique(t *testing.T) {
	existingNode := &node{NetworkNode: &NetworkNode{ID: []byte{1}}}
	newNode1 := &node{NetworkNode: &NetworkNode{ID: []byte{2}}}
	newNode2 := &node{NetworkNode: &NetworkNode{ID: []byte{3}}}
	duplicateNode := &node{NetworkNode: &NetworkNode{ID: []byte{1}}}

	sl := &shortList{
		Nodes:      []*NetworkNode{existingNode.NetworkNode},
		Comparator: []byte{0},
	}

	// Append unique nodes
	sl.AppendUnique([]*node{newNode1, newNode2})

	if sl.Len() != 3 {
		t.Errorf("Expected length 3, got %d", sl.Len())
	}

	// Try to append duplicate
	sl.AppendUnique([]*node{duplicateNode})

	if sl.Len() != 3 {
		t.Errorf("Expected length to remain 3 (duplicate not added), got %d", sl.Len())
	}
}

func TestNewNode(t *testing.T) {
	networkNode := &NetworkNode{
		ID:   []byte{1, 2, 3, 4},
		IP:   net.ParseIP("192.168.1.1"),
		Port: 8080,
	}

	n := newNode(networkNode)

	if n.NetworkNode == nil {
		t.Error("Expected NetworkNode to be set")
	}

	if !bytes.Equal(n.ID, networkNode.ID) {
		t.Errorf("Expected ID %v, got %v", networkNode.ID, n.ID)
	}

	if !n.IP.Equal(networkNode.IP) {
		t.Errorf("Expected IP %v, got %v", networkNode.IP, n.IP)
	}

	if n.Port != networkNode.Port {
		t.Errorf("Expected Port %d, got %d", networkNode.Port, n.Port)
	}
}
