package input

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func Parse(content []byte) (Evidence, error) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return Evidence{}, fmt.Errorf("%w: input is empty", ErrInvalidInput)
	}
	if bytes.HasPrefix(trimmed, []byte("0x")) {
		return parseRevert(string(trimmed), "$raw")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return Evidence{}, fmt.Errorf("%w: malformed JSON: %w", ErrInvalidInput, err)
	}
	switch {
	case object["error"] != nil && object["jsonrpc"] != nil:
		return parseRPCError(object)
	case object["status"] != nil:
		return parseReceipt(object)
	case object["type"] != nil || object["calls"] != nil:
		return parseTrace(trimmed)
	default:
		return Evidence{}, fmt.Errorf("%w: expected raw hex, JSON-RPC error, transaction receipt, or Geth callTracer", ErrUnsupportedShape)
	}
}

func ParseRaw(value string) (Evidence, error) {
	evidence, err := parseRevert(strings.TrimSpace(value), "$raw")
	if err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

func AttachArtifact(evidence Evidence, origin Origin, path string, artifact []byte) Evidence {
	evidence.Origin = origin
	evidence.ArtifactPath = path
	evidence.ArtifactSHA256 = digest(artifact)
	return evidence
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
