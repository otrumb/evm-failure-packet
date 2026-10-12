package input_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	inputadapter "github.com/otrumb/evm-failure-packet/internal/input"
)

func TestParse_recognizes_supported_shapes(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantShape inputadapter.Shape
		wantPath  string
		wantSHA   string
	}{
		{name: "raw", content: "0xdeadbeef", wantShape: inputadapter.ShapeRaw, wantPath: "$raw", wantSHA: "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953"},
		{name: "JSON-RPC error", content: readFixture(t, "rpc-error.json"), wantShape: inputadapter.ShapeRPCError, wantPath: "/error/data", wantSHA: "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953"},
		{name: "receipt", content: readFixture(t, "receipt.json"), wantShape: inputadapter.ShapeReceipt, wantPath: "/revertReason", wantSHA: "f9f3805b314056831e6947c7ff8914554894d56eed9e4e5a6f37a08651db9c58"},
		{name: "call tracer", content: readFixture(t, "call-trace.json"), wantShape: inputadapter.ShapeTrace, wantPath: "/output", wantSHA: "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence, err := inputadapter.Parse([]byte(test.content))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if evidence.Shape != test.wantShape || evidence.ExtractionPath != test.wantPath {
				t.Fatalf("Parse() = shape %q path %q, want %q %q", evidence.Shape, evidence.ExtractionPath, test.wantShape, test.wantPath)
			}
			if evidence.RevertDataSHA256 != test.wantSHA {
				t.Fatalf("revert data SHA-256 = %q", evidence.RevertDataSHA256)
			}
		})
	}
}

func TestParse_rejects_unsupported_shape(t *testing.T) {
	_, err := inputadapter.Parse([]byte(`{"result":"0xdeadbeef"}`))
	if !errors.Is(err, inputadapter.ErrUnsupportedShape) {
		t.Fatalf("Parse() error = %v, want ErrUnsupportedShape", err)
	}
}

func TestReadBounded_fails_instead_of_truncating(t *testing.T) {
	_, err := inputadapter.ReadBounded(strings.NewReader("12345"), 4, "test")
	if !errors.Is(err, inputadapter.ErrLimit) {
		t.Fatalf("ReadBounded() error = %v, want ErrLimit", err)
	}
}

func TestParse_reports_nested_trace_frame_path(t *testing.T) {
	content := `{"type":"CALL","calls":[{"type":"CALL","error":"reverted","output":"0xdeadbeef"}]}`
	evidence, err := inputadapter.Parse([]byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if evidence.ExtractionPath != "/calls/0/output" {
		t.Fatalf("extraction path = %q, want /calls/0/output", evidence.ExtractionPath)
	}
}

func TestParse_selects_first_failed_trace_frame_in_DFS_preorder(t *testing.T) {
	content := `{"type":"CALL","calls":[{"type":"CALL","calls":[{"type":"CALL","error":"reverted","output":"0x0102"}]},{"type":"CALL","error":"reverted","output":"0x0304"}]}`
	evidence, err := inputadapter.Parse([]byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if evidence.ExtractionPath != "/calls/0/calls/0/output" {
		t.Fatalf("extraction path = %q, want /calls/0/calls/0/output", evidence.ExtractionPath)
	}
}

func TestParse_accepts_trace_at_depth_limit(t *testing.T) {
	evidence, err := inputadapter.Parse([]byte(traceChain(inputadapter.MaxTraceDepth, true)))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if evidence.ExtractionPath != strings.Repeat("/calls/0", inputadapter.MaxTraceDepth-1)+"/output" {
		t.Fatalf("extraction path = %q", evidence.ExtractionPath)
	}
}

func TestParse_rejects_trace_over_depth_limit(t *testing.T) {
	_, err := inputadapter.Parse([]byte(traceChain(inputadapter.MaxTraceDepth+1, true)))
	if !errors.Is(err, inputadapter.ErrLimit) {
		t.Fatalf("Parse() error = %v, want ErrLimit at depth %s", err, strconv.Itoa(inputadapter.MaxTraceDepth+1))
	}
}

func TestParse_enforces_depth_limit_after_root_evidence(t *testing.T) {
	var content strings.Builder
	content.WriteString(`{"type":"CALL","error":"reverted","output":"0xdeadbeef","calls":[`)
	for range inputadapter.MaxTraceDepth {
		content.WriteString(`{"type":"CALL","calls":[`)
	}
	content.WriteString(`{"type":"CALL"}`)
	for range inputadapter.MaxTraceDepth {
		content.WriteString(`]}`)
	}
	content.WriteString(`]}`)
	_, err := inputadapter.Parse([]byte(content.String()))
	if !errors.Is(err, inputadapter.ErrLimit) {
		t.Fatalf("Parse() error = %v, want ErrLimit", err)
	}
}

func TestParse_accepts_trace_at_node_limit(t *testing.T) {
	evidence, err := inputadapter.Parse([]byte(wideTrace(inputadapter.MaxTraceNodes)))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if evidence.ExtractionPath != "/output" {
		t.Fatalf("extraction path = %q, want /output", evidence.ExtractionPath)
	}
}

func TestParse_rejects_first_node_over_limit_after_root_evidence(t *testing.T) {
	_, err := inputadapter.Parse([]byte(wideTrace(inputadapter.MaxTraceNodes + 1)))
	if !errors.Is(err, inputadapter.ErrLimit) {
		t.Fatalf("Parse() error = %v, want ErrLimit", err)
	}
}

func traceChain(depth int, leafEvidence bool) string {
	var content strings.Builder
	for range depth - 1 {
		content.WriteString(`{"type":"CALL","calls":[`)
	}
	content.WriteString(`{"type":"CALL"`)
	if leafEvidence {
		content.WriteString(`,"error":"reverted","output":"0xdeadbeef"`)
	}
	content.WriteByte('}')
	for range depth - 1 {
		content.WriteString(`]}`)
	}
	return content.String()
}

func wideTrace(nodes int) string {
	var content strings.Builder
	content.WriteString(`{"type":"CALL","error":"reverted","output":"0xdeadbeef","calls":[`)
	for index := 1; index < nodes; index++ {
		if index > 1 {
			content.WriteByte(',')
		}
		content.WriteString(`{"type":"CALL"}`)
	}
	content.WriteString(`]}`)
	return content.String()
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
