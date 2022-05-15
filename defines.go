package simpwebserv

import "errors"

const (
	bufferMaxSize      = 1024
	fileSendBufferSize = 4096
)

var (
	ErrBufferTooBig            = errors.New("buffer too big")
	ErrRequirementNotSatisfied = errors.New("requirement not satisfied")
)
