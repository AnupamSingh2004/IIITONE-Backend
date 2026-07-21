package materials

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidatePDF_RejectsNonPDF(t *testing.T) {
	data, err := os.ReadFile("testdata/corrupted.pdf")
	require.NoError(t, err)

	require.False(t, IsPDF(data))
}

func TestValidatePDF_AcceptsRealPDF(t *testing.T) {
	data, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	require.True(t, IsPDF(data))
}

func TestExtractText_WithTextLayer(t *testing.T) {
	text, hasLayer, err := ExtractText("testdata/with-text.pdf")
	require.NoError(t, err)
	require.True(t, hasLayer)
	require.Contains(t, text, "IIITOne test document")
}

func TestExtractText_NoTextLayer_DegradesGracefully(t *testing.T) {
	text, hasLayer, err := ExtractText("testdata/scanned-no-text.pdf")
	require.NoError(t, err, "no text layer must not be an error, per spec")
	require.False(t, hasLayer)
	require.Empty(t, text)
}

func TestExtractTextFromContentStream_TJKerningBecomesWordBreak(t *testing.T) {
	// Many PDF producers space words apart using a kerning number between TJ
	// string operands instead of a literal space glyph in either string.
	stream := []byte(`BT /F1 12 Tf [(Hello) -250 (World)] TJ ET`)

	got := extractTextFromContentStream(stream)

	require.Contains(t, got, "Hello World", "a sufficiently negative kerning number must become a word break, not be silently dropped")
}

func TestExtractTextFromContentStream_SmallKerningStaysJoined(t *testing.T) {
	// A small kerning adjustment (e.g. tightening two letters of the same word)
	// should NOT be treated as a word break.
	stream := []byte(`BT /F1 12 Tf [(Wo) -20 (rld)] TJ ET`)

	got := extractTextFromContentStream(stream)

	require.Contains(t, got, "World")
	require.NotContains(t, got, "Wo World", "small kerning adjustments must not be misread as a word break")
}

func TestExtractText_UnparseableFile_DegradesGracefully(t *testing.T) {
	// This is the crux of the "never fail the upload" contract this package
	// exists to satisfy — corrupted.pdf can't be parsed by pdfcpu at all.
	text, hasLayer, err := ExtractText("testdata/corrupted.pdf")
	require.NoError(t, err, "unparseable PDF must not be an error, per spec")
	require.False(t, hasLayer)
	require.Empty(t, text)
}

func TestParseLiteralString_DeeplyNestedParens_DoesNotStackOverflow(t *testing.T) {
	// parseLiteralString must be iterative, not recursive: a recursive
	// rewrite (e.g. "for readability") would reintroduce a stack-overflow
	// DoS on a maliciously/accidentally deep-nested literal string.
	const depth = 200_000
	var buf bytes.Buffer
	buf.WriteByte('(')
	for i := 0; i < depth; i++ {
		buf.WriteByte('(')
	}
	buf.WriteString("deepest")
	for i := 0; i < depth; i++ {
		buf.WriteByte(')')
	}
	buf.WriteByte(')')

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = parseLiteralString(buf.Bytes(), 0)
	}()

	select {
	case <-done:
		// completed without hanging or crashing
	case <-time.After(3 * time.Second):
		t.Fatal("parseLiteralString did not return within 3s on deeply nested input — possible non-linear blowup or hang")
	}
}

func TestExtractTextFromContentStream_TruncatedMalformedInput_DoesNotHang(t *testing.T) {
	cases := [][]byte{
		[]byte(`BT (unterminated string`),
		[]byte(`BT <unterminated hex string`),
		[]byte(`BT << /Unbalanced /Dict`),
		[]byte(`BT [(a) -100 (b`),
		[]byte(`\`), // lone trailing backslash at EOF
	}

	for _, stream := range cases {
		done := make(chan struct{})
		go func(s []byte) {
			defer close(done)
			_ = extractTextFromContentStream(s)
		}(stream)

		select {
		case <-done:
			// degraded cleanly, no panic (a panic in the goroutine would crash the test binary)
		case <-time.After(3 * time.Second):
			t.Fatalf("extractTextFromContentStream hung on malformed input: %q", stream)
		}
	}
}
