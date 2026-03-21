// Package gui implements the MinerGate graphical user interface.
//
// Key design points
//   - ChartWidget embeds a go-echarts interactive chart served from an
//     in-process HTTP server (charts.ChartServer).  Clicking "Open Chart"
//     launches the system browser pointing at that local URL.
//   - The inline sparkline is drawn with native Fyne canvas primitives so it
//     is visible immediately without a browser.
//   - Dashboard polls every miner target concurrently and appends the total
//     hashrate to a per-miner CSV via charts.Append().
package gui

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"minergate/internal/api"
	"minergate/internal/charts"
	"minergate/internal/config"
	"minergate/internal/models"
)

// ─────────────────────────────────────────────────────────────────────────────
// Palette
// ─────────────────────────────────────────────────────────────────────────────

var (
	colGreen    = color.RGBA{R: 0x2d, G: 0xaf, B: 0x32, A: 0xff}
	colBlue     = color.RGBA{R: 0x46, G: 0x82, B: 0xff, A: 0xff}
	colOrange   = color.RGBA{R: 0xff, G: 0x99, B: 0x00, A: 0xff}
	colRed      = color.RGBA{R: 0xff, G: 0x64, B: 0x64, A: 0xff}
	colDimBg    = color.RGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff}
	colGridLine = color.RGBA{R: 0x44, G: 0x44, B: 0x60, A: 0x90}
)

func statusColor(s string) color.Color {
	switch s {
	case "Online":
		return colGreen
	case "Warning":
		return colOrange
	default:
		return colRed
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Sparkline widget (Fyne-native mini chart, no browser required)
// ─────────────────────────────────────────────────────────────────────────────

// sparkWidget is a tiny inline chart drawn with Fyne canvas.Line objects.
type sparkWidget struct {
	widget.BaseWidget
	mu    sync.RWMutex
	pts   []models.HashRatePoint
	size  fyne.Size
}

func newSparkWidget() *sparkWidget {
	s := &sparkWidget{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *sparkWidget) SetData(pts []models.HashRatePoint) {
	s.mu.Lock()
	s.pts = pts
	s.mu.Unlock()
	s.Refresh()
}

func (s *sparkWidget) MinSize() fyne.Size { return fyne.NewSize(300, 100) }

func (s *sparkWidget) CreateRenderer() fyne.WidgetRenderer {
	return &sparkRenderer{w: s}
}

// sparkRenderer does the actual drawing.
type sparkRenderer struct {
	w       *sparkWidget
	bg      *canvas.Rectangle
	gridH   []*canvas.Line
	lines   []*canvas.Line
	areaRects []*canvas.Rectangle
	noData  *canvas.Text
}

func (r *sparkRenderer) MinSize() fyne.Size { return r.w.MinSize() }

func (r *sparkRenderer) Layout(size fyne.Size) {
	r.w.size = size
	r.rebuild(size)
}

func (r *sparkRenderer) Refresh() {
	r.rebuild(r.w.size)
	canvas.Refresh(r.w)
}

func (r *sparkRenderer) rebuild(size fyne.Size) {
	r.w.mu.RLock()
	pts := r.w.pts
	r.w.mu.RUnlock()

	// Background
	if r.bg == nil {
		r.bg = canvas.NewRectangle(colDimBg)
		r.bg.CornerRadius = 6
	}
	r.bg.Resize(size)
	r.bg.Move(fyne.Position{})

	// Clear old geometry
	r.gridH = r.gridH[:0]
	r.lines = r.lines[:0]
	r.areaRects = r.areaRects[:0]

	pad := float32(10)
	w := size.Width - pad*2
	h := size.Height - pad*2

	// Horizontal grid lines
	for i := 0; i <= 3; i++ {
		y := pad + float32(i)*(h/3)
		gl := canvas.NewLine(colGridLine)
		gl.StrokeWidth = 0.5
		gl.Position1 = fyne.Position{X: pad, Y: y}
		gl.Position2 = fyne.Position{X: pad + w, Y: y}
		r.gridH = append(r.gridH, gl)
	}

	// No-data text
	if r.noData == nil {
		r.noData = canvas.NewText("Waiting for data…", color.RGBA{R: 0x66, G: 0x66, B: 0x88, A: 0xff})
		r.noData.TextSize = 11
		r.noData.Alignment = fyne.TextAlignCenter
	}

	if len(pts) < 2 {
		r.noData.Hidden = false
		r.noData.Resize(size)
		r.noData.Move(fyne.Position{X: 0, Y: size.Height/2 - 8})
		return
	}
	r.noData.Hidden = true

	// Min / max
	minV, maxV := pts[0].Total, pts[0].Total
	for _, p := range pts {
		if p.Total < minV {
			minV = p.Total
		}
		if p.Total > maxV {
			maxV = p.Total
		}
	}
	rng := maxV - minV
	if rng < 0.001 {
		rng = 1
	}

	n := len(pts)
	yAt := func(v float64) float32 {
		return pad + h - float32((v-minV)/rng)*h
	}
	xAt := func(i int) float32 {
		return pad + float32(i)/float32(n-1)*w
	}

	for i := 1; i < n; i++ {
		x0, y0 := xAt(i-1), yAt(pts[i-1].Total)
		x1, y1 := xAt(i), yAt(pts[i].Total)

		// Area fill (approximated with thin rectangles)
		baseY := pad + h
		fillH := baseY - y0
		if fillH > 0 {
			rect := canvas.NewRectangle(color.RGBA{R: 0x46, G: 0x82, B: 0xff, A: 0x28})
			rect.Move(fyne.Position{X: x0, Y: y0})
			rect.Resize(fyne.NewSize(x1-x0, fillH))
			r.areaRects = append(r.areaRects, rect)
		}

		// Line segment
		seg := canvas.NewLine(colBlue)
		seg.StrokeWidth = 2
		seg.Position1 = fyne.Position{X: x0, Y: y0}
		seg.Position2 = fyne.Position{X: x1, Y: y1}
		r.lines = append(r.lines, seg)
	}
}

func (r *sparkRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg}
	for _, gl := range r.gridH {
		objs = append(objs, gl)
	}
	for _, ar := range r.areaRects {
		objs = append(objs, ar)
	}
	for _, l := range r.lines {
		objs = append(objs, l)
	}
	if r.noData != nil {
		objs = append(objs, r.noData)
	}
	return objs
}

func (r *sparkRenderer) Destroy() {}

// ─────────────────────────────────────────────────────────────────────────────
// ChartWidget  – the main chart panel shown in the Overview tab
// ─────────────────────────────────────────────────────────────────────────────

// rangeOption pairs a human label with a duration filter.
type rangeOption struct {
	label    string
	duration time.Duration
}

var rangeOptions = []rangeOption{
	{"1 h", time.Hour},
	{"6 h", 6 * time.Hour},
	{"24 h", 24 * time.Hour},
	{"7 d", 7 * 24 * time.Hour},
	{"All", 0},
}

// ChartWidget bundles:
//   - go-echarts interactive chart (served via ChartServer, opened in browser)
//   - Inline Fyne sparkline (always visible)
//   - Time-range selector
//   - Stat labels (current / avg / min / max)
type ChartWidget struct {
	widget.BaseWidget

	// Dependencies
	csvPath   string
	title     string
	window    fyne.Window
	server    *charts.ChartServer

	// State
	mu         sync.RWMutex
	allPts     []models.HashRatePoint
	filteredPts []models.HashRatePoint
	activeDur  time.Duration // 0 = all

	// Fyne sub-widgets
	spark      *sparkWidget
	statCurrent *widget.Label
	statAvg    *widget.Label
	statMin    *widget.Label
	statMax    *widget.Label
	rangebtns  []*widget.Button
	openBtn    *widget.Button
	refreshBtn *widget.Button

	// Auto-refresh
	stopCh chan struct{}
}

// NewChartWidget creates the widget, starts the chart HTTP server, loads CSV
// and begins auto-refreshing every 60 s.
func NewChartWidget(csvPath, title string, win fyne.Window) *ChartWidget {
	cw := &ChartWidget{
		csvPath:    csvPath,
		title:      title,
		window:     win,
		activeDur:  24 * time.Hour,
		stopCh:     make(chan struct{}),
	}

	// go-echarts HTTP server
	srv, err := charts.NewChartServer()
	if err == nil {
		cw.server = srv
	}

	// Sub-widgets
	cw.spark = newSparkWidget()
	cw.statCurrent = monoLabel("Current  —")
	cw.statAvg = monoLabel("Avg      —")
	cw.statMin = monoLabel("Min      —")
	cw.statMax = monoLabel("Max      —")

	cw.openBtn = widget.NewButton("📊  Open Interactive Chart", cw.openInBrowser)
	cw.openBtn.Importance = widget.HighImportance

	cw.refreshBtn = widget.NewButton("↺", func() { cw.Reload() })
	cw.refreshBtn.Importance = widget.LowImportance

	for i, ro := range rangeOptions {
		ro := ro
		i := i
		btn := widget.NewButton(ro.label, func() { cw.selectRange(i, ro.duration) })
		if ro.duration == cw.activeDur {
			btn.Importance = widget.HighImportance
		}
		cw.rangebtns = append(cw.rangebtns, btn)
	}

	cw.ExtendBaseWidget(cw)
	cw.Reload()
	go cw.autoRefreshLoop()
	return cw
}

// ── Fyne Widget interface ────────────────────────────────────────────────────

func (cw *ChartWidget) CreateRenderer() fyne.WidgetRenderer {
	rangeBar := container.NewHBox(layout.NewSpacer())
	for _, b := range cw.rangebtns {
		rangeBar.Add(b)
	}
	rangeBar.Add(layout.NewSpacer())
	rangeBar.Add(cw.refreshBtn)

	statsGrid := container.NewGridWithColumns(2,
		cw.statCurrent,
		cw.statAvg,
		cw.statMin,
		cw.statMax,
	)

	titleLabel := widget.NewLabel(cw.title)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	content := container.NewVBox(
		container.NewHBox(titleLabel, layout.NewSpacer()),
		widget.NewSeparator(),
		rangeBar,
		cw.spark,
		widget.NewSeparator(),
		statsGrid,
		cw.openBtn,
	)

	return widget.NewSimpleRenderer(content)
}

// ── Data management ──────────────────────────────────────────────────────────

// Reload reads the CSV, updates the sparkline and the go-echarts server.
func (cw *ChartWidget) Reload() {
	pts, _ := charts.ReadAll(cw.csvPath)

	cw.mu.Lock()
	cw.allPts = pts
	cw.mu.Unlock()

	cw.applyFilter()
}

func (cw *ChartWidget) selectRange(idx int, d time.Duration) {
	cw.mu.Lock()
	cw.activeDur = d
	cw.mu.Unlock()

	for i, b := range cw.rangebtns {
		if i == idx {
			b.Importance = widget.HighImportance
		} else {
			b.Importance = widget.MediumImportance
		}
		b.Refresh()
	}
	cw.applyFilter()
}

func (cw *ChartWidget) applyFilter() {
	cw.mu.Lock()
	dur := cw.activeDur
	all := cw.allPts
	cw.mu.Unlock()

	filtered := charts.FilterByDuration(all, dur)

	cw.mu.Lock()
	cw.filteredPts = filtered
	cw.mu.Unlock()

	// Update sparkline
	cw.spark.SetData(filtered)

	// Update stat labels
	st := charts.Compute(filtered)
	if st.Count == 0 {
		cw.statCurrent.SetText("Current  —")
		cw.statAvg.SetText("Avg      —")
		cw.statMin.SetText("Min      —")
		cw.statMax.SetText("Max      —")
	} else {
		u := st.Unit
		cw.statCurrent.SetText(fmt.Sprintf("Current  %.2f %s", st.Current, u))
		cw.statAvg.SetText(fmt.Sprintf("Avg      %.2f %s", st.Average, u))
		cw.statMin.SetText(fmt.Sprintf("Min      %.2f %s", st.Min, u))
		cw.statMax.SetText(fmt.Sprintf("Max      %.2f %s", st.Max, u))
	}

	// Rebuild go-echarts HTML
	if cw.server != nil {
		chartTitle := cw.title
		if dur > 0 {
			chartTitle = fmt.Sprintf("%s — last %s", cw.title, fmtDur(dur))
		}
		_ = cw.server.Update(chartTitle, filtered)
	}

	cw.Refresh()
}

func (cw *ChartWidget) autoRefreshLoop() {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			cw.Reload()
		case <-cw.stopCh:
			return
		}
	}
}

// Stop shuts down background goroutine + HTTP server.
func (cw *ChartWidget) Stop() {
	select {
	case <-cw.stopCh:
	default:
		close(cw.stopCh)
	}
	if cw.server != nil {
		cw.server.Stop()
	}
}

func (cw *ChartWidget) openInBrowser() {
	if cw.server == nil {
		dialog.ShowError(fmt.Errorf("chart server not running"), cw.window)
		return
	}
	if err := openURL(cw.server.URL()); err != nil {
		dialog.ShowError(fmt.Errorf("cannot open browser: %w", err), cw.window)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Dashboard
// ─────────────────────────────────────────────────────────────────────────────

// Dashboard is the root GUI controller.
type Dashboard struct {
	app    fyne.App
	window fyne.Window
	cfg    *config.Config

	// Mining data – keyed by target index
	mu      sync.RWMutex
	devices []*models.MinerDevice

	// UI – chart
	chartWidget *ChartWidget

	// UI – overview
	overviewCards *fyne.Container // holds stat cards for the summary row

	// UI – miner list
	minerList   *widget.List
	minerDetail *fyne.Container

	// UI – pools table
	poolsTable *widget.Table
	poolData   []models.Pool

	// UI – warnings
	warnList  *widget.List
	warnData  []models.Warning

	// UI – status bar
	statusBar  *widget.Label
	lastUpdate *widget.Label

	// Polling
	stopCh  chan struct{}
	ticker  *time.Ticker
}

// NewDashboard builds the full Fyne UI and starts background polling.
func NewDashboard(app fyne.App, win fyne.Window, cfg *config.Config) *Dashboard {
	d := &Dashboard{
		app:    app,
		window: win,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}

	_ = cfg.EnsureDataDir()

	// Chart widget uses total hashrate CSV
	csvPath := cfg.CSVPath("total")
	d.chartWidget = NewChartWidget(csvPath, "Total Hashrate History", win)

	d.buildUI()

	if cfg.AutoRefresh {
		d.startPolling()
	}

	// First poll immediately
	go d.pollAll()
	return d
}

// ─────────────────────────────────────────────────────────────────────────────
// UI construction
// ─────────────────────────────────────────────────────────────────────────────

func (d *Dashboard) buildUI() {
	d.window.SetTitle("MinerGate Dashboard v1.0.3")
	d.window.Resize(fyne.NewSize(1300, 840))

	tabs := container.NewAppTabs(
		container.NewTabItem("📊 Overview", d.buildOverviewTab()),
		container.NewTabItem("⛏ Miners", d.buildMinersTab()),
		container.NewTabItem("🏊 Pools", d.buildPoolsTab()),
		container.NewTabItem("⚠ Warnings", d.buildWarningsTab()),
		container.NewTabItem("⚙ Settings", d.buildSettingsTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	d.statusBar = widget.NewLabel("Ready")
	d.lastUpdate = widget.NewLabel("")
	d.lastUpdate.TextStyle = fyne.TextStyle{Italic: true}

	footer := container.NewBorder(nil, nil,
		d.statusBar, d.lastUpdate,
	)

	d.window.SetContent(
		container.NewBorder(nil, footer, nil, nil, tabs),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Overview tab
// ─────────────────────────────────────────────────────────────────────────────

func (d *Dashboard) buildOverviewTab() fyne.CanvasObject {
	// Summary cards row
	d.overviewCards = container.NewGridWithColumns(4,
		d.makeSummaryCard("⚡ Hashrate", "—", colBlue),
		d.makeSummaryCard("🌡 Temperature", "—", colOrange),
		d.makeSummaryCard("💨 Fan Speed", "—", colGreen),
		d.makeSummaryCard("⚙ Power", "—", colRed),
	)

	chartCard := widget.NewCard("", "", d.chartWidget)

	return container.NewVBox(
		d.overviewCards,
		chartCard,
	)
}

// makeSummaryCard creates one of the four overview stat cards.
func (d *Dashboard) makeSummaryCard(title, value string, col color.Color) fyne.CanvasObject {
	dot := canvas.NewCircle(col)
	dot.SetMinSize(fyne.NewSize(10, 10))

	titleLbl := widget.NewLabel(title)
	titleLbl.TextStyle = fyne.TextStyle{Bold: true}

	valLbl := widget.NewLabel(value)
	valLbl.TextStyle = fyne.TextStyle{Monospace: true}
	valLbl.Alignment = fyne.TextAlignCenter

	return widget.NewCard("", "",
		container.NewVBox(
			container.NewHBox(dot, titleLbl),
			valLbl,
		),
	)
}

// updateOverviewCards refreshes the four summary cards from current device data.
func (d *Dashboard) updateOverviewCards() {
	d.mu.RLock()
	devs := d.devices
	d.mu.RUnlock()

	if len(devs) == 0 || d.overviewCards == nil {
		return
	}

	var totalHash, totalPower float64
	var sumTemp, sumFan float64
	n := 0

	for _, dev := range devs {
		if !dev.Online || dev.Summary == nil {
			continue
		}
		s := dev.Summary
		totalHash += dev.TotalHashrate()
		totalPower += s.Power
		sumTemp += s.Temperature
		fans := []int{s.Fan1, s.Fan2, s.Fan3, s.Fan4}
		var fanSum float64
		cnt := 0
		for _, f := range fans {
			if f > 0 {
				fanSum += float64(f)
				cnt++
			}
		}
		if cnt > 0 {
			sumFan += fanSum / float64(cnt)
		}
		n++
	}

	unit := "TH/s"
	hashStr := fmt.Sprintf("%.2f %s", totalHash, unit)
	tempStr := "—"
	fanStr := "—"
	powerStr := "—"

	if n > 0 {
		tempStr = fmt.Sprintf("%.0f °C", sumTemp/float64(n))
		fanStr = fmt.Sprintf("%.0f RPM", sumFan/float64(n))
	}
	if totalPower > 0 {
		powerStr = fmt.Sprintf("%.0f W", totalPower)
	}

	vals := []string{hashStr, tempStr, fanStr, powerStr}
	for i, card := range d.overviewCards.Objects {
		if c, ok := card.(*widget.Card); ok {
			if vbox, ok := c.Content.(*fyne.Container); ok {
				if len(vbox.Objects) >= 2 {
					if lbl, ok := vbox.Objects[1].(*widget.Label); ok {
						lbl.SetText(vals[i])
					}
				}
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Miners tab
// ─────────────────────────────────────────────────────────────────────────────

func (d *Dashboard) buildMinersTab() fyne.CanvasObject {
	d.minerDetail = container.NewVBox(
		widget.NewLabel("Select a miner to see details."),
	)

	d.minerList = widget.NewList(
		func() int {
			d.mu.RLock()
			defer d.mu.RUnlock()
			return len(d.devices)
		},
		func() fyne.CanvasObject {
			return d.buildMinerListTemplate()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			d.mu.RLock()
			if id >= len(d.devices) {
				d.mu.RUnlock()
				return
			}
			dev := d.devices[id]
			d.mu.RUnlock()
			d.fillMinerListItem(obj, dev)
		},
	)
	d.minerList.OnSelected = func(id widget.ListItemID) {
		d.mu.RLock()
		if id >= len(d.devices) {
			d.mu.RUnlock()
			return
		}
		dev := d.devices[id]
		d.mu.RUnlock()
		d.showMinerDetail(dev)
	}

	toolbar := container.NewHBox(
		widget.NewButton("↺ Refresh All", func() {
			go d.pollAll()
		}),
		widget.NewButton("+ Add Miner", func() { d.showAddMinerDialog() }),
		layout.NewSpacer(),
	)

	split := container.NewHSplit(d.minerList, d.minerDetail)
	split.SetOffset(0.38)

	return container.NewBorder(toolbar, nil, nil, nil, split)
}

// ── Miner list item template ─────────────────────────────────────────────────

func (d *Dashboard) buildMinerListTemplate() fyne.CanvasObject {
	dot := canvas.NewCircle(colGreen)
	dot.SetMinSize(fyne.NewSize(12, 12))

	host := widget.NewLabel("host")
	host.TextStyle = fyne.TextStyle{Bold: true}

	minerType := widget.NewLabel("type")
	minerType.TextStyle = fyne.TextStyle{Italic: true}

	hashLbl := widget.NewLabel("hash")
	hashLbl.TextStyle = fyne.TextStyle{Monospace: true}

	tempLbl := widget.NewLabel("temp")
	fanLbl := widget.NewLabel("fan")

	return container.NewHBox(
		dot,
		container.NewVBox(
			container.NewHBox(host, widget.NewLabel("  "), minerType),
			container.NewHBox(hashLbl, widget.NewLabel("  "), tempLbl, widget.NewLabel("  "), fanLbl),
		),
	)
}

func (d *Dashboard) fillMinerListItem(obj fyne.CanvasObject, dev *models.MinerDevice) {
	box, ok := obj.(*fyne.Container)
	if !ok || len(box.Objects) < 2 {
		return
	}
	dot, _ := box.Objects[0].(*canvas.Circle)
	info, _ := box.Objects[1].(*fyne.Container)
	if dot == nil || info == nil || len(info.Objects) < 2 {
		return
	}

	dot.FillColor = statusColor(dev.StatusLabel())
	dot.Refresh()

	row0, _ := info.Objects[0].(*fyne.Container)
	row1, _ := info.Objects[1].(*fyne.Container)

	if row0 != nil && len(row0.Objects) >= 3 {
		row0.Objects[0].(*widget.Label).SetText(dev.Host)
		row0.Objects[2].(*widget.Label).SetText(dev.MinerType)
	}
	if row1 != nil && len(row1.Objects) >= 5 {
		if dev.Summary != nil {
			row1.Objects[0].(*widget.Label).SetText(
				fmt.Sprintf("%.2f %s", dev.TotalHashrate(), dev.Summary.RateUnit))
			row1.Objects[2].(*widget.Label).SetText(fmt.Sprintf("%.0f°C", dev.Summary.Temperature))
			row1.Objects[4].(*widget.Label).SetText(fmt.Sprintf("%d RPM", maxFan(dev.Summary)))
		} else {
			row1.Objects[0].(*widget.Label).SetText("—")
			row1.Objects[2].(*widget.Label).SetText("—")
			row1.Objects[4].(*widget.Label).SetText("—")
		}
	}
}

// ── Miner detail panel ───────────────────────────────────────────────────────

func (d *Dashboard) showMinerDetail(dev *models.MinerDevice) {
	if d.minerDetail == nil {
		return
	}
	d.minerDetail.Objects = nil

	addRow := func(k, v string) {
		d.minerDetail.Add(container.NewGridWithColumns(2,
			boldLabel(k+":"),
			widget.NewLabel(v),
		))
	}

	d.minerDetail.Add(boldLabel("📌 " + dev.Host))
	d.minerDetail.Add(widget.NewSeparator())

	addRow("Status", dev.StatusLabel())
	addRow("Type", dev.MinerType)
	addRow("Serial", dev.Serial)
	addRow("Firmware", dev.Firmware)

	if dev.Summary != nil {
		s := dev.Summary
		d.minerDetail.Add(widget.NewSeparator())
		d.minerDetail.Add(boldLabel("⚡ Hashrate"))
		addRow("Current", fmt.Sprintf("%.2f %s", s.Rate5s, s.RateUnit))
		addRow("Average", fmt.Sprintf("%.2f %s", s.RateAvg, s.RateUnit))
		addRow("Reject Ratio", s.RejectRatio)

		d.minerDetail.Add(widget.NewSeparator())
		d.minerDetail.Add(boldLabel("🌡 Environment"))
		addRow("Temperature", fmt.Sprintf("%.0f °C", s.Temperature))
		addRow("Fan 1", fmtRPM(s.Fan1))
		addRow("Fan 2", fmtRPM(s.Fan2))
		addRow("Fan 3", fmtRPM(s.Fan3))
		addRow("Fan 4", fmtRPM(s.Fan4))
		addRow("Power", fmt.Sprintf("%.0f W", s.Power))

		d.minerDetail.Add(widget.NewSeparator())
		d.minerDetail.Add(boldLabel("📊 Shares"))
		addRow("Accepted", fmt.Sprintf("%d", s.Accepted))
		addRow("Rejected", fmt.Sprintf("%d", s.Rejected))
		addRow("HW Errors", fmt.Sprintf("%d", s.HWErrors))
		addRow("Uptime", fmtUptime(s.Elapsed))
	}

	if dev.ErrorMsg != "" {
		d.minerDetail.Add(widget.NewSeparator())
		errLbl := widget.NewLabel("⚠ Error: " + dev.ErrorMsg)
		errLbl.Wrapping = fyne.TextWrapWord
		d.minerDetail.Add(errLbl)
	}

	d.minerDetail.Refresh()
}

// ─────────────────────────────────────────────────────────────────────────────
// Pools tab
// ─────────────────────────────────────────────────────────────────────────────

func (d *Dashboard) buildPoolsTab() fyne.CanvasObject {
	cols := []string{"#", "URL", "User", "Status", "Priority", "Accepted", "Rejected", "Diff A", "Diff R"}

	d.poolsTable = widget.NewTable(
		func() (int, int) {
			d.mu.RLock()
			defer d.mu.RUnlock()
			rows := len(d.poolData) + 1 // +1 for header
			if rows < 2 {
				rows = 2
			}
			return rows, len(cols)
		},
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return lbl
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			lbl := obj.(*widget.Label)
			if id.Row == 0 {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.SetText(cols[id.Col])
				return
			}
			lbl.TextStyle = fyne.TextStyle{}
			d.mu.RLock()
			idx := id.Row - 1
			if idx >= len(d.poolData) {
				d.mu.RUnlock()
				if idx == 0 {
					lbl.SetText("—")
				} else {
					lbl.SetText("")
				}
				return
			}
			p := d.poolData[idx]
			d.mu.RUnlock()

			switch id.Col {
			case 0:
				lbl.SetText(fmt.Sprintf("%d", p.Index+1))
			case 1:
				lbl.SetText(p.URL)
			case 2:
				lbl.SetText(p.User)
			case 3:
				lbl.SetText(p.Status)
			case 4:
				lbl.SetText(fmt.Sprintf("%d", p.Priority))
			case 5:
				lbl.SetText(fmt.Sprintf("%d", p.Accepted))
			case 6:
				lbl.SetText(fmt.Sprintf("%d", p.Rejected))
			case 7:
				lbl.SetText(p.DiffA)
			case 8:
				lbl.SetText(p.DiffR)
			}
		},
	)

	// Column widths
	widths := []float32{30, 300, 200, 70, 70, 90, 90, 90, 90}
	for i, w := range widths {
		d.poolsTable.SetColumnWidth(i, w)
	}

	return container.NewBorder(
		boldLabel("Mining Pools"),
		nil, nil, nil,
		d.poolsTable,
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Warnings tab
// ─────────────────────────────────────────────────────────────────────────────

func (d *Dashboard) buildWarningsTab() fyne.CanvasObject {
	noWarn := container.NewCenter(widget.NewLabel("✅  No active warnings"))

	d.warnList = widget.NewList(
		func() int {
			d.mu.RLock()
			defer d.mu.RUnlock()
			return len(d.warnData)
		},
		func() fyne.CanvasObject {
			return container.NewGridWithColumns(4,
				widget.NewLabel("code"),
				widget.NewLabel("cause"),
				widget.NewLabel("suggestion"),
				widget.NewLabel("time"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			d.mu.RLock()
			if id >= len(d.warnData) {
				d.mu.RUnlock()
				return
			}
			w := d.warnData[id]
			d.mu.RUnlock()
			row, _ := obj.(*fyne.Container)
			if row == nil || len(row.Objects) < 4 {
				return
			}
			row.Objects[0].(*widget.Label).SetText(w.Code)
			row.Objects[1].(*widget.Label).SetText(w.Cause)
			row.Objects[2].(*widget.Label).SetText(w.Suggestion)
			row.Objects[3].(*widget.Label).SetText(w.Timestamp)
		},
	)

	return container.NewStack(noWarn, d.warnList)
}

// ─────────────────────────────────────────────────────────────────────────────
// Settings tab
// ─────────────────────────────────────────────────────────────────────────────

func (d *Dashboard) buildSettingsTab() fyne.CanvasObject {
	refreshEntry := widget.NewEntry()
	refreshEntry.SetText(fmt.Sprintf("%d", d.cfg.RefreshRate))

	dataDirEntry := widget.NewEntry()
	dataDirEntry.SetText(d.cfg.DataDir)

	autoRefreshCheck := widget.NewCheck("Enable auto-refresh", func(v bool) {
		d.cfg.AutoRefresh = v
	})
	autoRefreshCheck.SetChecked(d.cfg.AutoRefresh)

	saveBtn := widget.NewButton("💾  Save Settings", func() {
		if r, err := parseInt(refreshEntry.Text); err == nil && r > 0 {
			d.cfg.RefreshRate = r
		}
		if dataDirEntry.Text != "" {
			d.cfg.DataDir = dataDirEntry.Text
		}
		if err := d.cfg.Save(); err != nil {
			dialog.ShowError(err, d.window)
			return
		}
		dialog.ShowInformation("Saved", "Settings saved successfully.", d.window)

		// Restart polling with new rate
		d.stopPolling()
		if d.cfg.AutoRefresh {
			d.startPolling()
		}
	})
	saveBtn.Importance = widget.HighImportance

	form := widget.NewForm(
		widget.NewFormItem("Refresh interval (s)", refreshEntry),
		widget.NewFormItem("Data directory", dataDirEntry),
	)

	// Target list
	targetRows := container.NewVBox()
	for i, t := range d.cfg.Miners {
		t := t
		i := i
		row := container.NewHBox(
			widget.NewLabel(fmt.Sprintf("%s  (%s:%d)", t.Name, t.Host, t.Port)),
			layout.NewSpacer(),
			widget.NewButton("Remove", func() {
				d.cfg.Miners = append(d.cfg.Miners[:i], d.cfg.Miners[i+1:]...)
				_ = d.cfg.Save()
				dialog.ShowInformation("Removed", "Miner removed. Restart to apply.", d.window)
			}),
		)
		targetRows.Add(row)
	}
	targetCard := widget.NewCard("Miner Targets", "", container.NewVScroll(targetRows))

	return container.NewVScroll(container.NewVBox(
		widget.NewCard("General", "", container.NewVBox(autoRefreshCheck, form)),
		targetCard,
		saveBtn,
	))
}

// showAddMinerDialog shows a dialog to add a new miner target.
func (d *Dashboard) showAddMinerDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("e.g. Farm-01")
	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("192.168.1.100")
	portEntry := widget.NewEntry()
	portEntry.SetText("80")
	apiEntry := widget.NewEntry()
	apiEntry.SetText("4028")

	form := widget.NewForm(
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Host / IP", hostEntry),
		widget.NewFormItem("HTTP Port", portEntry),
		widget.NewFormItem("API Port", apiEntry),
	)

	dialog.ShowCustomConfirm("Add Miner", "Add", "Cancel", form,
		func(ok bool) {
			if !ok || hostEntry.Text == "" {
				return
			}
			port, _ := parseInt(portEntry.Text)
			if port == 0 {
				port = 80
			}
			apiPort, _ := parseInt(apiEntry.Text)
			if apiPort == 0 {
				apiPort = 4028
			}
			d.cfg.Miners = append(d.cfg.Miners, config.MinerTarget{
				Name:    nameEntry.Text,
				Host:    hostEntry.Text,
				Port:    port,
				APIPort: apiPort,
				Enabled: true,
			})
			_ = d.cfg.Save()
			go d.pollAll()
		}, d.window)
}

// ─────────────────────────────────────────────────────────────────────────────
// Polling
// ─────────────────────────────────────────────────────────────────────────────

// pollAll fetches data from every enabled miner target concurrently.
func (d *Dashboard) pollAll() {
	d.setStatus("Polling miners…")

	targets := d.cfg.Miners

	type result struct {
		idx int
		dev *models.MinerDevice
	}

	ch := make(chan result, len(targets))

	for i, t := range targets {
		if !t.Enabled {
			continue
		}
		go func(idx int, target config.MinerTarget) {
			dev := d.pollOne(target)
			ch <- result{idx: idx, dev: dev}
		}(i, t)
	}

	newDevices := make([]*models.MinerDevice, len(targets))
	for range targets {
		r := <-ch
		newDevices[r.idx] = r.dev
	}

	// Remove nil slots (disabled targets)
	var cleaned []*models.MinerDevice
	for _, dev := range newDevices {
		if dev != nil {
			cleaned = append(cleaned, dev)
		}
	}

	d.mu.Lock()
	d.devices = cleaned
	d.mu.Unlock()

	// Aggregate pools and warnings
	d.aggregatePoolsAndWarnings(cleaned)

	// Record total hashrate to CSV and update chart
	d.recordHashrate(cleaned)

	// Refresh UI
	d.updateOverviewCards()
	if d.minerList != nil {
		d.minerList.Refresh()
	}
	if d.poolsTable != nil {
		d.poolsTable.Refresh()
	}
	if d.warnList != nil {
		d.warnList.Refresh()
	}

	d.lastUpdate.SetText("Updated: " + time.Now().Format("15:04:05"))
	d.setStatus(fmt.Sprintf("Ready — %d miner(s) polled", len(cleaned)))
}

// pollOne polls a single miner target.
func (d *Dashboard) pollOne(t config.MinerTarget) *models.MinerDevice {
	dev := &models.MinerDevice{
		ID:   fmt.Sprintf("%s:%d", t.Host, t.Port),
		Host: t.Host,
		Port: t.Port,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	client := api.New(t.Host, t.Port)
	pr := client.Poll(ctx)

	if pr.SysInfoErr == nil && pr.SysInfo != nil {
		dev.MinerType = pr.SysInfo.MinerType
		dev.Serial = pr.SysInfo.SerialNumber
		dev.Firmware = pr.SysInfo.SystemFilesystemVersion
	}

	if pr.SummaryErr == nil && pr.Summary != nil {
		dev.Summary = pr.Summary
		if dev.MinerType == "" {
			dev.MinerType = pr.Summary.MinerType
		}
		dev.Online = true
	} else {
		dev.Online = false
		if pr.SummaryErr != nil {
			dev.ErrorMsg = pr.SummaryErr.Error()
		}
	}

	dev.Stats = pr.Stats
	dev.Pools = pr.Pools
	dev.Warnings = pr.Warnings
	dev.LastSeen = time.Now()
	dev.HasWarning = len(pr.Warnings) > 0

	return dev
}

// aggregatePoolsAndWarnings collects all pools & warnings from all devices.
func (d *Dashboard) aggregatePoolsAndWarnings(devs []*models.MinerDevice) {
	var pools []models.Pool
	var warns []models.Warning
	seen := map[string]bool{}

	for _, dev := range devs {
		for _, p := range dev.Pools {
			key := fmt.Sprintf("%s|%s", p.URL, p.User)
			if !seen[key] {
				seen[key] = true
				pools = append(pools, p)
			}
		}
		warns = append(warns, dev.Warnings...)
	}

	sort.Slice(pools, func(i, j int) bool { return pools[i].Priority < pools[j].Priority })

	d.mu.Lock()
	d.poolData = pools
	d.warnData = warns
	d.mu.Unlock()
}

// recordHashrate appends the summed total hashrate to the CSV and refreshes the chart.
func (d *Dashboard) recordHashrate(devs []*models.MinerDevice) {
	var total float64
	var chains []float64
	unit := "TH/s"

	for _, dev := range devs {
		if !dev.Online || dev.Summary == nil {
			continue
		}
		total += dev.TotalHashrate()
		if dev.Summary.RateUnit != "" {
			unit = "TH/s" // always normalised to TH/s
		}

		// Per-chain data from stats
		if dev.Stats != nil && len(dev.Stats.Stats) > 0 {
			for _, ch := range dev.Stats.Stats[0].Chain {
				v := ch.RateReal
				if strings.EqualFold(ch.RateUnit, "GH/s") || strings.EqualFold(ch.RateUnit, "GH") {
					v /= 1000
				}
				chains = append(chains, v)
			}
		}
	}

	if total <= 0 {
		return
	}

	csvPath := d.cfg.CSVPath("total")
	_ = charts.Append(csvPath, total, unit, chains)
	d.chartWidget.Reload()
}

// ─────────────────────────────────────────────────────────────────────────────
// Polling lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func (d *Dashboard) startPolling() {
	interval := time.Duration(d.cfg.RefreshRate) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	d.ticker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-d.ticker.C:
				go d.pollAll()
			case <-d.stopCh:
				return
			}
		}
	}()
}

func (d *Dashboard) stopPolling() {
	if d.ticker != nil {
		d.ticker.Stop()
		d.ticker = nil
	}
}

// Stop shuts down all background work.  Call on app exit.
func (d *Dashboard) Stop() {
	select {
	case <-d.stopCh:
	default:
		close(d.stopCh)
	}
	d.stopPolling()
	d.chartWidget.Stop()
}

func (d *Dashboard) setStatus(msg string) {
	if d.statusBar != nil {
		d.statusBar.SetText(msg)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func monoLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Monospace: true}
	return l
}

func boldLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

func maxFan(s *models.Summary) int {
	m := 0
	for _, f := range []int{s.Fan1, s.Fan2, s.Fan3, s.Fan4} {
		if f > m {
			m = f
		}
	}
	return m
}

func fmtRPM(v int) string {
	if v <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d RPM", v)
}

func fmtUptime(elapsed int) string {
	d := elapsed / 86400
	h := (elapsed % 86400) / 3600
	m := ((elapsed % 86400) % 3600) / 60
	s := ((elapsed % 86400) % 3600) % 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", d, h, m, s)
	}
	return fmt.Sprintf("%dh %dm %ds", h, m, s)
}

func fmtDur(d time.Duration) string {
	switch d {
	case time.Hour:
		return "1 h"
	case 6 * time.Hour:
		return "6 h"
	case 24 * time.Hour:
		return "24 h"
	case 7 * 24 * time.Hour:
		return "7 d"
	default:
		return d.String()
	}
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	v, err := parseInt2(s)
	return v, err
}

func parseInt2(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// openURL opens rawURL in the default system browser.
func openURL(rawURL string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", rawURL}
	case "darwin":
		cmd = "open"
		args = []string{rawURL}
	default:
		cmd = "xdg-open"
		args = []string{rawURL}
	}
	return exec.Command(cmd, args...).Start()
}

// ensure math import is used for sparkline Y calculation.
var _ = math.Round
