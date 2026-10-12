package input

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func parseRevert(value, source string) (Evidence, error) {
	if !strings.HasPrefix(value, "0x") {
		return Evidence{}, &PathError{Path: source, Err: fmt.Errorf("%w: revert data must start with 0x", ErrInvalidInput)}
	}
	encoded := value[2:]
	if len(encoded)%2 != 0 {
		return Evidence{}, &PathError{Path: source, Err: fmt.Errorf("%w: revert data has odd hex length", ErrInvalidInput)}
	}
	if len(encoded)/2 > MaxRevertBytes {
		return Evidence{}, &PathError{Path: source, Err: fmt.Errorf("%w: revert data exceeds %d bytes", ErrLimit, MaxRevertBytes)}
	}
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return Evidence{}, &PathError{Path: source, Err: fmt.Errorf("%w: malformed hex: %w", ErrInvalidInput, err)}
	}
	return Evidence{
		Shape:            ShapeRaw,
		ExtractionPath:   source,
		RevertDataSHA256: digest(decoded),
		RevertData:       decoded,
	}, nil
}
