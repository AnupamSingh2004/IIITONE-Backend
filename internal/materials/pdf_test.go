package materials

import (
	"os"
	"testing"

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
