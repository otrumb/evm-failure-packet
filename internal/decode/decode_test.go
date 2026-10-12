package decode_test

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/otrumb/evm-failure-packet/internal/decode"
	"github.com/otrumb/evm-failure-packet/internal/packet"
)

func TestDecodeStandardErrorAndPanic(t *testing.T) {
	tests := []struct {
		name      string
		selector  []byte
		kind      string
		value     any
		signature string
		text      string
	}{
		{name: "Error string", selector: []byte{0x08, 0xc3, 0x79, 0xa0}, kind: "string", value: "denied", signature: "Error(string)", text: "denied"},
		{name: "Panic code", selector: []byte{0x4e, 0x48, 0x7b, 0x71}, kind: "uint256", value: big.NewInt(17), signature: "Panic(uint256)", text: "17"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			arguments := argumentsOf(t, test.kind)
			payload := append(append([]byte{}, test.selector...), mustPack(t, arguments, test.value)...)

			// When
			result, err := decode.Decode(payload)

			// Then
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if result.Status != packet.StatusDecoded || result.Signature != test.signature {
				t.Fatalf("result = %#v", result)
			}
			if len(result.Arguments) != 1 || result.Arguments[0].Value.Text != test.text {
				t.Fatalf("arguments = %#v", result.Arguments)
			}
		})
	}
}

func TestDecodeCustomErrorNormalizesValues(t *testing.T) {
	// Given
	contract := mustABI(t, `[{"type":"error","name":"Denied","inputs":[{"name":"account","type":"address"},{"name":"amounts","type":"uint256[]"},{"name":"proof","type":"bytes32"}]}]`)
	customError := contract.Errors["Denied"]
	address := common.HexToAddress("0x00000000000000000000000000000000000000AB")
	var proof [32]byte
	proof[31] = 0xff
	payload := append(append([]byte{}, customError.ID[:4]...), mustPack(t, customError.Inputs, address, []*big.Int{big.NewInt(2), big.NewInt(3)}, proof)...)

	// When
	result, err := decode.Decode(payload, decode.ABIBundle{Source: "contract.json", ABI: contract})

	// Then
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != packet.StatusDecoded || result.Signature != "Denied(address,uint256[],bytes32)" || result.Source != "contract.json" {
		t.Fatalf("result = %#v", result)
	}
	if result.Arguments[0].Value.Text != "0x00000000000000000000000000000000000000ab" {
		t.Fatalf("address = %#v", result.Arguments[0])
	}
	amounts := result.Arguments[1].Value.Items
	if len(amounts) != 2 || amounts[0].Text != "2" || amounts[1].Text != "3" {
		t.Fatalf("amounts = %#v", amounts)
	}
	if result.Arguments[2].Value.Text != "0x"+strings.Repeat("00", 31)+"ff" {
		t.Fatalf("proof = %#v", result.Arguments[2])
	}
}

func TestDecodePreservesUnknownMalformedAndAmbiguousOutcomes(t *testing.T) {
	t.Run("unknown selector", func(t *testing.T) {
		// Given
		payload := []byte{0xde, 0xad, 0xbe, 0xef}

		// When
		result, err := decode.Decode(payload)

		// Then
		if err != nil || result.Status != packet.StatusUnknown || result.Selector != "0xdeadbeef" {
			t.Fatalf("result = %#v, error = %v", result, err)
		}
	})

	t.Run("missing selector", func(t *testing.T) {
		// Given
		payload := []byte{0x08, 0xc3, 0x79}

		// When
		result, err := decode.Decode(payload)

		// Then
		var malformed *decode.MalformedError
		if !errors.As(err, &malformed) || result.Status != packet.StatusMalformed {
			t.Fatalf("result = %#v, error = %v", result, err)
		}
	})

	t.Run("invalid standard payload", func(t *testing.T) {
		// Given
		payload := []byte{0x08, 0xc3, 0x79, 0xa0, 0x01}

		// When
		result, err := decode.Decode(payload)

		// Then
		var malformed *decode.MalformedError
		if !errors.As(err, &malformed) || result.Status != packet.StatusMalformed || malformed.Selector != "0x08c379a0" {
			t.Fatalf("result = %#v, error = %v", result, err)
		}
	})

	t.Run("selector collision", func(t *testing.T) {
		// Given
		arguments := argumentsOf(t, "uint256")
		alpha := ethabi.NewError("Alpha", arguments)
		zulu := ethabi.NewError("Zulu", arguments)
		zulu.ID = alpha.ID
		contract := ethabi.ABI{Errors: map[string]ethabi.Error{"z": zulu, "a": alpha}}
		payload := append(append([]byte{}, alpha.ID[:4]...), mustPack(t, arguments, big.NewInt(7))...)

		// When
		result, err := decode.Decode(payload, decode.ABIBundle{Source: "contract.json", ABI: contract})

		// Then
		var ambiguous *decode.AmbiguousError
		if !errors.As(err, &ambiguous) || result.Status != packet.StatusAmbiguous {
			t.Fatalf("result = %#v, error = %v", result, err)
		}
		if len(result.Candidates) != 2 || result.Candidates[0].Signature != "Alpha(uint256)" || result.Candidates[1].Signature != "Zulu(uint256)" {
			t.Fatalf("candidates = %#v", result.Candidates)
		}
	})

	t.Run("collision is ambiguous when one candidate payload is malformed", func(t *testing.T) {
		// Given
		integerArguments := argumentsOf(t, "uint256")
		stringArguments := argumentsOf(t, "string")
		alpha := ethabi.NewError("Alpha", integerArguments)
		zulu := ethabi.NewError("Zulu", stringArguments)
		zulu.ID = alpha.ID
		contract := ethabi.ABI{Errors: map[string]ethabi.Error{"z": zulu, "a": alpha}}
		payload := append(append([]byte{}, alpha.ID[:4]...), mustPack(t, integerArguments, big.NewInt(7))...)

		// When
		result, err := decode.Decode(payload, decode.ABIBundle{Source: "contract.json", ABI: contract})

		// Then
		var ambiguous *decode.AmbiguousError
		if !errors.As(err, &ambiguous) || result.Status != packet.StatusAmbiguous || len(result.Candidates) != 2 {
			t.Fatalf("result = %#v, error = %v", result, err)
		}
		if len(result.Candidates[0].Arguments) != 1 || len(result.Candidates[1].Arguments) != 0 {
			t.Fatalf("candidates = %#v", result.Candidates)
		}
	})

	t.Run("same signature is ordered by source", func(t *testing.T) {
		arguments := argumentsOf(t, "uint256")
		customError := ethabi.NewError("Denied", arguments)
		contract := ethabi.ABI{Errors: map[string]ethabi.Error{"Denied": customError}}
		payload := append(append([]byte{}, customError.ID[:4]...), mustPack(t, arguments, big.NewInt(7))...)

		result, err := decode.Decode(payload,
			decode.ABIBundle{Source: "zulu.json", ABI: contract},
			decode.ABIBundle{Source: "alpha.json", ABI: contract},
		)

		var ambiguous *decode.AmbiguousError
		if !errors.As(err, &ambiguous) || result.Status != packet.StatusAmbiguous || len(result.Candidates) != 2 {
			t.Fatalf("result = %#v, error = %v", result, err)
		}
		if result.Candidates[0].Source != "alpha.json" || result.Candidates[1].Source != "zulu.json" {
			t.Fatalf("candidates = %#v", result.Candidates)
		}
	})
}

func argumentsOf(t *testing.T, kinds ...string) ethabi.Arguments {
	t.Helper()
	arguments := make(ethabi.Arguments, len(kinds))
	for index, kind := range kinds {
		abiType, err := ethabi.NewType(kind, "", nil)
		if err != nil {
			t.Fatalf("new ABI type %s: %v", kind, err)
		}
		arguments[index] = ethabi.Argument{Type: abiType}
	}
	return arguments
}

func mustPack(t *testing.T, arguments ethabi.Arguments, values ...any) []byte {
	t.Helper()
	packed, err := arguments.Pack(values...)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	return packed
}

func mustABI(t *testing.T, definition string) ethabi.ABI {
	t.Helper()
	contract, err := ethabi.JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	return contract
}
