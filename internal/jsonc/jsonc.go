// Package jsonc reads JSON that a person maintains.
//
// A manifest and a repository's settings are configuration, and configuration
// carries reasons. JSON has nowhere to put one, which is how a `why` key ends
// up beside a value as data rather than as the note it is. TOML is the format
// this repository writes now; the JSON it still reads takes comments instead.
package jsonc

// Strip removes comments and trailing commas, leaving JSON the standard
// decoder accepts.
//
// Every byte it removes is replaced by nothing, and everything else keeps its
// position and its order, so a decoder's error still points inside the
// document even where the stripping changed its length.
func Strip(body []byte) []byte {
	out := make([]byte, 0, len(body))
	for i := 0; i < len(body); {
		switch {
		case body[i] == '"':
			end := endOfString(body, i)
			out = append(out, body[i:end]...)
			i = end
		case body[i] == '/' && i+1 < len(body) && body[i+1] == '/':
			i = endOfLine(body, i)
		case body[i] == '/' && i+1 < len(body) && body[i+1] == '*':
			i = endOfBlock(body, i)
		case body[i] == ',':
			// A comma before a closing brace or bracket is the other thing a
			// hand-edited file carries, and it is one reordered line away at
			// any time.
			if next := nextMeaningful(body, i+1); next < len(body) && (body[next] == '}' || body[next] == ']') {
				i++
				continue
			}
			out = append(out, body[i])
			i++
		default:
			out = append(out, body[i])
			i++
		}
	}
	return out
}

// endOfString returns the index just past the string starting at i, which is
// where a quote inside it has to stop a scan from treating // as a comment.
func endOfString(body []byte, i int) int {
	for j := i + 1; j < len(body); j++ {
		switch body[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(body)
}

func endOfLine(body []byte, i int) int {
	for j := i; j < len(body); j++ {
		if body[j] == '\n' {
			return j
		}
	}
	return len(body)
}

func endOfBlock(body []byte, i int) int {
	for j := i + 2; j+1 < len(body); j++ {
		if body[j] == '*' && body[j+1] == '/' {
			return j + 2
		}
	}
	return len(body)
}

// nextMeaningful is the index of the next byte that is neither whitespace nor
// a comment, so a comma followed by a comment and then a brace is still a
// trailing comma.
func nextMeaningful(body []byte, i int) int {
	for i < len(body) {
		switch {
		case body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r':
			i++
		case body[i] == '/' && i+1 < len(body) && body[i+1] == '/':
			i = endOfLine(body, i)
		case body[i] == '/' && i+1 < len(body) && body[i+1] == '*':
			i = endOfBlock(body, i)
		default:
			return i
		}
	}
	return i
}
