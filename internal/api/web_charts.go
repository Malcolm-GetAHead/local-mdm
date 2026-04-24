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

// svgPieChart generates an SVG pie chart as a template-safe HTML string.
func svgPieChart(title string, slices []pieSlice) string {
	total := 0
	for _, s := range slices {
		total += s.Value
	}
	if total == 0 {
		return ""
	}

	var paths, legends strings.Builder
	cx, cy, r := 60.0, 60.0, 50.0
	startAngle := -math.Pi / 2 // start at top

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

		if fraction >= 0.999 {
			// Full circle — draw two arcs
			mx := cx + r*math.Cos(startAngle+math.Pi)
			my := cy + r*math.Sin(startAngle+math.Pi)
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f" fill="%s"/>`,
				x1, y1, r, r, mx, my, r, r, x1, y1, s.Color))
		} else {
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f L%.1f,%.1f A%.1f,%.1f 0 %d,1 %.1f,%.1f Z" fill="%s"/>`,
				cx, cy, x1, y1, r, r, largeArc, x2, y2, s.Color))
		}

		// Legend
		ly := 20 + i*18
		legends.WriteString(fmt.Sprintf(`<rect x="130" y="%d" width="10" height="10" rx="2" fill="%s"/>`, ly, s.Color))
		legends.WriteString(fmt.Sprintf(`<text x="145" y="%d" class="chart-label">%s (%d)</text>`, ly+9, s.Label, s.Value))

		startAngle = endAngle
	}

	return fmt.Sprintf(`<svg viewBox="0 0 220 120" class="w-full h-auto"><text x="60" y="12" text-anchor="middle" class="chart-title">%s</text>%s%s</svg>`,
		title, paths.String(), legends.String())
}
