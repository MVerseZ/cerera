package connection

import "fmt"

// ConnectionError represents an error related to a connection
type ConnectionError struct {
	ConnectionID string
	Operation    string
	Message      string
	Cause        error
}

func (e *ConnectionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("connection error [%s] %s: %s: %v", e.ConnectionID, e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("connection error [%s] %s: %s", e.ConnectionID, e.Operation, e.Message)
}

func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// NewConnectionError creates a new connection error
func NewConnectionError(connID, operation, message string, cause error) *ConnectionError {
	return &ConnectionError{
		ConnectionID: connID,
		Operation:    operation,
		Message:      message,
		Cause:        cause,
	}
}

