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
