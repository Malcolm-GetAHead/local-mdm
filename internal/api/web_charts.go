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

func svgPieChart(title string, slices []pieSlice) string {
	total := 0
	for _, s := range slices {
		total += s.Value
	}
	if total == 0 {
		return ""
	}

	var paths, legends strings.Builder
	cx, cy, r := 40.0, 52.0, 32.0
	startAngle := -math.Pi / 2

	for i, s := range slices {
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

		// Each slice+legend pair in a group for hover
		gOpen := fmt.Sprintf(`<g class="pie-slice" style="cursor:pointer">`)
		paths.WriteString(gOpen)

		if fraction >= 0.999 {
			mx := cx + r*math.Cos(startAngle+math.Pi)
			my := cy + r*math.Sin(startAngle+math.Pi)
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f" fill="%s" class="pie-path"/>`,
				x1, y1, r, r, mx, my, r, r, x1, y1, s.Color))
		} else {
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f L%.1f,%.1f A%.1f,%.1f 0 %d,1 %.1f,%.1f Z" fill="%s" class="pie-path"/>`,
				cx, cy, x1, y1, r, r, largeArc, x2, y2, s.Color))
		}

		ly := 28 + i*18
		paths.WriteString(fmt.Sprintf(`<circle cx="88" cy="%d" r="5" fill="%s"/>`, ly, s.Color))
		paths.WriteString(fmt.Sprintf(`<text x="98" y="%d" class="chart-label">%s (%d)</text>`, ly+4, s.Label, s.Value))
		paths.WriteString(`</g>`)

		startAngle = endAngle
	}

	return fmt.Sprintf(`<svg viewBox="0 0 185 95" class="w-full h-auto max-h-36"><text x="92" y="12" text-anchor="middle" class="chart-title">%s</text>%s%s</svg>`,
		title, paths.String(), legends.String())
}
