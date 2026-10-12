package input

const (
	MaxInputBytes  = 8 << 20
	MaxABIBytes    = 2 << 20
	MaxABIs        = 32
	MaxRevertBytes = 1 << 20
	MaxTraceDepth  = 128
	MaxTraceNodes  = 10_000
)

type Shape string

const (
	ShapeRaw      Shape = "raw"
	ShapeRPCError Shape = "json_rpc_error"
	ShapeReceipt  Shape = "transaction_receipt"
	ShapeTrace    Shape = "geth_call_tracer"
)

type Origin string

const (
	OriginRawFlag Origin = "--raw"
	OriginStdin   Origin = "stdin"
	OriginPath    Origin = "path"
)

type Evidence struct {
	Shape            Shape
	Origin           Origin
	ArtifactPath     string
	ExtractionPath   string
	ArtifactSHA256   string
	RevertDataSHA256 string
	RevertData       []byte
}
