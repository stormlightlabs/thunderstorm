package check

import (
	"strings"
	"testing"
)

// A document carries its identifier for the life of the repository and an
// issue cites the document by it, so the two properties that matter are the
// alphabet a reader transcribes and the odds of a collision.
func TestAULIDIsCrockfordBase32(t *testing.T) {
	id, err := ULID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 26 {
		t.Fatalf("%q is %d characters, want 26", id, len(id))
	}
	// I, L, O, and U are outside the alphabet so a transcribed identifier
	// cannot turn into a different one.
	for _, r := range id {
		if !strings.ContainsRune(crockford, r) {
			t.Errorf("%q holds %q, which the alphabet omits", id, r)
		}
	}
}

func TestTwoULIDsInTheSameMillisecondDiffer(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id, err := ULID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("%s was generated twice", id)
		}
		seen[id] = true
	}
}

// The timestamp is the leading half, so identifiers made in order sort in
// order and a tree of documents reads chronologically.
func TestULIDsSortByWhenTheyWereMade(t *testing.T) {
	first, err := ULID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := ULID()
	if err != nil {
		t.Fatal(err)
	}
	if first[:10] > second[:10] {
		t.Errorf("%s was made before %s and sorts after it", first, second)
	}
}
