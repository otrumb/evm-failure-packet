package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/otrumb/evm-failure-packet/internal/decode"
	inputadapter "github.com/otrumb/evm-failure-packet/internal/input"
	"github.com/otrumb/evm-failure-packet/internal/packet"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2
)

type abiPaths []string

type optionValue struct {
	value string
	set   bool
}

func (option *optionValue) String() string {
	return option.value
}

func (option *optionValue) Set(value string) error {
	option.value = value
	option.set = true
	return nil
}

func (paths *abiPaths) String() string {
	return fmt.Sprint([]string(*paths))
}

func (paths *abiPaths) Set(value string) error {
	*paths = append(*paths, value)
	return nil
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("evm-failure-packet", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var inputPath, raw optionValue
	flags.Var(&inputPath, "input", "input path, or - for stdin")
	flags.Var(&raw, "raw", "raw 0x-prefixed revert data")
	var abiFiles abiPaths
	flags.Var(&abiFiles, "abi", "contract ABI JSON path (repeatable)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}
	if flags.NArg() != 0 || inputPath.set == raw.set {
		fmt.Fprintln(stderr, "evm-failure-packet: exactly one of --input and --raw is required")
		return ExitUsage
	}

	evidence, err := loadEvidence(inputPath.value, raw.value, raw.set, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "evm-failure-packet: %v\n", err)
		return ExitFailure
	}
	loaded, err := inputadapter.LoadABIs(abiFiles)
	if err != nil {
		fmt.Fprintf(stderr, "evm-failure-packet: %v\n", err)
		return ExitFailure
	}
	contracts := make([]decode.ABIBundle, len(loaded))
	for index := range loaded {
		contracts[index] = decode.ABIBundle{Source: loaded[index].Path, ABI: loaded[index].ABI}
	}
	decoded, decodeErr := decode.Decode(evidence.RevertData, contracts...)
	result := packet.New(evidence.RevertData, packet.Provenance{
		InputShape:       packet.InputShape(evidence.Shape),
		ArtifactOrigin:   packet.ArtifactOrigin(evidence.Origin),
		ArtifactPath:     evidence.ArtifactPath,
		ExtractionPath:   evidence.ExtractionPath,
		ArtifactSHA256:   evidence.ArtifactSHA256,
		RevertDataSHA256: evidence.RevertDataSHA256,
	}, decoded)
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(stderr, "evm-failure-packet: write output: %v\n", err)
		return ExitFailure
	}
	if decodeErr != nil {
		fmt.Fprintf(stderr, "evm-failure-packet: %v\n", decodeErr)
		return ExitFailure
	}
	return ExitSuccess
}

func loadEvidence(path, raw string, hasRaw bool, stdin io.Reader) (inputadapter.Evidence, error) {
	if hasRaw {
		evidence, err := inputadapter.ParseRaw(raw)
		if err != nil {
			return inputadapter.Evidence{}, err
		}
		return inputadapter.AttachArtifact(evidence, inputadapter.OriginRawFlag, "", []byte(raw)), nil
	}
	if path == "-" {
		content, err := inputadapter.ReadBounded(stdin, inputadapter.MaxInputBytes, "stdin")
		if err != nil {
			return inputadapter.Evidence{}, err
		}
		evidence, err := inputadapter.Parse(content)
		if err != nil {
			return inputadapter.Evidence{}, err
		}
		return inputadapter.AttachArtifact(evidence, inputadapter.OriginStdin, "", content), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return inputadapter.Evidence{}, fmt.Errorf("open input %q: %w", path, err)
	}
	content, readErr := inputadapter.ReadBounded(file, inputadapter.MaxInputBytes, "input "+path)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		if closeErr != nil {
			closeErr = fmt.Errorf("close input %q: %w", path, closeErr)
		}
		return inputadapter.Evidence{}, errors.Join(readErr, closeErr)
	}
	evidence, err := inputadapter.Parse(content)
	if err != nil {
		return inputadapter.Evidence{}, err
	}
	return inputadapter.AttachArtifact(evidence, inputadapter.OriginPath, path, content), nil
}
