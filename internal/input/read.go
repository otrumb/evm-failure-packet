package input

import (
	"fmt"
	"io"
)

func ReadBounded(reader io.Reader, maximum int64, label string) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(content)) > maximum {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", ErrLimit, label, maximum)
	}
	return content, nil
}
