// Package materials implements PDF validation and text extraction for
// uploaded course materials.
package materials

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// IsPDF checks the PDF magic bytes (%PDF-) — the cheap, fast rejection of
// non-PDF uploads before any further (expensive) processing is attempted.
func IsPDF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("%PDF-"))
}

// ExtractText attempts to pull the embedded text layer out of a PDF.
//
// Per spec: a missing text layer, or a corrupted/encrypted file that pdfcpu
// can't parse, both degrade gracefully to ("", false, nil) rather than
// failing the upload — the file is still stored either way. A non-nil error
// is reserved for genuine infrastructure failures (e.g. unable to create a
// temp directory), never for "this PDF has no usable text".
//
// Implementation note: the pdfcpu version pinned here (v0.13.0) has no
// dedicated text-extraction API (no ExtractTexts/ExtractTextsFile — that
// function doesn't exist on this version, despite being a reasonable guess).
// Its pkg/api only exposes ExtractContent(File), which dumps the raw,
// undecoded PDF content-stream operators (the literal "BT ... Tj ... ET"
// mini-language) to disk. So this function extracts content streams via
// pdfcpu and then runs a small hand-rolled tokenizer over them to pull the
// operands of text-showing operators (Tj, TJ, ', ") out of the stream. This
// only decodes literal/hex string operands using standard PDF escaping; it
// does not resolve custom CID/Type0 font encodings, so text drawn with
// exotic embedded fonts may come out garbled rather than failing outright —
// consistent with the "never fail, only degrade" contract.
func ExtractText(path string) (string, bool, error) {
	tmpDir, err := os.MkdirTemp("", "iiitone-extract-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(tmpDir)

	if err := api.ExtractContentFile(path, tmpDir, nil, nil); err != nil {
		// pdfcpu couldn't parse this file at all (corrupted/encrypted/etc).
		return "", false, nil
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil || len(entries) == 0 {
		return "", false, nil
	}

	var combined strings.Builder
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
		if err != nil {
			continue
		}
		combined.WriteString(extractTextFromContentStream(content))
		combined.WriteByte('\n')
	}

	text := combined.String()
	if len(strings.TrimSpace(text)) == 0 {
		return "", false, nil
	}
	return text, true, nil
}

// extractTextFromContentStream runs a minimal tokenizer over a raw PDF
// content stream and returns the concatenated operands of text-showing
// operators (Tj, ', ", TJ). It intentionally only understands the subset of
// the content-stream mini-language needed to find string operands: literal
// strings "(...)", hex strings "<...>", arrays "[...]" (for TJ), and bare
// operator/operand tokens; everything else (numbers, names, dict operands,
// graphics operators) is skipped.
func extractTextFromContentStream(data []byte) string {
	var out strings.Builder

	var lastString string
	haveLastString := false
	inArray := false
	var arrayStrings []string

	flushOperator := func(op string) {
		switch op {
		case "Tj", "'", "\"":
			if haveLastString {
				out.WriteString(lastString)
				out.WriteByte('\n')
			}
			lastString = ""
			haveLastString = false
		case "TJ":
			for _, s := range arrayStrings {
				out.WriteString(s)
			}
			out.WriteByte('\n')
			arrayStrings = nil
		}
	}

	i := 0
	n := len(data)
	for i < n {
		c := data[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0:
			i++

		case c == '%':
			for i < n && data[i] != '\n' && data[i] != '\r' {
				i++
			}

		case c == '(':
			str, next := parseLiteralString(data, i)
			i = next
			if inArray {
				arrayStrings = append(arrayStrings, str)
			} else {
				lastString = str
				haveLastString = true
			}

		case c == '<':
			if i+1 < n && data[i+1] == '<' {
				// Inline dictionary (e.g. BDC operands) — skip to matching >>.
				i = skipDict(data, i)
			} else {
				str, next := parseHexString(data, i)
				i = next
				if inArray {
					arrayStrings = append(arrayStrings, str)
				} else {
					lastString = str
					haveLastString = true
				}
			}

		case c == '[':
			inArray = true
			arrayStrings = nil
			i++

		case c == ']':
			inArray = false
			i++

		case c == '{' || c == '}':
			i++

		default:
			start := i
			for i < n && !isDelimiterByte(data[i]) {
				i++
			}
			if i == start {
				i++
				continue
			}
			flushOperator(string(data[start:i]))
		}
	}

	return out.String()
}

// parseLiteralString parses a PDF "(...)" literal string starting at data[i]
// (data[i] == '('), handling nested balanced parens and standard escapes. It
// returns the decoded text and the index just past the closing ')'.
func parseLiteralString(data []byte, i int) (string, int) {
	n := len(data)
	i++ // skip '('
	depth := 1
	var sb strings.Builder

	for i < n && depth > 0 {
		ch := data[i]
		switch {
		case ch == '\\' && i+1 < n:
			esc := data[i+1]
			switch esc {
			case 'n':
				sb.WriteByte('\n')
				i += 2
			case 'r':
				sb.WriteByte('\r')
				i += 2
			case 't':
				sb.WriteByte('\t')
				i += 2
			case 'b':
				sb.WriteByte('\b')
				i += 2
			case 'f':
				sb.WriteByte('\f')
				i += 2
			case '(', ')', '\\':
				sb.WriteByte(esc)
				i += 2
			case '\r':
				i += 2
				if i < n && data[i] == '\n' {
					i++
				}
			case '\n':
				i += 2
			default:
				if esc >= '0' && esc <= '7' {
					j := i + 1
					val := 0
					cnt := 0
					for j < n && cnt < 3 && data[j] >= '0' && data[j] <= '7' {
						val = val*8 + int(data[j]-'0')
						j++
						cnt++
					}
					sb.WriteByte(byte(val))
					i = j
				} else {
					sb.WriteByte(esc)
					i += 2
				}
			}
		case ch == '(':
			depth++
			sb.WriteByte(ch)
			i++
		case ch == ')':
			depth--
			i++
			if depth > 0 {
				sb.WriteByte(ch)
			}
		default:
			sb.WriteByte(ch)
			i++
		}
	}

	return sb.String(), i
}

// parseHexString parses a PDF "<...>" hex string starting at data[i] (data[i]
// == '<'). It returns the decoded bytes as a string and the index just past
// the closing '>'.
func parseHexString(data []byte, i int) (string, int) {
	n := len(data)
	i++ // skip '<'
	var hexDigits []byte
	for i < n && data[i] != '>' {
		if isHexDigit(data[i]) {
			hexDigits = append(hexDigits, data[i])
		}
		i++
	}
	if i < n {
		i++ // skip '>'
	}
	if len(hexDigits)%2 == 1 {
		hexDigits = append(hexDigits, '0')
	}
	decoded := make([]byte, 0, len(hexDigits)/2)
	for k := 0; k+1 < len(hexDigits); k += 2 {
		b, err := strconv.ParseUint(string(hexDigits[k:k+2]), 16, 8)
		if err == nil {
			decoded = append(decoded, byte(b))
		}
	}
	return string(decoded), i
}

// skipDict skips over an inline PDF dictionary "<< ... >>" starting at
// data[i] (data[i:i+2] == "<<"), returning the index just past the matching
// closing ">>".
func skipDict(data []byte, i int) int {
	n := len(data)
	i += 2
	depth := 1
	for i < n && depth > 0 {
		if data[i] == '<' && i+1 < n && data[i+1] == '<' {
			depth++
			i += 2
		} else if data[i] == '>' && i+1 < n && data[i+1] == '>' {
			depth--
			i += 2
		} else {
			i++
		}
	}
	return i
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isDelimiterByte(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f', 0, '(', ')', '<', '>', '[', ']', '{', '}', '%', '/':
		return true
	default:
		return false
	}
}
