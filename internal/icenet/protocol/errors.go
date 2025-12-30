package protocol

import "fmt"

// ProtocolError represents an error in protocol encoding/decoding
type ProtocolError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ProtocolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("protocol error [%s]: %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("protocol error [%s]: %s", e.Type, e.Message)
}

func (e *ProtocolError) Unwrap() error {
	return e.Cause
}

// NewProtocolError creates a new protocol error
func NewProtocolError(msgType, message string, cause error) *ProtocolError {
	return &ProtocolError{
		Type:    msgType,
		Message: message,
		Cause:   cause,
	}
}

// ValidationError represents a message validation error
type ValidationError struct {
	Field   string
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation error [%s]: %s: %v", e.Field, e.Message, e.Cause)
	}
	return fmt.Sprintf("validation error [%s]: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string, cause error) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Cause:   cause,
	}
}

