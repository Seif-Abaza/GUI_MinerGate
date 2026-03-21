// Package charts handles hashrate history persistence (CSV) and interactive
// chart generation using github.com/go-echarts/go-echarts/v2.
package charts

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gecharts "github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	etypes "github.com/go-echarts/go-echarts/v2/types"

	"minergate/internal/models"
)

// ─────────────────────────────────────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	// MaxPoints is the maximum number of CSV rows kept per file (24 h @ 5 min).
	MaxPoints = 288
	// CSVTimeLayout is the timestamp format used in CSV files.
	CSVTimeLayout = "2006-01-02 15:04:05"
)

// ─────────────────────────────────────────────────────────────────────────────
// CSV I/O
// ─────────────────────────────────────────────────────────────────────────────

// Append adds one row to the CSV file at path.
// Columns: timestamp, total, unit [, chain0, chain1, …]
func Append(path string, total float64, unit string, chains []float64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	row := []string{
		time.Now().Format(CSVTimeLayout),
		strconv.FormatFloat(total, 'f', 3, 64),
		unit,
	}
	for _, c := range chains {
		row = append(row, strconv.FormatFloat(c, 'f', 3, 64))
	}

	w := csv.NewWriter(f)
	if err := w.Write(row); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

// ReadAll reads the CSV at path and returns at most MaxPoints data points
// sorted by ascending timestamp.
//
// Accepted column order: timestamp, total [, unit [, chain0, chain1, …]]
// GH/s values are automatically converted to TH/s.
func ReadAll(path string) ([]models.HashRatePoint, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1 // variable number of columns

	var pts []models.HashRatePoint
	skippedHeader := false

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}
		if len(row) < 2 {
			continue
		}

		// Skip header row (first row with non-numeric first column)
		if !skippedHeader {
			skippedHeader = true
			if strings.EqualFold(strings.TrimSpace(row[0]), "timestamp") ||
				strings.EqualFold(strings.TrimSpace(row[0]), "time") {
				continue
			}
		}

		ts := parseTimestamp(strings.TrimSpace(row[0]))
		if ts.IsZero() {
			continue
		}

		total, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err != nil {
			continue
		}

		unit := "TH/s"
		if len(row) >= 3 && strings.TrimSpace(row[2]) != "" {
			unit = strings.TrimSpace(row[2])
		}

		// Normalise GH/s → TH/s
		if isGH(unit) {
			total /= 1000
			unit = "TH/s"
		}

		pt := models.HashRatePoint{
			Timestamp: ts,
			Total:     total,
			Unit:      unit,
		}

		// Optional per-chain columns (starting at index 3)
		for i := 3; i < len(row); i++ {
			v, err := strconv.ParseFloat(strings.TrimSpace(row[i]), 64)
			if err != nil {
				continue
			}
			if isGH(unit) {
				v /= 1000
			}
			pt.Chains = append(pt.Chains, v)
		}

		pts = append(pts, pt)
	}

	sort.Slice(pts, func(i, j int) bool {
		return pts[i].Timestamp.Before(pts[j].Timestamp)
	})

	if len(pts) > MaxPoints {
		pts = pts[len(pts)-MaxPoints:]
	}
	return pts, nil
}

// FilterByDuration returns only the points within the last d.
// If d == 0 all points are returned.
func FilterByDuration(pts []models.HashRatePoint, d time.Duration) []models.HashRatePoint {
	if d == 0 || len(pts) == 0 {
		return pts
	}
	cutoff := time.Now().Add(-d)
	var out []models.HashRatePoint
	for _, p := range pts {
		if p.Timestamp.After(cutoff) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return pts // fallback: never return completely empty slice
	}
	return out
}

func parseTimestamp(s string) time.Time {
	for _, layout := range []string{
		CSVTimeLayout,
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0)
	}
	return time.Time{}
}

func isGH(unit string) bool {
	u := strings.ToUpper(unit)
	return u == "GH/S" || u == "GH"
}

// ─────────────────────────────────────────────────────────────────────────────
// Statistics helpers
// ─────────────────────────────────────────────────────────────────────────────

// Stats holds simple descriptive statistics for a set of points.
type Stats struct {
	Current float64
	Average float64
	Min     float64
	Max     float64
	Unit    string
	Count   int
}

// Compute returns Stats for the given slice.  Returns zero-value if empty.
func Compute(pts []models.HashRatePoint) Stats {
	if len(pts) == 0 {
		return Stats{}
	}
	s := Stats{
		Current: pts[len(pts)-1].Total,
		Min:     pts[0].Total,
		Max:     pts[0].Total,
		Unit:    pts[0].Unit,
		Count:   len(pts),
	}
	if s.Unit == "" {
		s.Unit = "TH/s"
	}
	var sum float64
	for _, p := range pts {
		sum += p.Total
		if p.Total < s.Min {
			s.Min = p.Total
		}
		if p.Total > s.Max {
			s.Max = p.Total
		}
	}
	s.Average = sum / float64(len(pts))
	return s
}

// ─────────────────────────────────────────────────────────────────────────────
// go-echarts HTML generation
// ─────────────────────────────────────────────────────────────────────────────

// GenerateHTML produces a self-contained go-echarts HTML chart from pts.
// title is shown at the top of the chart.
func GenerateHTML(title string, pts []models.HashRatePoint) ([]byte, error) {
	if len(pts) == 0 {
		return noDataHTML(title), nil
	}

	line := gecharts.NewLine()

	// ── Global options ──────────────────────────────────────────────────────
	unit := pts[0].Unit
	if unit == "" {
		unit = "TH/s"
	}

	line.SetGlobalOptions(
		gecharts.WithInitializationOpts(opts.Initialization{
			Theme:           etypes.ThemeDark,
			Width:           "100%",
			Height:          "500px",
			BackgroundColor: "#1e1e2e",
		}),
		gecharts.WithTitleOpts(opts.Title{
			Title: title,
			Subtitle: fmt.Sprintf("%d samples  ·  last updated %s",
				len(pts), time.Now().Format("15:04:05")),
			TitleStyle:    &opts.TextStyle{Color: "#e0e0e0", FontSize: 16},
			SubtitleStyle: &opts.TextStyle{Color: "#888", FontSize: 12},
		}),
		gecharts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
			AxisPointer: &opts.AxisPointer{
				Type: "cross",
			},
		}),
		gecharts.WithLegendOpts(opts.Legend{
			Show:      opts.Bool(true),
			Bottom:    "2%",
			TextStyle: &opts.TextStyle{Color: "#ccc"},
		}),
		gecharts.WithGridOpts(opts.Grid{
			Left:         "4%",
			Right:        "3%",
			Top:          "15%",
			Bottom:       "15%",
			ContainLabel: opts.Bool(true),
		}),
		gecharts.WithXAxisOpts(opts.XAxis{
			Type:        "category",
			BoundaryGap: opts.Bool(false),
			AxisLine:    &opts.AxisLine{LineStyle: &opts.LineStyle{Color: "#555"}},
			AxisLabel:   &opts.AxisLabel{Color: "#888", Rotate: 30, Show: opts.Bool(true)},
			SplitLine:   &opts.SplitLine{Show: opts.Bool(false)},
		}),
		gecharts.WithYAxisOpts(opts.YAxis{
			Type: "value",
			Name: fmt.Sprintf("Hashrate (%s)", unit),
			NameTextStyle: &opts.TextStyle{
				Color:    "#aaa",
				FontSize: 12,
				Padding:  []int{0, 0, 0, 40},
			},
			AxisLabel: &opts.AxisLabel{
				Color:     "#888",
				Show:      opts.Bool(true),
				Formatter: fmt.Sprintf("{value} %s", unit),
			},
			SplitLine: &opts.SplitLine{
				Show:      opts.Bool(true),
				LineStyle: &opts.LineStyle{Color: "#333", Type: "dashed"},
			},
		}),
		gecharts.WithDataZoomOpts(
			opts.DataZoom{
				Type:       "inside",
				Start:      0,
				End:        100,
				FilterMode: "none",
			},
			opts.DataZoom{
				Type:            "slider",
				Start:           0,
				End:             100,
				FilterMode:      "none",
				Height:          "6%",
				Bottom:          "1%",
				BorderColor:     "#444",
				BackgroundColor: "#1a1a2e",
				FillerColor:     "rgba(70,130,255,0.25)",
			},
		),
		gecharts.WithToolboxOpts(opts.Toolbox{
			Show:  opts.Bool(true),
			Right: "3%",
			Top:   "2%",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show:  opts.Bool(true),
					Title: "Save PNG",
				},
				Restore: &opts.ToolBoxFeatureRestore{
					Show:  opts.Bool(true),
					Title: "Reset zoom",
				},
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show:  opts.Bool(true),
					Title: map[string]string{"zoom": "Zoom", "back": "Back"},
				},
			},
		}),
	)

	// ── X-axis labels ────────────────────────────────────────────────────────
	xLabels := make([]string, len(pts))
	for i, p := range pts {
		xLabels[i] = p.Timestamp.Format("01/02 15:04")
	}
	line.SetXAxis(xLabels)

	// ── Total hashrate series ────────────────────────────────────────────────
	totalData := make([]opts.LineData, len(pts))
	for i, p := range pts {
		totalData[i] = opts.LineData{Value: round2(p.Total)}
	}

	line.AddSeries(fmt.Sprintf("Total (%s)", unit), totalData,
		gecharts.WithLineChartOpts(opts.LineChart{
			Smooth:     opts.Bool(true),
			Symbol:     "none",
			ShowSymbol: opts.Bool(false),
		}),
		gecharts.WithAreaStyleOpts(opts.AreaStyle{
			Opacity: 0.25,
			Color: "rgba(70,130,255,0.6)",
		}),
		gecharts.WithItemStyleOpts(opts.ItemStyle{Color: "#4682ff"}),
		gecharts.WithLineStyleOpts(opts.LineStyle{Width: 2.5, Color: "#4682ff"}),
	)

	// ── Per-chain series ─────────────────────────────────────────────────────
	chainColors := []string{"#2daf32", "#ff9900", "#ff6464", "#aa44ff", "#00cccc"}
	chainCount := 0
	if len(pts) > 0 {
		chainCount = len(pts[0].Chains)
	}
	for c := 0; c < chainCount; c++ {
		cd := make([]opts.LineData, len(pts))
		for i, p := range pts {
			if c < len(p.Chains) {
				cd[i] = opts.LineData{Value: round2(p.Chains[c])}
			}
		}
		col := chainColors[c%len(chainColors)]
		line.AddSeries(
			fmt.Sprintf("Chain %d", c+1),
			cd,
			gecharts.WithLineChartOpts(opts.LineChart{
				Smooth:     opts.Bool(true),
				Symbol:     "none",
				ShowSymbol: opts.Bool(false),
			}),
			gecharts.WithItemStyleOpts(opts.ItemStyle{Color: col}),
			gecharts.WithLineStyleOpts(opts.LineStyle{Width: 1.5, Color: col}),
		)
	}

	var buf bytes.Buffer
	if err := line.Render(&buf); err != nil {
		return nil, fmt.Errorf("go-echarts render: %w", err)
	}
	return buf.Bytes(), nil
}

func noDataHTML(title string) []byte {
	return []byte(fmt.Sprintf(`<!DOCTYPE html>
<html><body style="margin:0;background:#1e1e2e;display:flex;
align-items:center;justify-content:center;height:100vh;
font-family:sans-serif;color:#666;flex-direction:column;">
<h2 style="color:#888">%s</h2>
<p>No hashrate data available yet.<br>
Data is collected automatically once the miner is polled.</p>
</body></html>`, title))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// ─────────────────────────────────────────────────────────────────────────────
// ChartServer – local HTTP server serving the go-echarts HTML
// ─────────────────────────────────────────────────────────────────────────────

// ChartServer runs a minimal HTTP server and serves a go-echarts HTML page.
// It picks a random free port on 127.0.0.1 at startup.
type ChartServer struct {
	mu     sync.RWMutex
	port   int
	server *http.Server
	html   []byte
}

// NewChartServer starts the HTTP server and returns the ChartServer.
// Call Stop() when the application exits.
func NewChartServer() (*ChartServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("chart server listen: %w", err)
	}

	cs := &ChartServer{
		port: ln.Addr().(*net.TCPAddr).Port,
		html: noDataHTML("Hashrate Chart"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/chart", cs.serveChart)

	cs.server = &http.Server{Handler: mux}
	go cs.server.Serve(ln) //nolint:errcheck

	return cs, nil
}

func (cs *ChartServer) serveChart(w http.ResponseWriter, _ *http.Request) {
	cs.mu.RLock()
	h := cs.html
	cs.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(h) //nolint:errcheck
}

// Update re-renders the chart from pts and replaces the served HTML atomically.
func (cs *ChartServer) Update(title string, pts []models.HashRatePoint) error {
	html, err := GenerateHTML(title, pts)
	if err != nil {
		return err
	}
	cs.mu.Lock()
	cs.html = html
	cs.mu.Unlock()
	return nil
}

// URL returns the full chart URL.
func (cs *ChartServer) URL() string {
	return fmt.Sprintf("http://localhost:%d/chart", cs.port)
}

// Stop shuts the HTTP server down.
func (cs *ChartServer) Stop() {
	if cs.server != nil {
		cs.server.Close() //nolint:errcheck
	}
}
