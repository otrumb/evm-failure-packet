package decode

import "fmt"

type MalformedError struct {
	Selector string
	Cause    error
}

func (e *MalformedError) Error() string {
	if e.Selector == "" {
		return "malformed revert data: missing 4-byte selector"
	}
	return fmt.Sprintf("malformed revert data for selector %s: %v", e.Selector, e.Cause)
}

func (e *MalformedError) Unwrap() error { return e.Cause }

type AmbiguousError struct {
	Selector   string
	Candidates []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("ambiguous revert selector %s: %v", e.Selector, e.Candidates)
}
