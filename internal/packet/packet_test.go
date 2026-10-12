package packet_test

import (
	"encoding/json"
	"testing"

	"github.com/otrumb/evm-failure-packet/internal/packet"
)

func TestNewBuildsVersionedPacketWithRawDataProvenance(t *testing.T) {
	// Given
	raw := []byte{0xde, 0xad, 0xbe, 0xef}
	decoded := packet.Decoded{Status: packet.StatusUnknown, Selector: "0xdeadbeef"}

	// When
	provenance := packet.Provenance{
		InputShape:       "raw",
		ArtifactOrigin:   "--raw",
		ExtractionPath:   "$raw",
		ArtifactSHA256:   "4142710b9b4caaeb000b8e5de271bbebac7f509aab2f5e61d1ed1958bfe6d583",
		RevertDataSHA256: "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953",
	}
	result := packet.New(raw, provenance, decoded)

	// Then
	if result.Schema != "evm-failure-packet/v1" {
		t.Fatalf("schema = %q", result.Schema)
	}
	if result.Provenance != provenance {
		t.Fatalf("provenance = %#v", result.Provenance)
	}
	if result.Failure.Raw != "0xdeadbeef" || result.Failure.Decoded.Status != decoded.Status || result.Failure.Decoded.Selector != decoded.Selector {
		t.Fatalf("failure = %#v", result.Failure)
	}
}

func TestValueMarshalsWithoutUntypedJSONValues(t *testing.T) {
	// Given
	value := packet.Value{
		Kind: packet.ValueTuple,
		Type: "(uint256,bytes)",
		Fields: []packet.Field{
			{Name: "amount", Value: packet.Value{Kind: packet.ValueScalar, Type: "uint256", Text: "9007199254740993"}},
			{Name: "data", Value: packet.Value{Kind: packet.ValueScalar, Type: "bytes", Text: "0x00ff"}},
		},
	}

	// When
	encoded, err := json.Marshal(value)

	// Then
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	want := `{"kind":"tuple","type":"(uint256,bytes)","fields":[{"name":"amount","value":{"kind":"scalar","type":"uint256","text":"9007199254740993"}},{"name":"data","value":{"kind":"scalar","type":"bytes","text":"0x00ff"}}]}`
	if string(encoded) != want {
		t.Fatalf("JSON = %s", encoded)
	}
}
