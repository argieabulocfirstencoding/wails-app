package types

import "fmt"

// RemoteAPIError preserves the HTTP status from a remote service without
// exposing request credentials or response bodies to the UI.
type RemoteAPIError struct {
	Service    string
	StatusCode int
	Message    string
}

func (e *RemoteAPIError) Error() string {
	if e == nil {
		return "remote API error"
	}
	return fmt.Sprintf("%s request failed (%d): %s", e.Service, e.StatusCode, e.Message)
}

func NewRemoteAPIError(service string, statusCode int, message string) error {
	return &RemoteAPIError{
		Service:    service,
		StatusCode: statusCode,
		Message:    message,
	}
}

type OperationError struct {
	Op       string
	RecordID uint
	UserID   string
	Err      error
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("op=%s record_id=%d user_id=%s: %v", e.Op, e.RecordID, e.UserID, e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

func WrapOperation(op string, recordID uint, userID string, err error) error {
	if err == nil {
		return nil
	}
	return &OperationError{
		Op:       op,
		RecordID: recordID,
		UserID:   userID,
		Err:      err,
	}
}

type StableError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *StableError) Error() string {
	return e.Message
}

func NewStableError(code, message string) error {
	return &StableError{
		Code:    code,
		Message: message,
	}
}
