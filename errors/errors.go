package errors

import (
	"errors"
)

type ErrorResponse struct {
	Message string `json:"error"`
}

var (
	ErrListVolumesFailed   = errors.New("failed to list volumes")
	ErrGetVolumeFailed     = errors.New("failed to get volume")
	ErrCreateVolumeFailed  = errors.New("failed to create volume")
	ErrDestroyVolumeFailed = errors.New("failed to destroy volume")
	ErrSetPropertyFailed   = errors.New("failed to set property on volume")
	ErrGetPrivilegedFailed = errors.New("failed to get privileged status of volume")
	ErrSetPrivilegedFailed = errors.New("failed to change privileged status of volume")
	ErrStreamInFailed      = errors.New("failed to stream in to volume")
	ErrStreamOutFailed     = errors.New("failed to stream out from volume")
	ErrStreamOutNotFound   = errors.New("no such file or directory")
)
