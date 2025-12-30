package protocol

import (
	"fmt"
	"net"
	"strings"

	"github.com/cerera/internal/cerera/types"
)

const (
	// MaxMessageSize is the maximum size of a protocol message in bytes
	MaxMessageSize = 64 * 1024 // 64 KB
)

// Validator provides message validation functionality
type Validator struct{}

// NewValidator creates a new message validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateMessageSize checks if a message size is within acceptable limits
func (v *Validator) ValidateMessageSize(size int) error {
	if size <= 0 {
		return fmt.Errorf("message size must be positive, got %d", size)
	}
	if size > MaxMessageSize {
		return fmt.Errorf("message size %d exceeds maximum allowed size %d", size, MaxMessageSize)
	}
	return nil
}

// ValidateAddress validates a Cerera address format
func (v *Validator) ValidateAddress(addr types.Address) error {
	if addr == (types.Address{}) {
		return fmt.Errorf("address cannot be zero")
	}
	return nil
}

// ValidateNetworkAddress validates a network address in format "ip:port"
func (v *Validator) ValidateNetworkAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("network address cannot be empty")
	}

	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("network address must be in format 'ip:port', got: %s", addr)
	}

	ip := parts[0]
	port := parts[1]

	// Validate IP
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	// Validate port
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Try to parse port as number
	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		return fmt.Errorf("invalid port format: %s", port)
	}

	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %d", portNum)
	}

	return nil
}

// ValidateHexAddress validates and parses a hex-encoded address
func (v *Validator) ValidateHexAddress(addrStr string) (*types.Address, error) {
	addrStr = strings.TrimSpace(addrStr)
	if addrStr == "" {
		return nil, fmt.Errorf("address string cannot be empty")
	}

	// Remove 0x prefix if present
	addrStr = strings.TrimPrefix(addrStr, "0x")

	addr := types.HexToAddress(addrStr)
	if addr == (types.Address{}) {
		return nil, fmt.Errorf("invalid address format: %s", addrStr)
	}

	if err := v.ValidateAddress(addr); err != nil {
		return nil, err
	}

	return &addr, nil
}

// SanitizeMessage removes potentially dangerous characters from a message
func (v *Validator) SanitizeMessage(msg string) string {
	// Remove null bytes and control characters except newline and carriage return
	var result strings.Builder
	for _, r := range msg {
		if r == '\n' || r == '\r' {
			result.WriteRune(r)
		} else if r >= 32 && r != 127 {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ValidateMessage validates a message structure
func (v *Validator) ValidateMessage(msg Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	if !msg.Type().IsValid() {
		return fmt.Errorf("invalid message type: %s", msg.Type())
	}

	// Type-specific validation
	switch m := msg.(type) {
	case *ReadyRequestMessage:
		if err := v.ValidateAddress(m.Address); err != nil {
			return fmt.Errorf("invalid address in ReadyRequest: %w", err)
		}
		if err := v.ValidateNetworkAddress(m.NetworkAddr); err != nil {
			return fmt.Errorf("invalid network address in ReadyRequest: %w", err)
		}
	case *REQMessage:
		if err := v.ValidateAddress(m.Address); err != nil {
			return fmt.Errorf("invalid address in REQ: %w", err)
		}
		if err := v.ValidateNetworkAddress(m.NetworkAddr); err != nil {
			return fmt.Errorf("invalid network address in REQ: %w", err)
		}
		for i, node := range m.Nodes {
			if err := v.ValidateAddress(node.Address); err != nil {
				return fmt.Errorf("invalid node address at index %d: %w", i, err)
			}
			if node.NetworkAddr != "" {
				if err := v.ValidateNetworkAddress(node.NetworkAddr); err != nil {
					return fmt.Errorf("invalid node network address at index %d: %w", i, err)
				}
			}
		}
	case *WhoIsMessage:
		if err := v.ValidateAddress(m.NodeAddress); err != nil {
			return fmt.Errorf("invalid node address in WhoIs: %w", err)
		}
	case *WhoIsResponseMessage:
		if err := v.ValidateAddress(m.NodeAddress); err != nil {
			return fmt.Errorf("invalid node address in WhoIsResponse: %w", err)
		}
		if err := v.ValidateNetworkAddress(m.NetworkAddr); err != nil {
			return fmt.Errorf("invalid network address in WhoIsResponse: %w", err)
		}
	case *BlockMessage:
		if m.Block == nil {
			return fmt.Errorf("block cannot be nil in BlockMessage")
		}
		if m.Block.Head == nil {
			return fmt.Errorf("block header cannot be nil in BlockMessage")
		}
	}

	return nil
}
