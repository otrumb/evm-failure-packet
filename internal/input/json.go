package input

import (
	"encoding/json"
	"fmt"
)

func parseRPCError(object map[string]json.RawMessage) (Evidence, error) {
	var rpcError struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(object["error"], &rpcError); err != nil {
		return Evidence{}, &PathError{Path: "/error", Err: fmt.Errorf("%w: %w", ErrInvalidInput, err)}
	}
	var value string
	if err := json.Unmarshal(rpcError.Data, &value); err != nil {
		return Evidence{}, &PathError{Path: "/error/data", Err: fmt.Errorf("%w: expected hex string", ErrInvalidInput)}
	}
	evidence, err := parseRevert(value, "/error/data")
	if err != nil {
		return Evidence{}, err
	}
	evidence.Shape = ShapeRPCError
	return evidence, nil
}

func parseReceipt(object map[string]json.RawMessage) (Evidence, error) {
	status := string(object["status"])
	if status != `"0x0"` && status != `"0x00"` && status != "0" {
		return Evidence{}, &PathError{Path: "/status", Err: fmt.Errorf("%w: receipt is not failed", ErrInvalidInput)}
	}
	raw, ok := object["revertReason"]
	if !ok {
		return Evidence{}, &PathError{Path: "/revertReason", Err: fmt.Errorf("%w: failed receipt has no revert data", ErrInvalidInput)}
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return Evidence{}, &PathError{Path: "/revertReason", Err: fmt.Errorf("%w: expected hex string", ErrInvalidInput)}
	}
	evidence, err := parseRevert(value, "/revertReason")
	if err != nil {
		return Evidence{}, err
	}
	evidence.Shape = ShapeReceipt
	return evidence, nil
}
