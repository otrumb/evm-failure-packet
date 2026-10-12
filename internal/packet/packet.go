package packet

import "encoding/hex"

const Schema = "evm-failure-packet/v1"

type DecodeStatus string

type InputShape string

type ArtifactOrigin string

const (
	StatusDecoded   DecodeStatus = "decoded"
	StatusUnknown   DecodeStatus = "unknown"
	StatusMalformed DecodeStatus = "malformed"
	StatusAmbiguous DecodeStatus = "ambiguous"
)

type Packet struct {
	Schema     string     `json:"schema"`
	Provenance Provenance `json:"provenance"`
	Failure    Failure    `json:"failure"`
}

type Provenance struct {
	InputShape       InputShape     `json:"input_shape"`
	ArtifactOrigin   ArtifactOrigin `json:"artifact_origin"`
	ArtifactPath     string         `json:"artifact_path,omitempty"`
	ExtractionPath   string         `json:"extraction_path"`
	ArtifactSHA256   string         `json:"artifact_sha256"`
	RevertDataSHA256 string         `json:"revert_data_sha256"`
}

type Failure struct {
	Raw     string  `json:"raw"`
	Decoded Decoded `json:"decoded"`
}

type Decoded struct {
	Status     DecodeStatus `json:"status"`
	Selector   string       `json:"selector,omitempty"`
	Name       string       `json:"name,omitempty"`
	Signature  string       `json:"signature,omitempty"`
	Source     string       `json:"source,omitempty"`
	Arguments  []Argument   `json:"arguments,omitempty"`
	Candidates []Candidate  `json:"candidates,omitempty"`
}

type Candidate struct {
	Name      string     `json:"name"`
	Signature string     `json:"signature"`
	Source    string     `json:"source"`
	Arguments []Argument `json:"arguments,omitempty"`
}

type Argument struct {
	Name  string `json:"name,omitempty"`
	Type  string `json:"type"`
	Value Value  `json:"value"`
}

type ValueKind string

const (
	ValueScalar ValueKind = "scalar"
	ValueArray  ValueKind = "array"
	ValueTuple  ValueKind = "tuple"
)

type Value struct {
	Kind   ValueKind `json:"kind"`
	Type   string    `json:"type"`
	Text   string    `json:"text,omitempty"`
	Items  []Value   `json:"items,omitempty"`
	Fields []Field   `json:"fields,omitempty"`
}

type Field struct {
	Name  string `json:"name,omitempty"`
	Value Value  `json:"value"`
}

func New(raw []byte, provenance Provenance, decoded Decoded) Packet {
	return Packet{
		Schema:     Schema,
		Provenance: provenance,
		Failure:    Failure{Raw: "0x" + hex.EncodeToString(raw), Decoded: decoded},
	}
}
