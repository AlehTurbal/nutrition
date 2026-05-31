// Package llm is a thin client for the Anthropic Messages API plus helpers for
// coaxing structured JSON out of model responses.
package llm

import (
	"errors"
	"strings"
)

// ErrNoJSON is returned when no JSON value can be found in the text.
var ErrNoJSON = errors.New("no JSON found in response")

// ExtractJSON returns the first balanced JSON object or array found in s,
// tolerating surrounding prose or ```json fences.
func ExtractJSON(s string) (string, error) {
	start := -1
	var open, close byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start, open, close = i, '{', '}'
			break
		}
		if s[i] == '[' {
			start, open, close = i, '[', ']'
			break
		}
	}
	if start == -1 {
		return "", ErrNoJSON
	}

	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1]), nil
			}
		}
	}
	return "", ErrNoJSON
}
