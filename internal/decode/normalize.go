package decode

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/otrumb/evm-failure-packet/internal/packet"
)

func normalizeArguments(arguments ethabi.Arguments, values []any) ([]packet.Argument, error) {
	if len(arguments) != len(values) {
		return nil, fmt.Errorf("argument count mismatch: have %d, want %d", len(values), len(arguments))
	}
	result := make([]packet.Argument, len(arguments))
	for index, argument := range arguments {
		value, err := normalizeValue(argument.Type, reflect.ValueOf(values[index]))
		if err != nil {
			return nil, fmt.Errorf("argument %d: %w", index, err)
		}
		result[index] = packet.Argument{Name: argument.Name, Type: argument.Type.String(), Value: value}
	}
	return result, nil
}

func normalizeValue(abiType ethabi.Type, value reflect.Value) (packet.Value, error) {
	if !value.IsValid() {
		return packet.Value{}, fmt.Errorf("invalid %s value", abiType.String())
	}
	switch abiType.T {
	case ethabi.SliceTy, ethabi.ArrayTy:
		items := make([]packet.Value, value.Len())
		for index := range value.Len() {
			item, err := normalizeValue(*abiType.Elem, value.Index(index))
			if err != nil {
				return packet.Value{}, err
			}
			items[index] = item
		}
		return packet.Value{Kind: packet.ValueArray, Type: abiType.String(), Items: items}, nil
	case ethabi.TupleTy:
		fields := make([]packet.Field, len(abiType.TupleElems))
		for index, element := range abiType.TupleElems {
			field, err := normalizeValue(*element, value.Field(index))
			if err != nil {
				return packet.Value{}, err
			}
			fields[index] = packet.Field{Name: abiType.TupleRawNames[index], Value: field}
		}
		return packet.Value{Kind: packet.ValueTuple, Type: abiType.String(), Fields: fields}, nil
	default:
		text, err := scalarText(abiType, value)
		if err != nil {
			return packet.Value{}, err
		}
		return packet.Value{Kind: packet.ValueScalar, Type: abiType.String(), Text: text}, nil
	}
}

func scalarText(abiType ethabi.Type, value reflect.Value) (string, error) {
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", fmt.Errorf("nil %s value", abiType.String())
		}
		if number, ok := value.Interface().(*big.Int); ok {
			return number.String(), nil
		}
		value = value.Elem()
	}
	switch abiType.T {
	case ethabi.IntTy, ethabi.UintTy:
		if value.CanInterface() {
			if number, ok := value.Interface().(big.Int); ok {
				return number.String(), nil
			}
		}
		if value.Kind() >= reflect.Int && value.Kind() <= reflect.Int64 {
			return strconv.FormatInt(value.Int(), 10), nil
		}
		if value.Kind() >= reflect.Uint && value.Kind() <= reflect.Uint64 {
			return strconv.FormatUint(value.Uint(), 10), nil
		}
	case ethabi.BoolTy:
		return strconv.FormatBool(value.Bool()), nil
	case ethabi.StringTy:
		return value.String(), nil
	case ethabi.AddressTy:
		if address, ok := value.Interface().(common.Address); ok {
			return strings.ToLower(address.Hex()), nil
		}
	case ethabi.BytesTy:
		return "0x" + hex.EncodeToString(value.Bytes()), nil
	case ethabi.FixedBytesTy, ethabi.FunctionTy:
		bytes := make([]byte, value.Len())
		reflect.Copy(reflect.ValueOf(bytes), value)
		return "0x" + hex.EncodeToString(bytes), nil
	}
	return "", fmt.Errorf("unsupported %s value %s", abiType.String(), value.Type())
}
