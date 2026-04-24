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
	cx, cy, r := 40.0, 48.0, 32.0
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

		if fraction >= 0.999 {
			mx := cx + r*math.Cos(startAngle+math.Pi)
			my := cy + r*math.Sin(startAngle+math.Pi)
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f A%.1f,%.1f 0 1,1 %.1f,%.1f" fill="%s"/>`,
				x1, y1, r, r, mx, my, r, r, x1, y1, s.Color))
		} else {
			paths.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f L%.1f,%.1f A%.1f,%.1f 0 %d,1 %.1f,%.1f Z" fill="%s"/>`,
				cx, cy, x1, y1, r, r, largeArc, x2, y2, s.Color))
		}

		ly := 24 + i*16
		legends.WriteString(fmt.Sprintf(`<circle cx="88" cy="%d" r="4" fill="%s"/>`, ly, s.Color))
		legends.WriteString(fmt.Sprintf(`<text x="96" y="%d" class="chart-label">%s (%d)</text>`, ly+4, s.Label, s.Value))

		startAngle = endAngle
	}

	return fmt.Sprintf(`<svg viewBox="0 0 170 90" class="w-full h-auto max-h-32"><text x="85" y="10" text-anchor="middle" class="chart-title">%s</text>%s%s</svg>`,
		title, paths.String(), legends.String())
}
