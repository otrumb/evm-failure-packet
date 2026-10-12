package decode

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/otrumb/evm-failure-packet/internal/packet"
)

const (
	errorSelector = "08c379a0"
	panicSelector = "4e487b71"
)

type definition struct {
	name      string
	signature string
	source    string
	inputs    ethabi.Arguments
}

type ABIBundle struct {
	Source string
	ABI    ethabi.ABI
}

func Decode(data []byte, contracts ...ABIBundle) (packet.Decoded, error) {
	if len(data) < 4 {
		result := packet.Decoded{Status: packet.StatusMalformed}
		return result, &MalformedError{}
	}

	selector := hex.EncodeToString(data[:4])
	result := packet.Decoded{Status: packet.StatusUnknown, Selector: "0x" + selector}
	definitions, err := matchingDefinitions(data[:4], contracts)
	if err != nil {
		result.Status = packet.StatusMalformed
		return result, &MalformedError{Selector: result.Selector, Cause: err}
	}
	if len(definitions) == 0 {
		return result, nil
	}

	candidates := make([]packet.Candidate, 0, len(definitions))
	var decodeErrors []error
	for _, candidate := range definitions {
		decoded := packet.Candidate{Name: candidate.name, Signature: candidate.signature, Source: candidate.source}
		values, unpackErr := candidate.inputs.Unpack(data[4:])
		if unpackErr != nil {
			decodeErrors = append(decodeErrors, fmt.Errorf("%s: %w", candidate.signature, unpackErr))
			candidates = append(candidates, decoded)
			continue
		}
		arguments, normalizeErr := normalizeArguments(candidate.inputs, values)
		if normalizeErr != nil {
			decodeErrors = append(decodeErrors, fmt.Errorf("%s: %w", candidate.signature, normalizeErr))
			candidates = append(candidates, decoded)
			continue
		}
		decoded.Arguments = arguments
		candidates = append(candidates, decoded)
	}
	if len(candidates) > 1 {
		result.Status = packet.StatusAmbiguous
		result.Candidates = candidates
		signatures := make([]string, len(candidates))
		for index := range candidates {
			signatures[index] = candidates[index].Signature
		}
		return result, &AmbiguousError{Selector: result.Selector, Candidates: signatures}
	}
	if len(decodeErrors) != 0 {
		result.Status = packet.StatusMalformed
		return result, &MalformedError{Selector: result.Selector, Cause: errors.Join(decodeErrors...)}
	}

	result.Status = packet.StatusDecoded
	result.Name = candidates[0].Name
	result.Signature = candidates[0].Signature
	result.Source = candidates[0].Source
	result.Arguments = candidates[0].Arguments
	return result, nil
}

func matchingDefinitions(selector []byte, contracts []ABIBundle) ([]definition, error) {
	definitions, err := standardDefinitions(selector)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, item := range definitions {
		seen[item.signature+"\x00"+item.source] = true
	}
	for _, contract := range contracts {
		for _, abiError := range contract.ABI.Errors {
			key := abiError.Sig + "\x00" + contract.Source
			if bytes.Equal(selector, abiError.ID[:4]) && !seen[key] {
				definitions = append(definitions, definition{name: abiError.Name, signature: abiError.Sig, source: contract.Source, inputs: abiError.Inputs})
				seen[key] = true
			}
		}
	}
	sort.Slice(definitions, func(left, right int) bool {
		if definitions[left].signature == definitions[right].signature {
			return definitions[left].source < definitions[right].source
		}
		return definitions[left].signature < definitions[right].signature
	})
	return definitions, nil
}

func standardDefinitions(selector []byte) ([]definition, error) {
	hexSelector := hex.EncodeToString(selector)
	var name, argumentName, signature, kind string
	switch hexSelector {
	case errorSelector:
		name, argumentName, signature, kind = "Error", "reason", "Error(string)", "string"
	case panicSelector:
		name, argumentName, signature, kind = "Panic", "code", "Panic(uint256)", "uint256"
	default:
		return nil, nil
	}
	abiType, err := ethabi.NewType(kind, "", nil)
	if err != nil {
		return nil, fmt.Errorf("build standard %s type: %w", name, err)
	}
	return []definition{{name: name, signature: signature, source: "builtin", inputs: ethabi.Arguments{{Name: argumentName, Type: abiType}}}}, nil
}
