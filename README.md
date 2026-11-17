# evm-failure-packet

`evm-failure-packet` turns EVM revert evidence into deterministic JSON. It runs offline and writes only JSON to stdout; diagnostics go to stderr.

## Build and run

Requires Go 1.24.4 or later.

This repository is source-only. It has no published tags or release artifacts; build the CLI from the checked-out source.

```sh
go build -o evm-failure-packet .
./evm-failure-packet --raw 0x08c379a0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000046f6f707300000000000000000000000000000000000000000000000000000000
./evm-failure-packet --input receipt.json --abi contract.json
cat trace.json | ./evm-failure-packet --input - --abi contract.json --abi library.json
```

The synthetic raw input is the complete ABI encoding of `Error("oops")`. Its exact output is:

```json
{"schema":"evm-failure-packet/v1","provenance":{"input_shape":"raw","artifact_origin":"--raw","extraction_path":"$raw","artifact_sha256":"fdb36d023e421993e96708522915de79d2248966a461f3629a7694ab55d5c7cc","revert_data_sha256":"2c9ca18d84ef2fe1e385552765a4c3da8d4244d84b79051b7463d4793d8f9476"},"failure":{"raw":"0x08c379a0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000046f6f707300000000000000000000000000000000000000000000000000000000","decoded":{"status":"decoded","selector":"0x08c379a0","name":"Error","signature":"Error(string)","source":"builtin","arguments":[{"name":"reason","type":"string","value":{"kind":"scalar","type":"string","text":"oops"}}]}}}
```

Exactly one of `--raw` and `--input` is required. `--abi` may be repeated up to 32 times.

## Accepted input

- Raw `0x`-prefixed revert data, through `--raw` or as input content.
- JSON-RPC error envelope with revert data at `error.data`.
- Failed transaction receipt with revert data at `revertReason`.
- Geth `callTracer` frame, including nested `calls`; failed frame `output` is used.

Errors identify exact JSON Pointer or call-frame path. Unsupported JSON shapes fail explicitly rather than guessing.

## Packet contract

Packets use schema `evm-failure-packet/v1`. Provenance records input shape, artifact origin (`--raw`, `stdin`, or `path`), path when applicable, exact extraction path, artifact SHA-256, and revert-data SHA-256. Decoded standard errors use source `builtin`; custom-error matches use local ABI path. Ambiguous candidates sort by signature, then source.

## Limits

- Main input: 8 MiB.
- Each ABI: 2 MiB; at most 32 ABI files.
- Revert data: 1 MiB decoded.
- Trace: depth 128; 10,000 frames.

Limits fail closed; data is never truncated.

## Exit codes

- `0`: decoded JSON written successfully.
- `1`: input, ABI, validation, or decode failure.
- `2`: command-line usage error.

## Non-goals

No RPC or network access, hosted service, ABI fetching, source maps, transaction replay, simulation, or chain-state reconstruction. Input must already contain revert bytes; a receipt without revert data is not replayed.

## Development

```sh
gofmt -w .
go test -race -shuffle=on -count=1 ./...
go vet ./...
go build -trimpath .
```

Licensed under MIT. See `LICENSE`.
