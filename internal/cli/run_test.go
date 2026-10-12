package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/otrumb/evm-failure-packet/internal/cli"
	"github.com/otrumb/evm-failure-packet/internal/packet"
)

func TestRun_decodes_raw_error_to_JSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "error-string.hex"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := cli.Run([]string{"--raw", strings.TrimSpace(string(raw))}, strings.NewReader(""), &stdout, &stderr)
	if exit != cli.ExitSuccess {
		t.Fatalf("Run() exit = %d, stderr = %s", exit, stderr.String())
	}
	var result packet.Packet
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if result.Failure.Decoded.Signature != "Error(string)" || result.Failure.Decoded.Source != "builtin" {
		t.Fatalf("decoded = %#v", result)
	}
	if result.Provenance.InputShape != "raw" || result.Provenance.ArtifactOrigin != "--raw" || result.Provenance.ExtractionPath != "$raw" {
		t.Fatalf("provenance = %#v", result.Provenance)
	}
	if result.Provenance.ArtifactSHA256 != digest(strings.TrimSpace(string(raw))) || result.Provenance.RevertDataSHA256 != "2c9ca18d84ef2fe1e385552765a4c3da8d4244d84b79051b7463d4793d8f9476" {
		t.Fatalf("provenance digests = %#v", result.Provenance)
	}
}

func TestRun_reads_stdin_and_emits_deterministic_JSON(t *testing.T) {
	args := []string{"--input", "-"}
	input := `{"jsonrpc":"2.0","error":{"data":"0xdeadbeef"}}`
	var first, second bytes.Buffer
	firstExit := cli.Run(args, strings.NewReader(input), &first, &bytes.Buffer{})
	secondExit := cli.Run(args, strings.NewReader(input), &second, &bytes.Buffer{})
	if firstExit != cli.ExitSuccess || secondExit != cli.ExitSuccess {
		t.Fatalf("Run() exits = %d, %d", firstExit, secondExit)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("outputs differ:\n%s\n%s", first.Bytes(), second.Bytes())
	}
	var result packet.Packet
	if err := json.Unmarshal(first.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Provenance.InputShape != "json_rpc_error" || result.Provenance.ArtifactOrigin != "stdin" || result.Provenance.ExtractionPath != "/error/data" {
		t.Fatalf("provenance = %#v", result.Provenance)
	}
	if result.Provenance.ArtifactSHA256 != digest(input) || result.Provenance.RevertDataSHA256 != "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953" {
		t.Fatalf("provenance digests = %#v", result.Provenance)
	}
}

func TestRun_records_path_artifact_provenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	content := []byte(`{"status":"0x0","revertReason":"0xdeadbeef"}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if exit := cli.Run([]string{"--input", path}, strings.NewReader(""), &stdout, &bytes.Buffer{}); exit != cli.ExitSuccess {
		t.Fatalf("Run() exit = %d", exit)
	}
	var result packet.Packet
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Provenance.InputShape != "transaction_receipt" || result.Provenance.ArtifactOrigin != "path" || result.Provenance.ArtifactPath != path || result.Provenance.ExtractionPath != "/revertReason" {
		t.Fatalf("provenance = %#v", result.Provenance)
	}
	if result.Provenance.ArtifactSHA256 != digest(string(content)) || result.Provenance.RevertDataSHA256 != "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953" {
		t.Fatalf("provenance digests = %#v", result.Provenance)
	}
}

func TestRun_returns_usage_for_mutually_exclusive_inputs(t *testing.T) {
	tests := [][]string{{}, {"--raw", "0xdeadbeef", "--input", "-"}}
	for _, args := range tests {
		exit := cli.Run(args, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
		if exit != cli.ExitUsage {
			t.Fatalf("Run(%v) exit = %d, want %d", args, exit, cli.ExitUsage)
		}
	}
}

func TestRun_returns_failure_for_empty_raw_value(t *testing.T) {
	exit := cli.Run([]string{"--raw", ""}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if exit != cli.ExitFailure {
		t.Fatalf("Run() exit = %d, want %d", exit, cli.ExitFailure)
	}
}

func TestRun_returns_failure_for_unsupported_shape(t *testing.T) {
	var stderr bytes.Buffer
	exit := cli.Run([]string{"--input", "-"}, strings.NewReader(`{"result":"0xdeadbeef"}`), &bytes.Buffer{}, &stderr)
	if exit != cli.ExitFailure || !strings.Contains(stderr.String(), "unsupported input shape") {
		t.Fatalf("Run() exit = %d stderr = %q", exit, stderr.String())
	}
}

func TestRun_uses_ABI_file_for_custom_error(t *testing.T) {
	directory := t.TempDir()
	abiPath := filepath.Join(directory, "contract.json")
	abiJSON := `[{"type":"error","name":"Denied","inputs":[{"name":"account","type":"address"}]}]`
	if err := os.WriteFile(abiPath, []byte(abiJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	data := append(crypto.Keccak256([]byte("Denied(address)"))[:4], make([]byte, 32)...)
	var stdout, stderr bytes.Buffer
	exit := cli.Run([]string{"--raw", "0x" + fmt.Sprintf("%x", data), "--abi", abiPath}, strings.NewReader(""), &stdout, &stderr)
	if exit != cli.ExitSuccess || !bytes.Contains(stdout.Bytes(), []byte(`"signature":"Denied(address)"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"source":`+fmt.Sprintf("%q", abiPath))) {
		t.Fatalf("Run() exit = %d stdout = %s stderr = %s", exit, stdout.String(), stderr.String())
	}
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
