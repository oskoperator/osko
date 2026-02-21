package errors

import (
	"errors"
	"time"
)

var (
	ErrTransient          = errors.New("transient error")
	ErrPermanent          = errors.New("permanent error")
	ErrDependencyNotReady = errors.New("dependency not ready")
	ErrInvalidTarget      = errors.New("invalid SLO target")
)

type ReconcileError struct {
	Err          error
	Type         error
	RequeueAfter time.Duration
}

func (e *ReconcileError) Error() string {
	if e.Err == nil {
		return "unknown error"
	}
	return e.Err.Error()
}
func (e *ReconcileError) Unwrap() error { return e.Err }

func Transient(err error, requeueAfter time.Duration) *ReconcileError {
	return &ReconcileError{Err: err, Type: ErrTransient, RequeueAfter: requeueAfter}
}

func Permanent(err error) *ReconcileError {
	return &ReconcileError{Err: err, Type: ErrPermanent}
}

func DependencyNotReady(err error) *ReconcileError {
	return &ReconcileError{Err: err, Type: ErrDependencyNotReady, RequeueAfter: 10 * time.Second}
}
