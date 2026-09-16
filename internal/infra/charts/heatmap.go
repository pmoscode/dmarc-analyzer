package charts

import (
	"fmt"
	"image"
	"image/draw"

	chart "github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
)

// Heatmaps sind in go-chart kein eigener Diagrammtyp — die Zellen werden
// deshalb direkt als Rechtecke auf ein image.RGBA gezeichnet
// (image/draw), Beschriftungen über denselben Font/GraphicContext, den
// go-chart selbst nutzt (drawing.RasterGraphicContext), statt eine
// zusätzliche Abhängigkeit nur für Text im Bild einzuführen.
const (
	heatmapCellWidth       = 36
	heatmapCellHeight      = 22
	heatmapLabelGutterLeft = 90
	heatmapLabelGutterTop  = 24
	heatmapMargin          = 8
)

var colorNoData = drawing.Color{R: 0xe4, G: 0xe4, B: 0xe4, A: 0xff}

// HeatmapChart zeichnet die Quelle-×-Tag-Pass-Rate-Matrix.
func (Renderer) HeatmapChart(data analysis.Heatmap) (image.Image, error) {
	if len(data.Sources) == 0 || len(data.Days) == 0 {
		return blankImage(defaultHeight), nil
	}

	width := heatmapLabelGutterLeft + len(data.Days)*heatmapCellWidth + heatmapMargin
	height := heatmapLabelGutterTop + len(data.Sources)*heatmapCellHeight + heatmapMargin

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	drawFill(img, image.NewUniform(chart.ColorWhite))

	gc, err := drawing.NewRasterGraphicContext(img)
	if err != nil {
		return nil, fmt.Errorf("heatmap-kontext konnte nicht erzeugt werden: %w", err)
	}
	font, err := chart.GetDefaultFont()
	if err != nil {
		return nil, fmt.Errorf("schriftart konnte nicht geladen werden: %w", err)
	}
	gc.SetFont(font)
	gc.SetFontSize(9)
	gc.SetFillColor(chart.ColorBlack)

	for si, source := range data.Sources {
		y := heatmapLabelGutterTop + si*heatmapCellHeight
		_, _ = gc.FillStringAt(source.String(), 2, float64(y+heatmapCellHeight-6))

		for di := range data.Days {
			x := heatmapLabelGutterLeft + di*heatmapCellWidth
			cell := data.Cells[si][di]
			cellColor := colorNoData
			if cell.HasData {
				cellColor = passRateColor(cell.PassRate)
			}
			drawFill(img.SubImage(image.Rect(x, y, x+heatmapCellWidth-2, y+heatmapCellHeight-2)).(draw.Image), image.NewUniform(cellColor))
		}
	}

	for di, day := range data.Days {
		x := heatmapLabelGutterLeft + di*heatmapCellWidth
		gc.SetFontSize(8)
		_, _ = gc.FillStringAt(day.Format("02.01."), float64(x), heatmapLabelGutterTop-8)
	}

	return img, nil
}

// drawFill füllt dst vollständig mit src (image/draw statt go-charts
// Pfad-API — für einfache Rechtecke direkter und ohne Path-Konstruktion).
func drawFill(dst draw.Image, src image.Image) {
	draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)
}
