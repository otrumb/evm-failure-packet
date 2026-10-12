package input

import (
	"errors"
	"fmt"
)

var (
	ErrLimit            = errors.New("resource limit exceeded")
	ErrUnsupportedShape = errors.New("unsupported input shape")
	ErrInvalidInput     = errors.New("invalid input")
)

type PathError struct {
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

func (e *PathError) Unwrap() error {
	return e.Err
}
