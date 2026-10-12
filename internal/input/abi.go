package input

import (
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

type NamedABI struct {
	Path string
	ABI  abi.ABI
}

func LoadABIs(paths []string) ([]NamedABI, error) {
	if len(paths) > MaxABIs {
		return nil, fmt.Errorf("%w: ABI count exceeds %d", ErrLimit, MaxABIs)
	}
	loaded := make([]NamedABI, 0, len(paths))
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open ABI %q: %w", path, err)
		}
		content, readErr := ReadBounded(file, MaxABIBytes, "ABI "+path)
		closeErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close ABI %q: %w", path, closeErr)
		}
		parsed, err := abi.JSON(strings.NewReader(string(content)))
		if err != nil {
			return nil, fmt.Errorf("parse ABI %q: %w", path, err)
		}
		loaded = append(loaded, NamedABI{Path: path, ABI: parsed})
	}
	return loaded, nil
}
