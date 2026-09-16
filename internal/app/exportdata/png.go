package exportdata

import (
	"fmt"
	"image"
	"image/png"
	"io"
)

// WriteChartPNG kodiert ein bereits gerendertes Diagramm (siehe
// analysis.ChartRenderer) als PNG nach w.
func WriteChartPNG(w io.Writer, img image.Image) error {
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("diagramm konnte nicht als png geschrieben werden: %w", err)
	}
	return nil
}
