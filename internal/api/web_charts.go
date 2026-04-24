package api

import (
	"fmt"
	"math"
	"strings"
)

type pieSlice struct {
	Label string
	Value int
	Color string
}

type chartData struct {
	Title  string
	Pie    string // SVG circle only
	Legend []pieSlice
}

func buildChart(title string, slices []pieSlice) chartData {
	total := 0
	for _, s := range slices {
		total += s.Value
	}

	var paths strings.Builder
	cx, cy, r := 50.0, 50.0, 48.0
	startAngle := -math.Pi / 2

	for _, s := range slices {
		if s.Value == 0 {
			continue
		}
		fraction := float64(s.Value) / float64(total)
		endAngle := startAngle + fraction*2*math.Pi

		x1 := cx + r*math.Cos(startAngle)
		y1 := cy + r*math.Sin(startAngle)
		x2 := cx + r*math.Cos(endAngle)
		y2 := cy + r*math.Sin(endAngle)

		largeArc := 0
		if fraction > 0.5 {
			largeArc = 1
		}

		if fraction >= 0.999 {
			mx := cx + r*math.Cos(startAngle+math.Pi)
			my := cy + r*math.Sin(startAngle+math.Pi)
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f" fill="%s"/>`,
				x1, y1, r, r, mx, my, r, r, x1, y1, s.Color))
		} else {
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f L%.1f,%.1f A%.1f,%.1f 0 %d,1 %.1f,%.1f Z" fill="%s" class="hover:opacity-70 transition-opacity"/>`,
				cx, cy, x1, y1, r, r, largeArc, x2, y2, s.Color))
		}
		startAngle = endAngle
	}

	var legend []pieSlice
	for _, s := range slices {
		if s.Value > 0 {
			legend = append(legend, s)
		}
	}

	return chartData{
		Title:  title,
		Pie:    fmt.Sprintf(`<svg viewBox="0 0 100 100" width="100" height="100">%s</svg>`, paths.String()),
		Legend: legend,
	}
}
