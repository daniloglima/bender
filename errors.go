package bender

import (
	"fmt"
	"strings"
)

type MissingBindingError struct {
	Key  Key
	Path []Key
}

func (e MissingBindingError) Error() string {
	var sb strings.Builder
	sb.WriteString("bender: missing binding for ")
	sb.WriteString(fmt.Sprintf("%s (name=%q)", e.Key.t, e.Key.name))
	if len(e.Path) > 0 {
		sb.WriteString("\nresolution path:")
		for _, k := range e.Path {
			sb.WriteString("\n  -> ")
			sb.WriteString(fmt.Sprintf("%s (name=%q)", k.t, k.name))
		}
	}
	return sb.String()
}

// MissingBidingError is kept as a compatibility alias.
// Deprecated: use MissingBindingError.
type MissingBidingError = MissingBindingError

type CycleError struct {
	Cycle []Key
}

func (e CycleError) Error() string {
	var sb strings.Builder
	sb.WriteString("bender: dependency cycle detected:\n")
	for _, k := range e.Cycle {
		sb.WriteString("  -> ")
		sb.WriteString(fmt.Sprintf("%s (name=%q)\n", k.t, k.name))
	}
	return sb.String()
}
