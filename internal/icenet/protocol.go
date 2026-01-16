package icenet

import (
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"github.com/cerera/internal/cerera/types"
)

// Protocol constants
const (
	// MaxMessageSize is the maximum size of a protocol message in bytes
	MaxMessageSize = 64 * 1024 // 64 KB

	// MaxBufferSize is the maximum buffer size for reading messages
	MaxBufferSize = 64 * 1024 // 64 KB

	// MinBufferSize is the minimum buffer size for reading messages
	MinBufferSize = 1024 // 1 KB

	// DefaultReadBufferSize is the default buffer size for reading
	DefaultReadBufferSize = 4096 // 4 KB

	// DefaultConnectionBufferSize is the default buffer size for connections
	DefaultConnectionBufferSize = 1024 // 1 KB
)

// Protocol message types
const (
	MsgTypeReadyRequest      = "READY_REQUEST"
	MsgTypeREQ               = "REQ"
	MsgTypeNodeOk            = "NODE_OK"
	MsgTypeWhoIs             = "WHO_IS"
	MsgTypeWhoIsResponse     = "WHO_IS_RESPONSE"
	MsgTypeConsensusStatus   = "CONSENSUS_STATUS"
	MsgTypeNodes             = "NODES"
	MsgTypeNodesCount        = "NODES_COUNT"
	MsgTypeBlock             = "BLOCK"
	MsgTypePing              = "PING"
	MsgTypeKeepAlive         = "KEEPALIVE"
	MsgTypeStartConsensus    = "START_CONSENSUS"
	MsgTypeConsensusStart    = "CONSENSUS_START"
	MsgTypeBeginConsensus    = "BEGIN_CONSENSUS"
	MsgTypeConsensusBegin    = "CONSENSUS_BEGIN"
	MsgTypeBroadcastNonce    = "BROADCAST_NONCE"
)

// Protocol timeouts
const (
	DefaultConnectionTimeout  = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultExtendedReadTimeout = 60 * time.Second // Extended timeout for long-running operations
	DefaultWriteTimeout      = 5 * time.Second
	DefaultPingTimeout        = 1 * time.Second
	DefaultKeepAliveInterval = 30 * time.Second
	DefaultPingInterval      = 10 * time.Second
	DefaultNodeOkInterval    = 3 * time.Second
	DefaultBlockCheckInterval = 5 * time.Second
)

// ValidateMessageSize checks if a message size is within acceptable limits
func ValidateMessageSize(size int) error {
	if size <= 0 {
		return fmt.Errorf("message size must be positive, got %d", size)
	}
	if size > MaxMessageSize {
		return fmt.Errorf("message size %d exceeds maximum allowed size %d", size, MaxMessageSize)
	}
	return nil
}

// ValidateAddress валидирует формат Cerera адреса.
// Проверяет, что адрес не является нулевым.
//
// Параметры:
//   - addr: адрес для валидации
//
// Возвращает ошибку, если адрес невалиден.
func ValidateAddress(addr types.Address) error {
	if addr == (types.Address{}) {
		return fmt.Errorf("address cannot be zero")
	}
	// Additional validation can be added here if needed
	return nil
}

// ValidateNetworkAddress валидирует сетевой адрес в формате "ip:port".
// Проверяет корректность IP адреса и порта (должен быть в диапазоне 1-65535).
//
// Параметры:
//   - addr: сетевой адрес в формате "ip:port"
//
// Возвращает ошибку, если адрес невалиден.
func ValidateNetworkAddress(addr string) error {
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

// ValidateHexAddress валидирует и парсит hex-кодированный адрес.
// Поддерживает адреса с префиксом "0x" и без него.
//
// Параметры:
//   - addrStr: строка с hex-кодированным адресом
//
// Возвращает указатель на адрес и ошибку, если парсинг не удался.
func ValidateHexAddress(addrStr string) (*types.Address, error) {
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

	if err := ValidateAddress(addr); err != nil {
		return nil, err
	}

	return &addr, nil
}

// SanitizeMessage удаляет потенциально опасные символы из сообщения.
// Удаляет null-байты и управляющие символы, кроме символов новой строки и возврата каретки.
// Используется для защиты от инъекций в протокол.
//
// Параметры:
//   - msg: исходное сообщение
//
// Возвращает очищенное сообщение.
func SanitizeMessage(msg string) string {
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

// CalculateBackoff вычисляет экспоненциальную задержку для повторных попыток.
// Формула: baseDelay * (2^attempt), ограничена значением maxDelay.
// Используется для переподключения к узлам сети.
//
// Параметры:
//   - attempt: номер попытки (начинается с 0)
//   - baseDelay: базовая задержка
//   - maxDelay: максимальная задержка
//
// Возвращает вычисленную задержку.
func CalculateBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// MessageType represents a protocol message type
type MessageType string

// IsValid checks if the message type is valid
func (mt MessageType) IsValid() bool {
	validTypes := []string{
		MsgTypeReadyRequest,
		MsgTypeREQ,
		MsgTypeNodeOk,
		MsgTypeWhoIs,
		MsgTypeWhoIsResponse,
		MsgTypeConsensusStatus,
		MsgTypeNodes,
		MsgTypeNodesCount,
		MsgTypeBlock,
		MsgTypePing,
		MsgTypeKeepAlive,
		MsgTypeStartConsensus,
		MsgTypeConsensusStart,
		MsgTypeBeginConsensus,
		MsgTypeConsensusBegin,
		MsgTypeBroadcastNonce,
	}
	for _, vt := range validTypes {
		if string(mt) == vt {
			return true
		}
	}
	return false
}

// ParseMessageType extracts message type from a message string
func ParseMessageType(msg string) MessageType {
	msg = strings.TrimSpace(msg)
	// Find the first pipe or newline
	idx := strings.IndexAny(msg, "|\n")
	if idx > 0 {
		return MessageType(msg[:idx])
	}
	return MessageType(msg)
}

