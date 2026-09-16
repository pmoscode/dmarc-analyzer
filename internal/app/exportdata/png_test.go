package exportdata_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
)

func TestWriteChartPNG_RoundTrips(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	src.Set(0, 0, color.RGBA{R: 200, G: 20, B: 20, A: 255})

	var buf bytes.Buffer
	require.NoError(t, exportdata.WriteChartPNG(&buf, src))
	require.NotZero(t, buf.Len())

	decoded, err := png.Decode(&buf)
	require.NoError(t, err)
	require.Equal(t, src.Bounds(), decoded.Bounds())

	r, g, b, _ := decoded.At(0, 0).RGBA()
	require.Equal(t, uint32(200*0x101), r)
	require.Equal(t, uint32(20*0x101), g)
	require.Equal(t, uint32(20*0x101), b)
}
