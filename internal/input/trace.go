package input

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

type traceCandidate struct {
	output string
	path   string
	found  bool
}

type traceParser struct {
	decoder *json.Decoder
	nodes   int
}

func parseTrace(content []byte) (Evidence, error) {
	parser := traceParser{decoder: json.NewDecoder(bytes.NewReader(content))}
	candidate, err := parser.parseFrame("", 1)
	if err != nil {
		return Evidence{}, err
	}
	if err := parser.rejectTrailing(); err != nil {
		return Evidence{}, err
	}
	if !candidate.found {
		return Evidence{}, fmt.Errorf("%w: callTracer contains no failed frame with output", ErrInvalidInput)
	}
	evidence, err := parseRevert(candidate.output, candidate.path)
	if err != nil {
		return Evidence{}, err
	}
	evidence.Shape = ShapeTrace
	return evidence, nil
}

func (p *traceParser) parseFrame(path string, depth int) (traceCandidate, error) {
	p.nodes++
	if p.nodes > MaxTraceNodes {
		return traceCandidate{}, fmt.Errorf("%w: trace nodes exceed %d", ErrLimit, MaxTraceNodes)
	}
	if depth > MaxTraceDepth {
		return traceCandidate{}, &PathError{Path: path, Err: fmt.Errorf("%w: trace depth exceeds %d", ErrLimit, MaxTraceDepth)}
	}
	if err := p.expectDelimiter('{'); err != nil {
		return traceCandidate{}, err
	}
	var output, frameError string
	var childCandidate traceCandidate
	for p.decoder.More() {
		field, err := p.readString()
		if err != nil {
			return traceCandidate{}, err
		}
		switch field {
		case "type":
			if _, err := p.readFrameString(); err != nil {
				return traceCandidate{}, err
			}
		case "output":
			output, err = p.readFrameString()
			if err != nil {
				return traceCandidate{}, err
			}
		case "error":
			frameError, err = p.readFrameString()
			if err != nil {
				return traceCandidate{}, err
			}
		case "calls":
			childCandidate, err = p.parseCalls(path, depth)
			if err != nil {
				return traceCandidate{}, err
			}
		default:
			if err := p.skipValue(); err != nil {
				return traceCandidate{}, err
			}
		}
	}
	if err := p.expectDelimiter('}'); err != nil {
		return traceCandidate{}, err
	}
	if frameError != "" && output != "" {
		return traceCandidate{output: output, path: path + "/output", found: true}, nil
	}
	return childCandidate, nil
}

func (p *traceParser) parseCalls(path string, depth int) (traceCandidate, error) {
	token, err := p.decoder.Token()
	if err != nil {
		return traceCandidate{}, malformedTrace(err)
	}
	if token == nil {
		return traceCandidate{}, nil
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '[' {
		return traceCandidate{}, malformedTrace(fmt.Errorf("calls must be an array"))
	}
	var first traceCandidate
	for index := 0; p.decoder.More(); index++ {
		candidate, err := p.parseFrame(path+"/calls/"+strconv.Itoa(index), depth+1)
		if err != nil {
			return traceCandidate{}, err
		}
		if !first.found && candidate.found {
			first = candidate
		}
	}
	if err := p.expectDelimiter(']'); err != nil {
		return traceCandidate{}, err
	}
	return first, nil
}

func (p *traceParser) readString() (string, error) {
	token, err := p.decoder.Token()
	if err != nil {
		return "", malformedTrace(err)
	}
	value, ok := token.(string)
	if !ok {
		return "", malformedTrace(fmt.Errorf("expected string"))
	}
	return value, nil
}

func (p *traceParser) readFrameString() (string, error) {
	token, err := p.decoder.Token()
	if err != nil {
		return "", malformedTrace(err)
	}
	if token == nil {
		return "", nil
	}
	value, ok := token.(string)
	if !ok {
		return "", malformedTrace(fmt.Errorf("expected string"))
	}
	return value, nil
}

func (p *traceParser) expectDelimiter(want json.Delim) error {
	token, err := p.decoder.Token()
	if err != nil {
		return malformedTrace(err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != want {
		return malformedTrace(fmt.Errorf("expected %q", want))
	}
	return nil
}

func (p *traceParser) skipValue() error {
	token, err := p.decoder.Token()
	if err != nil {
		return malformedTrace(err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || (delimiter != '{' && delimiter != '[') {
		return nil
	}
	open := 1
	for open > 0 {
		token, err = p.decoder.Token()
		if err != nil {
			return malformedTrace(err)
		}
		delimiter, ok = token.(json.Delim)
		if !ok {
			continue
		}
		switch delimiter {
		case '{', '[':
			open++
		case '}', ']':
			open--
		}
	}
	return nil
}

func (p *traceParser) rejectTrailing() error {
	_, err := p.decoder.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return malformedTrace(err)
	}
	return malformedTrace(fmt.Errorf("multiple JSON values"))
}

func malformedTrace(err error) error {
	return fmt.Errorf("%w: malformed callTracer: %w", ErrInvalidInput, err)
}
