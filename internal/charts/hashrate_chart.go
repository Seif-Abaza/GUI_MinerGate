// =============================================================================
// Package charts - رسم بياني Hashrate بـ fynesimplechart v0.2.1 (v1.0.4)
// =============================================================================
// API الصحيح لـ fynesimplechart v0.2.1:
//
//	fynesimplechart.NewNode(x, y float32) *Node
//	fynesimplechart.NewPlot(nodes []Node, title string) *Plot
//	fynesimplechart.NewGraphWidget(plots []Plot) *ScatterPlot
//
// =============================================================================
package charts

import (
	"encoding/csv"
	"image/color"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	fynesimplechart "github.com/alexiusacademia/fynesimplechart"
)

// ألوان Catppuccin Mocha
var (
	ChartLineColor  = color.RGBA{R: 137, G: 180, B: 250, A: 255} // #89b4fa أزرق
	ChartFillColor  = color.RGBA{R: 137, G: 180, B: 250, A: 40}  // #89b4fa شفاف
	ChartLabelColor = color.RGBA{R: 166, G: 173, B: 200, A: 255} // #a6adc8
)

// CSVRecord سجل واحد من ملف CSV
type CSVRecord struct {
	Time  time.Time
	Value float64
}

// ReadCSVValues يقرأ ملف CSV ويُرجع []float64 مرتبة زمنياً.
// صيغة الملف: "2006-01-02 15:04:05","110.50"
// يُرجع nil إذا كان الملف غير موجود.
func ReadCSVValues(path string) []float64 {
	const maxPoints = 288

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return nil
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	var records []CSVRecord
	firstRow := true

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(row) < 2 {
			continue
		}
		if firstRow {
			firstRow = false
			tsStr := strings.ToLower(strings.TrimSpace(row[0]))
			if tsStr == "time" || tsStr == "timestamp" || tsStr == "date" {
				continue
			}
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04:05",
			strings.TrimSpace(row[0]), time.Local)
		if err != nil {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err != nil || val < 0 {
			continue
		}
		records = append(records, CSVRecord{Time: ts, Value: val})
	}

	if len(records) == 0 {
		return nil
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})
	if len(records) > maxPoints {
		records = records[len(records)-maxPoints:]
	}

	values := make([]float64, len(records))
	for i, rec := range records {
		values[i] = rec.Value
	}
	return values
}

// BuildGraphWidget يُنشئ *fynesimplechart.ScatterPlot بالألوان الاحترافية.
// data: مصفوفة القيم (TH/s) - المحور X = الفهرس، المحور Y = القيمة
func BuildGraphWidget(data []float64, seriesTitle string) *fynesimplechart.ScatterPlot {
	if len(data) == 0 {
		return nil
	}

	nodes := make([]fynesimplechart.Node, len(data))
	for i, v := range data {
		nodes[i] = *fynesimplechart.NewNode(float32(i), float32(v))
	}

	plot := fynesimplechart.NewPlot(nodes, seriesTitle)
	plot.ShowLine = true
	plot.LineWidth = 2.5
	plot.PlotColor = ChartLineColor
	plot.ShowPoints = false
	plot.FillArea = true
	plot.FillToZero = true
	plot.FillColor = ChartFillColor

	gw := fynesimplechart.NewGraphWidget([]fynesimplechart.Plot{*plot})
	gw.ShowGrid = true
	gw.ShowLegend = false
	gw.YAxisTitle = "TH/s"

	return gw
}

// BuildGraphWidgetFromCSV اختصار يقرأ CSV ويُنشئ الرسم البياني.
func BuildGraphWidgetFromCSV(csvPath, seriesTitle string) *fynesimplechart.ScatterPlot {
	data := ReadCSVValues(csvPath)
	if len(data) == 0 {
		return nil
	}
	return BuildGraphWidget(data, seriesTitle)
}
