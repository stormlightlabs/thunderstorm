package jsonc

import (
	"encoding/json"
	"testing"
)

func TestStripLeavesJSONADecoderAccepts(t *testing.T) {
	for name, body := range map[string]string{
		"a line comment":        "{\n  // why this is here\n  \"a\": 1\n}",
		"a block comment":       "{\n  /* why\n     this is here */\n  \"a\": 1\n}",
		"a comment after value": "{\"a\": 1 // the reason\n}",
		"a trailing comma":      "{\"a\": 1,}",
		"a trailing comma in a list": `{"a": 1, "b": [
			2,
			3,
		]}`,
		"a comma before a comment then a brace": "{\"a\": 1, // last\n}",
		"no comments at all":                    `{"a": 1}`,
	} {
		t.Run(name, func(t *testing.T) {
			var held map[string]any
			if err := json.Unmarshal(Strip([]byte(body)), &held); err != nil {
				t.Fatalf("%v, from %q", err, Strip([]byte(body)))
			}
			if held["a"] != float64(1) {
				t.Errorf("a = %v", held["a"])
			}
		})
	}
}

// A comment marker inside a string is content, and so is a comma before a
// brace that the string only looks like.
func TestStringsAreNotStripped(t *testing.T) {
	body := `{"url": "https://example.invalid/x", "note": "a /* b */ c", "path": "x,}"}`
	var held map[string]string
	if err := json.Unmarshal(Strip([]byte(body)), &held); err != nil {
		t.Fatalf("%v, from %q", err, Strip([]byte(body)))
	}
	for key, want := range map[string]string{
		"url":  "https://example.invalid/x",
		"note": "a /* b */ c",
		"path": "x,}",
	} {
		if held[key] != want {
			t.Errorf("%s = %q, want %q", key, held[key], want)
		}
	}
}

// An escaped quote does not end the string it is in.
func TestAnEscapedQuoteDoesNotEndTheString(t *testing.T) {
	body := `{"note": "he said \"// not a comment\"", "a": 1}`
	var held map[string]any
	if err := json.Unmarshal(Strip([]byte(body)), &held); err != nil {
		t.Fatalf("%v, from %q", err, Strip([]byte(body)))
	}
	if held["note"] != `he said "// not a comment"` {
		t.Errorf("note = %v", held["note"])
	}
}

// Stripping is not validation: what was not JSON before is not JSON after, and
// the decoder is what says so.
func TestBrokenJSONStaysBroken(t *testing.T) {
	var held map[string]any
	if err := json.Unmarshal(Strip([]byte(`{"a": `)), &held); err == nil {
		t.Error("a truncated document was accepted")
	}
}
