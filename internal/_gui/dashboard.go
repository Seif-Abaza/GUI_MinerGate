// =============================================================================
// Package gui — MinerGate Operations Center v2.0
// Dark-Cyber Professional Dashboard for ASIC Mining Operations
//
// Redesign: Deep navy background, teal (#00E5B4) primary accent,
// orange (#FF6420) alert accent — matching the MinerGate screenshot.
// =============================================================================
package gui

import (
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	api "minergate/internal/api"
	"minergate/internal/charts"
	"minergate/internal/config"
	goasic "minergate/internal/dbgoasic"
	frp "minergate/internal/frp"
	"minergate/internal/models"
	"minergate/internal/plugins"
	"minergate/internal/update"
	// fynesimplechart "github.com/alexiusacademia/fynesimplechart"
)

// =============================================================================
// Color Palette — Deep Cyber Dark
// =============================================================================

var (
	// Structural backgrounds
	colorBackground  = color.RGBA{R: 8, G: 10, B: 20, A: 255}   // #080A14 — canvas void
	colorSurface     = color.RGBA{R: 13, G: 16, B: 30, A: 255}  // #0D101E — panel base
	colorSurfaceHigh = color.RGBA{R: 20, G: 24, B: 42, A: 255}  // #14182A — raised card
	colorCard        = color.RGBA{R: 11, G: 14, B: 26, A: 255}  // #0B0E1A — card inner
	colorBorder      = color.RGBA{R: 30, G: 38, B: 65, A: 255}  // #1E2641 — subtle divider
	colorHeaderBg    = color.RGBA{R: 10, G: 12, B: 24, A: 255}  // #0A0C18 — header
	colorSidebarBg   = color.RGBA{R: 10, G: 13, B: 24, A: 255}  // #0A0D18 — left nav

	// Typography
	colorTextPrimary   = color.RGBA{R: 220, G: 228, B: 245, A: 255} // #DCE4F5 — headings
	colorTextSecondary = color.RGBA{R: 100, G: 116, B: 155, A: 255} // #64749B — labels
	colorTextDim       = color.RGBA{R: 65, G: 78, B: 115, A: 255}   // #414E73 — disabled

	// Teal — SHA-256 / primary accent
	colorTeal    = color.RGBA{R: 0, G: 229, B: 180, A: 255}  // #00E5B4
	colorTealDim = color.RGBA{R: 0, G: 80, B: 63, A: 255}    // dark variant

	// Orange — alerts / ASIC-FOCUS / Scrypt secondary
	colorOrange    = color.RGBA{R: 255, G: 100, B: 32, A: 255} // #FF6420
	colorOrangeDim = color.RGBA{R: 70, G: 28, B: 8, A: 255}

	// Status colours
	colorGreen  = color.RGBA{R: 34, G: 197, B: 94, A: 255}  // #22C55E — online / normal
	colorRed    = color.RGBA{R: 239, G: 68, B: 68, A: 255}   // #EF4444 — error / offline
	colorYellow = color.RGBA{R: 245, G: 158, B: 11, A: 255}  // #F59E0B — warning / Scrypt
	colorBlue   = color.RGBA{R: 56, G: 189, B: 248, A: 255}  // #38BDF8 — info
	colorPurple = color.RGBA{R: 168, G: 85, B: 247, A: 255}  // #A855F7 — misc

	// Alert panel background
	colorAlertBg = color.RGBA{R: 48, G: 16, B: 6, A: 240}
)

// =============================================================================
// MiningTheme — Custom Fyne Theme
// =============================================================================

type MiningTheme struct{}

func (m MiningTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorBackground
	case theme.ColorNameButton:
		return colorSurface
	case theme.ColorNameDisabledButton:
		return colorSurfaceHigh
	case theme.ColorNameInputBackground:
		return colorCard
	case theme.ColorNameOverlayBackground:
		return colorSurface
	case theme.ColorNameDisabled:
		return colorTextDim
	case theme.ColorNameForeground:
		return colorTextPrimary
	case theme.ColorNamePlaceHolder:
		return colorTextSecondary
	case theme.ColorNamePressed:
		return colorSurfaceHigh
	case theme.ColorNamePrimary:
		return colorTeal
	case theme.ColorNameHover:
		return colorSurfaceHigh
	case theme.ColorNameFocus:
		return colorTeal
	case theme.ColorNameScrollBar:
		return colorBorder
	case theme.ColorNameSeparator:
		return colorBorder
	case theme.ColorNameShadow:
		return color.RGBA{A: 80}
	case theme.ColorNameSelection:
		return colorTealDim
	case theme.ColorNameMenuBackground:
		return colorCard
	case theme.ColorNameHeaderBackground:
		return colorHeaderBg
	default:
		return colorTextPrimary
	}
}

func (m MiningTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m MiningTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m MiningTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 10
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius:
		return 4
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// =============================================================================
// GaugeWidget — Circular Arc Hash-rate Meter
// =============================================================================

// GaugeWidget renders a 270° circular arc gauge using a Raster pixel function.
// Accent colour drives both the filled arc and the central value text.
type GaugeWidget struct {
	widget.BaseWidget
	mu       sync.Mutex
	value    float64
	maxValue float64
	label    string
	unit     string
	accent   color.RGBA
	raster   *canvas.Raster
}

// NewGaugeWidget creates a ready-to-use GaugeWidget.
func NewGaugeWidget(label, unit string, maxValue float64, accent color.RGBA) *GaugeWidget {
	g := &GaugeWidget{
		label:    label,
		unit:     unit,
		maxValue: maxValue,
		accent:   accent,
	}
	g.ExtendBaseWidget(g)
	return g
}

// SetValue updates the displayed value and triggers a redraw.
func (g *GaugeWidget) SetValue(v float64) {
	g.mu.Lock()
	g.value = v
	g.mu.Unlock()
	if g.raster != nil {
		g.raster.Refresh()
	}
}

// CreateRenderer implements fyne.Widget.
func (g *GaugeWidget) CreateRenderer() fyne.WidgetRenderer {
	g.raster = canvas.NewRasterWithPixels(func(px, py, w, h int) color.Color {
		return g.pixelColor(px, py, w, h)
	})

	// Value text (big, coloured)
	valText := canvas.NewText(fmt.Sprintf("%.1f", g.value), g.accent)
	valText.TextSize = 32
	valText.TextStyle = fyne.TextStyle{Bold: true}

	// Unit label
	unitText := canvas.NewText(g.unit, colorTextSecondary)
	unitText.TextSize = 12

	// Label below
	subText := canvas.NewText(g.label, colorTextDim)
	subText.TextSize = 10

	return &gaugeRenderer{
		gauge:    g,
		raster:   g.raster,
		valText:  valText,
		unitText: unitText,
		subText:  subText,
	}
}

// pixelColor is called per pixel by the Raster; renders the arc + transparent background.
func (g *GaugeWidget) pixelColor(px, py, w, h int) color.Color {
	cx := float64(w) / 2
	cy := float64(h) * 0.60

	dx := float64(px) - cx
	dy := float64(py) - cy
	dist := math.Sqrt(dx*dx + dy*dy)

	outerR := math.Min(float64(w), float64(h)) * 0.42
	thickness := outerR * 0.17
	innerR := outerR - thickness

	if dist < innerR-1 || dist > outerR+1 {
		return color.NRGBA{A: 0}
	}

	// Angle in [0, 2π]
	angle := math.Atan2(dy, dx)
	if angle < 0 {
		angle += 2 * math.Pi
	}

	// Arc: start=135°, sweep=270° clockwise.
	const startDeg = 135.0
	const sweepDeg = 270.0
	startR := startDeg * math.Pi / 180.0
	endR := (startDeg + sweepDeg) * math.Pi / 180.0 // 405° → wraps

	// Normalise angle into arc space
	var inArc bool
	var pct float64

	if angle >= startR {
		// First quadrant of arc (135°–360°)
		pct = (angle - startR) / (sweepDeg * math.Pi / 180.0)
		inArc = pct <= 1.0
	} else if angle+2*math.Pi <= endR {
		// Wrapped quadrant (0°–45°)
		pct = (angle + 2*math.Pi - startR) / (sweepDeg * math.Pi / 180.0)
		inArc = pct <= 1.0
	}

	if !inArc {
		return color.NRGBA{A: 0}
	}

	// Track background
	track := color.NRGBA{R: 22, G: 28, B: 52, A: 255}

	// Filled portion
	g.mu.Lock()
	val, max := g.value, g.maxValue
	g.mu.Unlock()

	valuePct := 0.0
	if max > 0 {
		valuePct = math.Min(val/max, 1.0)
	}

	if pct <= valuePct {
		// Soft anti-alias on edges
		edgeDist := math.Min(dist-innerR, outerR-dist)
		alpha := uint8(255)
		if edgeDist < 1.5 {
			alpha = uint8(255 * edgeDist / 1.5)
		}
		a := g.accent
		return color.NRGBA{R: a.R, G: a.G, B: a.B, A: alpha}
	}
	return track
}

// gaugeRenderer implements fyne.WidgetRenderer for GaugeWidget.
type gaugeRenderer struct {
	gauge    *GaugeWidget
	raster   *canvas.Raster
	valText  *canvas.Text
	unitText *canvas.Text
	subText  *canvas.Text
}

func (r *gaugeRenderer) Layout(s fyne.Size) {
	r.raster.Resize(s)
	r.raster.Move(fyne.NewPos(0, 0))

	// Centre the value text at 55% of height
	vw := r.valText.MinSize().Width
	vh := r.valText.MinSize().Height
	cx := s.Width / 2
	cy := s.Height * 0.55
	r.valText.Move(fyne.NewPos(cx-vw/2, cy-vh/2-6))

	uw := r.unitText.MinSize().Width
	r.unitText.Move(fyne.NewPos(cx-uw/2, cy-vh/2+vh))

	sw := r.subText.MinSize().Width
	r.subText.Move(fyne.NewPos(cx-sw/2, s.Height*0.82))
}

func (r *gaugeRenderer) MinSize() fyne.Size {
	return fyne.NewSize(180, 165)
}

func (r *gaugeRenderer) Refresh() {
	r.gauge.mu.Lock()
	v := r.gauge.value
	r.gauge.mu.Unlock()
	r.valText.Text = fmt.Sprintf("%.1f", v)
	canvas.Refresh(r.valText)
	r.raster.Refresh()
}

func (r *gaugeRenderer) Destroy() {}

func (r *gaugeRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.raster, r.valText, r.unitText, r.subText}
}

// =============================================================================
// AppState — Application Data Bindings
// =============================================================================

// AppState holds all data and metrics rendered in the interface.
type AppState struct {
	// Core metrics
	OnlineDevices  binding.Int
	TotalDevices   binding.Int
	TotalHashrate  binding.Float
	Hashrate       binding.Float
	ActiveWorkers  binding.Int
	OfflineDevices binding.Int
	Efficiency     binding.Float
	Power          binding.Int
	Balance        binding.Float
	Revenue24h     binding.Float
	Uptime         binding.Float
	Hashprice      binding.Float
	DailyBTC       binding.Float
	DailyProfit    binding.Float
	MonthlyProfit  binding.Float
	LastUpdate     binding.String

	// Selected-device metrics
	SelectedHashrate   binding.Float
	SelectedTemp       binding.Float
	SelectedPower      binding.Int
	SelectedUptime     binding.Float
	SelectedEfficiency binding.Float
	SelectedErrors     binding.Int

	// Devices
	Devices       []*models.Miner
	Workers       []Worker
	SelectedIndex int

	// Chart data
	HashrateHistory []float64

	// Settings
	AutoRefresh binding.Bool
	RefreshRate int

	// Service managers
	APIClient     *api.Client
	FRPClient     *frp.Client
	GoASICManager *goasic.Manager
	PluginManager *plugins.Manager
	UpdateManager *update.Updater

	// Configuration
	Config *config.Config
}

// Worker represents a row in the workers table.
type Worker struct {
	Name       string
	Subaccount string
	Hashrate   float64
	Efficiency float64
	Status     string
}

// NewAppState initialises an AppState with fresh bindings.
func NewAppState(cfg *config.Config) *AppState {
	return &AppState{
		OnlineDevices:      binding.NewInt(),
		TotalDevices:       binding.NewInt(),
		TotalHashrate:      binding.NewFloat(),
		Hashrate:           binding.NewFloat(),
		ActiveWorkers:      binding.NewInt(),
		OfflineDevices:     binding.NewInt(),
		Efficiency:         binding.NewFloat(),
		Power:              binding.NewInt(),
		Balance:            binding.NewFloat(),
		Revenue24h:         binding.NewFloat(),
		Uptime:             binding.NewFloat(),
		Hashprice:          binding.NewFloat(),
		DailyBTC:           binding.NewFloat(),
		DailyProfit:        binding.NewFloat(),
		MonthlyProfit:      binding.NewFloat(),
		LastUpdate:         binding.NewString(),
		SelectedHashrate:   binding.NewFloat(),
		SelectedTemp:       binding.NewFloat(),
		SelectedPower:      binding.NewInt(),
		SelectedUptime:     binding.NewFloat(),
		SelectedEfficiency: binding.NewFloat(),
		SelectedErrors:     binding.NewInt(),
		AutoRefresh:        binding.NewBool(),
		RefreshRate:        cfg.RefreshRate,
		Config:             cfg,
		Devices:            make([]*models.Miner, 0),
		Workers:            make([]Worker, 0),
		SelectedIndex:      -1,
	}
}

// SetDefaults populates demo/default values for initial display.
func (s *AppState) SetDefaults() {
	s.OnlineDevices.Set(18)
	s.TotalDevices.Set(20)
	s.TotalHashrate.Set(143.1)
	s.Power.Set(47400)
	s.DailyBTC.Set(0.00027945)
	s.DailyProfit.Set(-16.96)
	s.MonthlyProfit.Set(-508.73)
	s.Revenue24h.Set(4812.0)
	s.Efficiency.Set(3.02)
	s.LastUpdate.Set(time.Now().Format("15:04:05"))

	s.SelectedHashrate.Set(110.0)
	s.SelectedTemp.Set(62.0)
	s.SelectedPower.Set(3200)
	s.SelectedUptime.Set(99.1)
	s.SelectedEfficiency.Set(26.9)
	s.SelectedErrors.Set(0)

	s.Hashrate.Set(98.4)
	s.ActiveWorkers.Set(18)
	s.OfflineDevices.Set(2)

	s.AutoRefresh.Set(true)

	s.Workers = []Worker{
		{Name: "Worker-01", Subaccount: "main", Hashrate: 78.5, Efficiency: 95.2, Status: "online"},
		{Name: "Worker-02", Subaccount: "backup", Hashrate: 74.3, Efficiency: 92.7, Status: "online"},
		{Name: "Worker-03", Subaccount: "test", Hashrate: 65.8, Efficiency: 90.1, Status: "offline"},
	}

	s.Devices = make([]*models.Miner, 0)
	s.UpdateDeviceCounts()

	s.HashrateHistory = []float64{
		480, 492, 505, 499, 510, 518, 525, 520,
		515, 522, 530, 527, 518, 512, 520, 526,
		532, 528, 523, 519, 517, 514, 510, 508,
	}
}

// UpdateDeviceCounts recalculates device counts from the current device list.
func (s *AppState) UpdateDeviceCounts() {
	total := len(s.Devices)
	online := 0
	for _, d := range s.Devices {
		if d != nil && d.Status == "online" {
			online++
		}
	}
	s.TotalDevices.Set(total)
	s.OnlineDevices.Set(online)
	s.ActiveWorkers.Set(online)
	s.OfflineDevices.Set(total - online)
}

// =============================================================================
// DashboardApp — Application Root
// =============================================================================

// DashboardApp is the top-level application controller.
type DashboardApp struct {
	App            fyne.App
	Window         fyne.Window
	State          *AppState
	Chart          *ChartWidget
	Ticker         *time.Ticker
	StopChan       chan bool
	mu             sync.RWMutex
	DeviceList     *widget.List
	DeviceCountStr binding.String

	// Gauge references for live updates
	sha256Gauge *GaugeWidget
	scryptGauge *GaugeWidget
}

// NewDashboard wires up all service managers and returns an initialised DashboardApp.
func NewDashboard(
	cfg *config.Config,
	apiClient *api.Client,
	frpClient *frp.Client,
	goasicMgr *goasic.Manager,
	pluginMgr *plugins.Manager,
	updateMgr *update.Updater,
) *DashboardApp {

	a := app.NewWithID("io.minergate.dashboard")
	a.Settings().SetTheme(&MiningTheme{})

	w := a.NewWindow(fmt.Sprintf("MinerGate — Operations Center | v%s", config.Version))
	w.Resize(fyne.NewSize(float32(cfg.WindowWidth), float32(cfg.WindowHeight)))
	w.SetFullScreen(false)
	w.CenterOnScreen()

	state := NewAppState(cfg)
	state.APIClient = apiClient
	state.FRPClient = frpClient
	state.GoASICManager = goasicMgr
	state.PluginManager = pluginMgr
	state.UpdateManager = updateMgr
	state.SetDefaults()

	d := &DashboardApp{
		App:      a,
		Window:   w,
		State:    state,
		StopChan: make(chan bool),
	}

	if goasicMgr != nil {
		goasicMgr.OnDeviceDiscovered(func(device *goasic.DiscoveredDevice) { go d.refreshData() })
		goasicMgr.OnDeviceUpdate(func(device *goasic.DiscoveredDevice) { go d.refreshData() })
		goasicMgr.OnDeviceLost(func(ip string) { go d.refreshData() })
	}
	return d
}

// Run builds the UI, starts auto-refresh, and enters the event loop.
func (d *DashboardApp) Run() {
	content := d.buildUI(d.State.Config.ApplicationName, d.State.Config.FarmUUID)
	d.Window.SetContent(content)
	d.startAutoRefresh()

	if d.State.GoASICManager != nil {
		go func() {
			for !d.State.GoASICManager.IsScanning() {
				time.Sleep(200 * time.Millisecond)
			}
			for d.State.GoASICManager.IsScanning() {
				time.Sleep(500 * time.Millisecond)
			}
			d.refreshData()
		}()
	}

	d.Window.SetCloseIntercept(func() {
		d.StopChan <- true
		d.App.Quit()
	})
	d.Window.ShowAndRun()
}

// =============================================================================
// UI Construction — Top Level
// =============================================================================

// buildUI assembles the full application layout:
//
//	Header bar
//	├── Left icon nav sidebar
//	├── Center content  (metrics + gauges + chart)
//	└── Right ASIC panel
//	Bottom ticker bar
func (d *DashboardApp) buildUI(appName, farmUUID string) fyne.CanvasObject {
	header := d.createHeader(appName, farmUUID)
	footer := d.createTickerBar()

	navSidebar := d.createNavSidebar()
	centerContent := d.createMainContent()
	asicPanel := d.createRightASICPanel()

	// Three-column horizontal split: nav | center | asic-panel
	body := container.NewBorder(
		nil, nil,
		navSidebar, // left fixed nav
		asicPanel,  // right fixed ASIC panel
		centerContent,
	)

	return container.NewBorder(header, footer, nil, nil, body)
}

// =============================================================================
// Header Bar
// =============================================================================

func (d *DashboardApp) createHeader(appName, farmUUID string) fyne.CanvasObject {
	// ── Left: MG badge + title ──────────────────────────────────────────────
	mgBadge := d.makeBadge("MG", colorTeal, colorBackground)

	titleText := canvas.NewText("MINERGATE — OPERATIONS CENTER", colorTextPrimary)
	titleText.TextSize = 15
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	// ── Centre: ASIC FOCUS button ────────────────────────────────────────────
	asicFocusBtn := widget.NewButton("⊕ ASIC FOCUS", nil)
	asicFocusBtn.Importance = widget.DangerImportance

	// ── Right: Pool + time ───────────────────────────────────────────────────
	poolDot := canvas.NewText("●", colorGreen)
	poolDot.TextSize = 11

	poolText := canvas.NewText("pool.minergate.com:3333", colorTextSecondary)
	poolText.TextSize = 12

	timeLabel := widget.NewLabelWithData(
		binding.NewDataItem().(binding.String), // placeholder; replaced below
	)
	// Use a live updating clock
	clockStr := binding.NewString()
	clockStr.Set(time.Now().Format("15:04:05"))
	go func() {
		for range time.Tick(time.Second) {
			clockStr.Set(time.Now().Format("15:04:05"))
		}
	}()
	clockLabel := widget.NewLabelWithData(clockStr)
	clockLabel.TextStyle = fyne.TextStyle{Monospace: true}
	_ = timeLabel

	right := container.NewHBox(poolDot, poolText, widget.NewSeparator(), clockLabel)
	left := container.NewHBox(mgBadge, d.spacer(6), titleText)
	centre := container.NewHBox(asicFocusBtn)

	row := container.NewBorder(nil, nil, left, right, container.NewCenter(centre))

	headerBg := canvas.NewRectangle(colorHeaderBg)
	divider := canvas.NewRectangle(colorBorder)
	divider.SetMinSize(fyne.NewSize(0, 1))

	return container.NewStack(
		headerBg,
		container.NewVBox(container.NewPadded(row), divider),
	)
}

// makeBadge creates a coloured pill label (e.g. the "MG" logo).
func (d *DashboardApp) makeBadge(text string, bg, fg color.RGBA) fyne.CanvasObject {
	label := canvas.NewText(text, fg)
	label.TextSize = 13
	label.TextStyle = fyne.TextStyle{Bold: true}

	bgRect := canvas.NewRectangle(bg)
	bgRect.CornerRadius = 5

	padded := container.NewPadded(label)
	padded.Move(fyne.NewPos(2, 2))

	return container.NewStack(bgRect, padded)
}

// spacer inserts a fixed horizontal gap.
func (d *DashboardApp) spacer(width float32) fyne.CanvasObject {
	s := canvas.NewRectangle(color.NRGBA{A: 0})
	s.SetMinSize(fyne.NewSize(width, 1))
	return s
}

// =============================================================================
// Left Navigation Sidebar (icon-only)
// =============================================================================

func (d *DashboardApp) createNavSidebar() fyne.CanvasObject {
	items := []struct{ icon, label string }{
		{"▦", "Dashboard"},
		{"▤", "Reports"},
		{"◉", "Network"},
		{"⌁", "Activity"},
		{"❐", "Layers"},
		{"✦", "Settings"},
	}

	var buttons []fyne.CanvasObject
	buttons = append(buttons, d.spacer(1)) // top padding

	for i, item := range items {
		iconText := canvas.NewText(item.icon, colorTextSecondary)
		iconText.TextSize = 16

		if i == 0 {
			iconText.Color = colorTeal // active item
			bgRect := canvas.NewRectangle(colorSurfaceHigh)
			bgRect.CornerRadius = 6
			cell := container.NewStack(bgRect, container.NewCenter(iconText))
			cell.(*fyne.Container).Resize(fyne.NewSize(40, 40))
			buttons = append(buttons, container.NewPadded(container.NewCenter(cell)))
		} else {
			buttons = append(buttons, container.NewPadded(container.NewCenter(iconText)))
		}
	}

	col := container.NewVBox(buttons...)

	bg := canvas.NewRectangle(colorSidebarBg)
	bg.SetMinSize(fyne.NewSize(52, 0))

	divider := canvas.NewRectangle(colorBorder)
	divider.SetMinSize(fyne.NewSize(1, 0))

	return container.NewStack(
		bg,
		container.NewBorder(nil, nil, nil, divider, col),
	)
}

// =============================================================================
// Main Content Area
// =============================================================================

func (d *DashboardApp) createMainContent() fyne.CanvasObject {
	// Top: six stats cards
	statsBar := d.createTopStatsBar()

	// Middle: SHA-256 gauge | Performance chart | Scrypt gauge
	gaugeRow := d.createHashrateRow()

	// Alert banner (conditionally shown)
	alert := d.createAlertBanner("ASIC-04 — Fan speed below threshold (820 RPM). Action required.")

	content := container.NewVBox(
		statsBar,
		d.thinSeparator(),
		gaugeRow,
		d.thinSeparator(),
		alert,
	)

	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(640, 600))

	bg := canvas.NewRectangle(colorBackground)
	return container.NewStack(bg, container.NewPadded(scroll))
}

// =============================================================================
// Top Stats Bar — six KPI cards
// =============================================================================

func (d *DashboardApp) createTopStatsBar() fyne.CanvasObject {
	type card struct {
		title  string
		bValue binding.DataItem
		unit   string
		accent color.RGBA
		trend  string
	}

	cards := []card{
		{"TOTAL HASHRATE", d.State.TotalHashrate, "TH/s", colorTeal, "▲ 2.3%"},
		{"ACTIVE DEVICES", d.State.ActiveWorkers, "/20", colorGreen, ""},
		{"24H REVENUE", d.State.Revenue24h, "$", colorBlue, "▲ 0.3%"},
		{"AVG TEMPERATURE", d.State.SelectedTemp, "°C", colorYellow, "— stable"},
		{"POWER DRAW", d.State.Power, "W", colorOrange, "▲ 1.0%"},
		{"EFFICIENCY", d.State.Efficiency, "TH/kW", colorPurple, "▲ 0.8%"},
	}

	cells := make([]fyne.CanvasObject, len(cards))
	for i, c := range cards {
		cells[i] = d.createMetricCard(c.title, c.bValue, c.unit, c.accent, c.trend)
	}

	grid := container.NewGridWithColumns(len(cells), cells...)
	return grid
}

// =============================================================================
// Metric Card
// =============================================================================

func (d *DashboardApp) createMetricCard(title string, value binding.DataItem, unit string, accent color.RGBA, trend string) fyne.CanvasObject {
	// Title
	titleLbl := canvas.NewText(title, colorTextSecondary)
	titleLbl.TextSize = 10

	// Trend indicator
	var trendLbl *canvas.Text
	if trend != "" {
		trendColor := colorGreen
		if strings.HasPrefix(trend, "▼") {
			trendColor = colorRed
		} else if strings.HasPrefix(trend, "—") {
			trendColor = colorTextSecondary
		}
		trendLbl = canvas.NewText(trend, trendColor)
		trendLbl.TextSize = 10
	}

	// Main value display
	valText := canvas.NewText("—", accent)
	valText.TextSize = 26
	valText.TextStyle = fyne.TextStyle{Bold: true}

	// Wire up binding
	updateVal := func() {
		var s string
		switch v := value.(type) {
		case binding.Float:
			f, _ := v.Get()
			switch unit {
			case "$":
				s = fmt.Sprintf("$%.0f", f)
			case "TH/s", "TH/kW":
				s = fmt.Sprintf("%.1f %s", f, unit)
			case "°C":
				s = fmt.Sprintf("%.1f°C", f)
			default:
				s = fmt.Sprintf("%.1f %s", f, unit)
			}
		case binding.Int:
			i, _ := v.Get()
			if unit != "" && !strings.HasPrefix(unit, "/") {
				s = fmt.Sprintf("%d %s", i, unit)
			} else {
				s = fmt.Sprintf("%d%s", i, unit)
			}
		}
		valText.Text = s
		valText.Refresh()
	}

	switch v := value.(type) {
	case binding.Float:
		v.AddListener(binding.NewDataListener(updateVal))
	case binding.Int:
		v.AddListener(binding.NewDataListener(updateVal))
	}
	updateVal()

	// Accent underline
	underline := canvas.NewRectangle(accent)
	underline.SetMinSize(fyne.NewSize(0, 2))

	// Header row
	var headerRow fyne.CanvasObject
	if trendLbl != nil {
		headerRow = container.NewBorder(nil, nil, nil, trendLbl, titleLbl)
	} else {
		headerRow = titleLbl
	}

	inner := container.NewVBox(headerRow, valText, underline)
	bg := canvas.NewRectangle(colorCard)
	bg.CornerRadius = 6

	padded := container.NewPadded(inner)
	return container.NewStack(bg, padded)
}

// =============================================================================
// Hashrate Row — SHA-256 gauge | Performance chart | Scrypt gauge
// =============================================================================

func (d *DashboardApp) createHashrateRow() fyne.CanvasObject {
	sha256Panel := d.createHashPanel("SHA-256 HASHRATE", "BTC", "TH/s", 120.0, colorTeal, d.State.Hashrate)
	scryptPanel := d.createHashPanel("SCRYPT HASHRATE", "LTC", "GH/s", 60.0, colorYellow, d.State.SelectedHashrate)
	chartPanel := d.createChart()

	row := container.NewGridWithColumns(3, sha256Panel, chartPanel, scryptPanel)
	return row
}

// createHashPanel builds the labelled gauge panel for one algorithm.
func (d *DashboardApp) createHashPanel(title, coin, unit string, maxVal float64, accent color.RGBA, bindVal binding.Float) fyne.CanvasObject {
	// Header
	titleLbl := canvas.NewText(title, colorTextPrimary)
	titleLbl.TextSize = 12
	titleLbl.TextStyle = fyne.TextStyle{Bold: true}

	coinBadge := d.makeBadge(coin, accent, colorBackground)

	header := container.NewBorder(nil, nil, titleLbl, coinBadge)

	// Gauge
	gauge := NewGaugeWidget("", unit, maxVal, accent)
	gauge.ExtendBaseWidget(gauge)

	// Store reference so refreshData can update it
	if strings.Contains(title, "SHA-256") {
		d.sha256Gauge = gauge
	} else {
		d.scryptGauge = gauge
	}

	// Wire binding to gauge
	bindVal.AddListener(binding.NewDataListener(func() {
		v, _ := bindVal.Get()
		gauge.SetValue(v)
	}))
	if v, err := bindVal.Get(); err == nil {
		gauge.SetValue(v)
	}

	// Mini-stats row below gauge
	statRow := d.createGaugeStats(title)

	inner := container.NewVBox(header, gauge, statRow)
	bg := canvas.NewRectangle(colorCard)
	bg.CornerRadius = 8
	return container.NewStack(bg, container.NewPadded(inner))
}

// createGaugeStats renders the three-slot stat row under a gauge.
func (d *DashboardApp) createGaugeStats(title string) fyne.CanvasObject {
	var items []struct{ label, value string; col color.RGBA }

	if strings.Contains(title, "SHA-256") {
		items = []struct{ label, value string; col color.RGBA }{
			{"DEVICES", "12", colorTextPrimary},
			{"UPTIME", "99.1%", colorGreen},
			{"POWER", "31.4 kW", colorOrange},
		}
	} else {
		items = []struct{ label, value string; col color.RGBA }{
			{"DEVICES", "6", colorTextPrimary},
			{"UPTIME", "97.3%", colorGreen},
			{"POWER", "15.8 kW", colorOrange},
		}
	}

	cells := make([]fyne.CanvasObject, len(items))
	for i, it := range items {
		lbl := canvas.NewText(it.label, colorTextDim)
		lbl.TextSize = 9
		val := canvas.NewText(it.value, it.col)
		val.TextSize = 12
		val.TextStyle = fyne.TextStyle{Bold: true}
		c := container.NewVBox(container.NewCenter(lbl), container.NewCenter(val))
		cells[i] = c
	}
	return container.NewGridWithColumns(len(cells), cells...)
}

// =============================================================================
// Alert Banner
// =============================================================================

func (d *DashboardApp) createAlertBanner(msg string) fyne.CanvasObject {
	icon := canvas.NewText("⚠", colorOrange)
	icon.TextSize = 14

	text := canvas.NewText(msg, colorOrange)
	text.TextSize = 12

	dismissBtn := widget.NewButton("✕", nil)
	dismissBtn.Importance = widget.LowImportance

	row := container.NewBorder(nil, nil, container.NewHBox(icon, text), dismissBtn)

	bg := canvas.NewRectangle(colorAlertBg)
	bg.CornerRadius = 6

	return container.NewStack(bg, container.NewPadded(row))
}

// =============================================================================
// Performance History Chart
// =============================================================================

func (d *DashboardApp) createChart() fyne.CanvasObject {
	titleLbl := canvas.NewText("PERFORMANCE HISTORY", colorTextPrimary)
	titleLbl.TextSize = 12
	titleLbl.TextStyle = fyne.TextStyle{Bold: true}

	// Time range buttons
	btn1H := widget.NewButton("1H", nil)
	btn6H := widget.NewButton("6H", nil)
	btn24H := widget.NewButton("24H", nil)
	btn24H.Importance = widget.HighImportance
	btn7D := widget.NewButton("7D", nil)

	rangeRow := container.NewHBox(btn1H, btn6H, btn24H, btn7D)
	header := container.NewBorder(nil, nil, titleLbl, rangeRow)

	// Chart widget
	chartObj := d.createChartWidget()

	inner := container.NewBorder(header, nil, nil, nil, chartObj)
	bg := canvas.NewRectangle(colorCard)
	bg.CornerRadius = 8
	return container.NewStack(bg, container.NewPadded(inner))
}

// =============================================================================
// Right ASIC Device Panel
// =============================================================================

func (d *DashboardApp) createRightASICPanel() fyne.CanvasObject {
	// Header
	titleLbl := canvas.NewText("ASIC DEVICES", colorTextPrimary)
	titleLbl.TextSize = 13
	titleLbl.TextStyle = fyne.TextStyle{Bold: true}

	d.DeviceCountStr = binding.NewString()
	d.DeviceCountStr.Set(fmt.Sprintf("%d / %d", 18, 20))
	countLbl := widget.NewLabelWithData(d.DeviceCountStr)
	countLbl.TextStyle = fyne.TextStyle{Monospace: true}

	header := container.NewBorder(nil, nil, titleLbl, countLbl)
	divider := d.thinSeparator()

	// Device list
	list := widget.NewList(
		func() int {
			d.mu.RLock()
			defer d.mu.RUnlock()
			return len(d.State.Devices)
		},
		func() fyne.CanvasObject {
			return d.buildASICCardTemplate()
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			d.mu.RLock()
			if int(id) >= len(d.State.Devices) {
				d.mu.RUnlock()
				return
			}
			dev := d.State.Devices[id]
			d.mu.RUnlock()
			d.populateASICCard(item, dev)
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		d.mu.Lock()
		d.State.SelectedIndex = int(id)
		d.mu.Unlock()
		d.mu.RLock()
		d.updateSelectedDevice()
		d.mu.RUnlock()
	}
	list.OnUnselected = func(id widget.ListItemID) {
		d.mu.Lock()
		d.State.SelectedIndex = -1
		d.mu.Unlock()
		d.mu.RLock()
		d.updateSelectedDevice()
		d.mu.RUnlock()
	}
	d.DeviceList = list

	content := container.NewBorder(container.NewVBox(header, divider), nil, nil, nil, list)

	bg := canvas.NewRectangle(colorSurface)
	bg.SetMinSize(fyne.NewSize(270, 0))

	leftDiv := canvas.NewRectangle(colorBorder)
	leftDiv.SetMinSize(fyne.NewSize(1, 0))

	return container.NewStack(bg, container.NewBorder(nil, nil, leftDiv, nil, container.NewPadded(content)))
}

// buildASICCardTemplate creates the reusable template for a device card.
func (d *DashboardApp) buildASICCardTemplate() fyne.CanvasObject {
	statusDot := canvas.NewText("●", colorGreen)
	statusDot.TextSize = 10

	modelLbl := canvas.NewText("Antminer S19 Pro", colorTextPrimary)
	modelLbl.TextSize = 12
	modelLbl.TextStyle = fyne.TextStyle{Bold: true}

	ipLbl := canvas.NewText("ASIC-01", colorTextSecondary)
	ipLbl.TextSize = 10

	hrLbl := canvas.NewText("110 TH/s", colorTeal)
	hrLbl.TextSize = 11
	hrLbl.TextStyle = fyne.TextStyle{Bold: true}

	tempLbl := canvas.NewText("Tmp: 62°C", colorTextSecondary)
	tempLbl.TextSize = 10

	fanLbl := canvas.NewText("Fan: 4200 RPM", colorTextSecondary)
	fanLbl.TextSize = 10

	pwrLbl := canvas.NewText("Pwr: 3.2 kW", colorTextSecondary)
	pwrLbl.TextSize = 10

	bar := widget.NewProgressBar()
	bar.SetValue(0.99)

	topRow := container.NewHBox(statusDot, d.spacer(4), modelLbl, layout.NewSpacer(), ipLbl)
	statsRow := container.NewHBox(hrLbl, d.spacer(8), tempLbl, layout.NewSpacer())
	fanRow := container.NewHBox(fanLbl, layout.NewSpacer(), pwrLbl)

	inner := container.NewVBox(topRow, statsRow, fanRow, bar)
	bg := canvas.NewRectangle(colorSurfaceHigh)
	bg.CornerRadius = 6
	return container.NewStack(bg, container.NewPadded(inner))
}

// populateASICCard fills a template card with real device data.
func (d *DashboardApp) populateASICCard(item fyne.CanvasObject, dev *models.Miner) {
	stack, ok := item.(*fyne.Container)
	if !ok || len(stack.Objects) < 2 {
		return
	}
	padded, ok := stack.Objects[1].(*fyne.Container)
	if !ok {
		return
	}
	vbox, ok := padded.Objects[0].(*fyne.Container)
	if !ok || len(vbox.Objects) < 4 {
		return
	}

	// topRow
	if topRow, ok := vbox.Objects[0].(*fyne.Container); ok && len(topRow.Objects) >= 5 {
		if dot, ok := topRow.Objects[0].(*canvas.Text); ok {
			if dev.Status == "online" {
				dot.Color = colorGreen
			} else {
				dot.Color = colorRed
			}
			dot.Refresh()
		}
		if mdl, ok := topRow.Objects[2].(*canvas.Text); ok {
			name := dev.Model
			if name == "" {
				name = dev.Name
			}
			mdl.Text = name
			mdl.Refresh()
		}
		if ip, ok := topRow.Objects[4].(*canvas.Text); ok {
			ip.Text = dev.IPAddress
			ip.Refresh()
		}
	}

	// statsRow
	if statsRow, ok := vbox.Objects[1].(*fyne.Container); ok && len(statsRow.Objects) >= 1 {
		if hr, ok := statsRow.Objects[0].(*canvas.Text); ok {
			hr.Text = fmt.Sprintf("%.1f TH/s", dev.Stats.Hashrate)
			hr.Refresh()
		}
		if tmp, ok := statsRow.Objects[2].(*canvas.Text); ok {
			tmp.Text = fmt.Sprintf("Tmp: %.0f°C", dev.Stats.Temperature)
			tmp.Refresh()
		}
	}

	// fanRow
	if fanRow, ok := vbox.Objects[2].(*fyne.Container); ok && len(fanRow.Objects) >= 2 {
		fanSpeed := 0
		if len(dev.Stats.FanSpeeds) > 0 {
			fanSpeed = dev.Stats.FanSpeeds[0]
		}
		if fan, ok := fanRow.Objects[0].(*canvas.Text); ok {
			fan.Text = fmt.Sprintf("Fan: %d RPM", fanSpeed)
			// Warn if fan is low
			if fanSpeed > 0 && fanSpeed < 2000 {
				fan.Color = colorOrange
			} else {
				fan.Color = colorTextSecondary
			}
			fan.Refresh()
		}
		if pwr, ok := fanRow.Objects[2].(*canvas.Text); ok {
			pwr.Text = fmt.Sprintf("Pwr: %.1f kW", float64(dev.Stats.Power)/1000.0)
			pwr.Refresh()
		}
	}

	// progress bar — efficiency as 0-1
	if bar, ok := vbox.Objects[3].(*widget.ProgressBar); ok {
		pct := 0.95
		if dev.Stats.Efficiency > 0 && dev.Stats.Efficiency < 100 {
			pct = dev.Stats.Efficiency / 100.0
		}
		bar.SetValue(pct)
	}
}

// =============================================================================
// Device List (sidebar — kept for backwards-compat; superseded by ASIC panel)
// =============================================================================

func (d *DashboardApp) createDeviceList() fyne.CanvasObject {
	return d.createRightASICPanel()
}

// =============================================================================
// Bottom Ticker Bar
// =============================================================================

func (d *DashboardApp) createFooter() fyne.CanvasObject {
	return d.createTickerBar()
}

func (d *DashboardApp) createTickerBar() fyne.CanvasObject {
	type tick struct{ label string; col color.RGBA }
	items := []tick{
		{"BTC $67,620  ▲2.14%", colorGreen},
		{"LTC $84.32  ▲0.87%", colorGreen},
		{"ETH $3,218  ▼0.15%", colorRed},
		{"Difficulty 88.1T  ▲2.3% epoch", colorTextSecondary},
		{"Block #843,221", colorBlue},
		{"Pool Shares  1,284 accepted  3 rejected", colorTextSecondary},
		{"Latency 12ms", colorTeal},
		{"Uptime 18d 6h 43m", colorTextSecondary},
	}

	cells := make([]fyne.CanvasObject, 0, len(items)*2)
	for i, it := range items {
		t := canvas.NewText(it.label, it.col)
		t.TextSize = 11
		cells = append(cells, t)
		if i < len(items)-1 {
			sep := canvas.NewText("│", colorBorder)
			sep.TextSize = 11
			cells = append(cells, sep)
		}
	}

	row := container.NewHBox(cells...)
	paddedRow := container.NewPadded(row)

	bg := canvas.NewRectangle(colorHeaderBg)
	topLine := canvas.NewRectangle(colorBorder)
	topLine.SetMinSize(fyne.NewSize(0, 1))

	return container.NewStack(bg, container.NewVBox(topLine, paddedRow))
}

// =============================================================================
// Metrics Grid — kept for buildUI compatibility
// =============================================================================

func (m *DashboardApp) createMetricsGrid() fyne.CanvasObject {
	return m.createTopStatsBar()
}

// =============================================================================
// Thin separator helper
// =============================================================================

func (d *DashboardApp) thinSeparator() fyne.CanvasObject {
	sep := canvas.NewRectangle(colorBorder)
	sep.SetMinSize(fyne.NewSize(0, 1))
	return sep
}

// createTab is kept for header tab compatibility (currently unused).
func (m *DashboardApp) createTab(text string, active bool) fyne.CanvasObject {
	col := colorTextSecondary
	if active {
		col = colorTeal
	}
	lbl := canvas.NewText(text, col)
	lbl.TextSize = 13
	if active {
		lbl.TextStyle = fyne.TextStyle{Bold: true}
	}
	return container.NewPadded(lbl)
}

// createMenuItem kept for side-nav compatibility.
func (m *DashboardApp) createMenuItem(name, icon, badge string, active bool) fyne.CanvasObject {
	col := colorTextSecondary
	if active {
		col = colorTeal
	}
	lbl := canvas.NewText(fmt.Sprintf("%s %s", icon, name), col)
	lbl.TextSize = 13
	if badge != "" {
		badgeLbl := canvas.NewText(badge, colorGreen)
		badgeLbl.TextSize = 10
		return container.NewPadded(container.NewHBox(lbl, badgeLbl))
	}
	return container.NewPadded(lbl)
}

// =============================================================================
// ChartWidget — Hashrate Time-series
// =============================================================================

// ChartWidget manages the hashrate chart display.
type ChartWidget struct {
	widget.BaseWidget
	mu       sync.Mutex
	inner    *fyne.Container
	csvPath  string
	appState *AppState
	data     []float64
	minVal   float64
	maxVal   float64
}

// UpdateData replaces chart data and redraws.
func (c *ChartWidget) UpdateData(newData []float64) {
	var newObj fyne.CanvasObject
	if len(newData) > 0 {
		if gw := charts.BuildGraphWidget(newData, "Hashrate (TH/s)"); gw != nil {
			newObj = gw
		}
	}
	if newObj == nil {
		ph := canvas.NewText("⏳ Waiting for data…", charts.ChartLabelColor)
		ph.TextSize = 13
		ph.TextStyle = fyne.TextStyle{Italic: true}
		newObj = ph
	}
	c.mu.Lock()
	if len(c.inner.Objects) >= 2 {
		c.inner.Objects[1] = newObj
	} else {
		c.inner.Objects = append(c.inner.Objects, newObj)
	}
	c.mu.Unlock()
	fyne.Do(func() { c.inner.Refresh() })
}

// UpdateFromCSV reads CSV values and refreshes the chart.
func (c *ChartWidget) UpdateFromCSV() {
	if c.csvPath == "" {
		return
	}
	if vals := charts.ReadCSVValues(c.csvPath); len(vals) > 0 {
		c.UpdateData(vals)
	}
}

// createChartWidget initialises a ChartWidget with existing CSV data.
func (m *DashboardApp) createChartWidget() fyne.CanvasObject {
	csvPath := filepath.Join("device_log", "total_hashrate.csv")

	var data []float64
	if csvData := charts.ReadCSVValues(csvPath); len(csvData) > 0 {
		data = csvData
	} else if len(m.State.Devices) > 0 {
		if m.State.SelectedIndex >= 0 && m.State.SelectedIndex < len(m.State.Devices) {
			data = m.State.Devices[m.State.SelectedIndex].Stats.HashrateHistory
		} else {
			var maxLen int
			for _, d := range m.State.Devices {
				if len(d.Stats.HashrateHistory) > maxLen {
					maxLen = len(d.Stats.HashrateHistory)
				}
			}
			data = make([]float64, maxLen)
			for _, d := range m.State.Devices {
				offset := maxLen - len(d.Stats.HashrateHistory)
				for i, h := range d.Stats.HashrateHistory {
					data[offset+i] += h
				}
			}
		}
	} else {
		data = m.State.HashrateHistory
	}

	chart := &ChartWidget{data: data, appState: m.State, csvPath: csvPath}

	var initialObj fyne.CanvasObject
	if len(data) > 0 {
		if gw := charts.BuildGraphWidget(data, "Hashrate (TH/s)"); gw != nil {
			initialObj = gw
		}
	}
	if initialObj == nil {
		ph := canvas.NewText("⏳ Waiting for hashrate data…", charts.ChartLabelColor)
		ph.TextSize = 13
		ph.TextStyle = fyne.TextStyle{Italic: true}
		initialObj = ph
	}

	chart.inner = container.NewStack(initialObj)
	chart.ExtendBaseWidget(chart)
	chart.calculateRange()

	m.Chart = chart
	return chart
}

func (c *ChartWidget) calculateRange() {
	if len(c.data) == 0 {
		return
	}
	c.minVal = c.data[0]
	c.maxVal = c.data[0]
	for _, v := range c.data {
		if v < c.minVal {
			c.minVal = v
		}
		if v > c.maxVal {
			c.maxVal = v
		}
	}
	if c.maxVal == c.minVal {
		c.maxVal = c.minVal + 1
	}
}

func (c *ChartWidget) CreateRenderer() fyne.WidgetRenderer {
	if c.inner == nil {
		ph := canvas.NewText("⏳ Waiting for hashrate data…", charts.ChartLabelColor)
		ph.TextSize = 13
		ph.TextStyle = fyne.TextStyle{Italic: true}
		c.inner = container.NewStack(ph)
	}
	return widget.NewSimpleRenderer(c.inner)
}

// =============================================================================
// Legacy chartRenderer / fynesimplechartWidget
// (retained for binary-compat; not actively used)
// =============================================================================

type chartRenderer struct {
	widget     *ChartWidget
	lineChart  *fynesimplechartWidget
	placeholder *canvas.Text
}

type fynesimplechartWidget struct {
	gw fyne.Widget
}

func newFyneSimpleChart(data []float64) *fynesimplechartWidget {
	gw := charts.BuildGraphWidget(data, "Hashrate")
	if gw == nil {
		return nil
	}
	return &fynesimplechartWidget{gw: gw}
}

func (r *chartRenderer) build() {
	r.placeholder = canvas.NewText("⏳ Waiting for hashrate data…", charts.ChartLabelColor)
	r.placeholder.TextSize = 13
	r.placeholder.TextStyle = fyne.TextStyle{Italic: true}
	if len(r.widget.data) > 0 {
		r.lineChart = newFyneSimpleChart(r.widget.data)
	}
}

func (r *chartRenderer) Layout(size fyne.Size) {
	if r.lineChart != nil {
		r.lineChart.gw.Resize(size)
		r.lineChart.gw.Move(fyne.NewPos(0, 0))
	}
	if r.placeholder != nil {
		r.placeholder.Resize(size)
		r.placeholder.Move(fyne.NewPos(0, size.Height/2-8))
	}
}

func (r *chartRenderer) MinSize() fyne.Size { return fyne.NewSize(400, 180) }

func (r *chartRenderer) Refresh() {
	if len(r.widget.data) == 0 {
		if r.lineChart != nil {
			r.lineChart.gw.Hide()
		}
		if r.placeholder != nil {
			r.placeholder.Show()
			canvas.Refresh(r.placeholder)
		}
		return
	}
	if r.placeholder != nil {
		r.placeholder.Hide()
	}
	r.lineChart = newFyneSimpleChart(r.widget.data)
	if r.lineChart != nil {
		r.lineChart.gw.Show()
		canvas.Refresh(r.lineChart.gw)
	}
}

func (r *chartRenderer) Destroy() {}

func (r *chartRenderer) Objects() []fyne.CanvasObject {
	var objs []fyne.CanvasObject
	if r.lineChart != nil {
		objs = append(objs, r.lineChart.gw)
	}
	if r.placeholder != nil {
		objs = append(objs, r.placeholder)
	}
	return objs
}

// =============================================================================
// Auto-Refresh & Device Sync
// =============================================================================

func (d *DashboardApp) startAutoRefresh() {
	d.Ticker = time.NewTicker(time.Duration(d.State.RefreshRate) * time.Second)
	go func() {
		for {
			select {
			case <-d.Ticker.C:
				if ar, _ := d.State.AutoRefresh.Get(); ar {
					d.refreshData()
				}
			case <-d.StopChan:
				d.Ticker.Stop()
				return
			}
		}
	}()
}

func (d *DashboardApp) syncDevices() {
	if d.State.GoASICManager == nil {
		return
	}
	discovered := d.State.GoASICManager.GetDevices()
	if len(discovered) == 0 && d.State.GoASICManager.IsScanning() {
		return
	}

	existingDevices := make(map[string]*models.Miner)
	for _, dev := range d.State.Devices {
		existingDevices[dev.ID] = dev
	}

	newDevices := make([]*models.Miner, 0, len(discovered))
	for _, dev := range discovered {
		miner := &models.Miner{
			ID: dev.IP, Name: dev.IP, Model: dev.Model,
			Manufacturer: dev.Make, IPAddress: dev.IP,
			Status: string(dev.Status), LastSeen: dev.LastSeen,
		}
		if dev.Data != nil {
			if dev.Data.Hostname != "" {
				miner.Name = dev.Data.Hostname
			} else if dev.Model != "" {
				miner.Name = dev.Model + " (" + dev.IP + ")"
			}
			miner.IsMining = dev.Data.IsMining
			miner.Algorithm = dev.Data.Algorithm
			if dev.Data.Hashrate != nil {
				miner.Stats.Hashrate = *dev.Data.Hashrate
			}
			if len(dev.Data.Temperature) > 0 {
				miner.Stats.Temperature = dev.Data.Temperature[0]
			}
			if dev.Data.Wattage != nil {
				miner.Stats.Power = *dev.Data.Wattage
			}
			if dev.Data.Efficiency != nil {
				miner.Stats.Efficiency = *dev.Data.Efficiency
			}
			if dev.Data.Uptime != nil {
				miner.Uptime = *dev.Data.Uptime
			}
			miner.Stats.Errors = len(dev.Data.Errors)
			if len(dev.Data.FanSpeeds) > 0 {
				miner.Stats.FanSpeeds = dev.Data.FanSpeeds
			}
		}
		if existing, found := existingDevices[miner.ID]; found {
			miner.Stats.HashrateHistory = append([]float64(nil), existing.Stats.HashrateHistory...)
		}
		if miner.Status == "online" {
			miner.Stats.HashrateHistory = append(miner.Stats.HashrateHistory, miner.Stats.Hashrate)
		} else {
			miner.Stats.HashrateHistory = append(miner.Stats.HashrateHistory, 0.0)
		}
		const maxHistory = 288
		if len(miner.Stats.HashrateHistory) > maxHistory {
			miner.Stats.HashrateHistory = miner.Stats.HashrateHistory[len(miner.Stats.HashrateHistory)-maxHistory:]
		}
		newDevices = append(newDevices, miner)
	}
	d.State.Devices = newDevices

	if d.DeviceCountStr != nil {
		online := 0
		for _, m := range d.State.Devices {
			if m.Status == "online" {
				online++
			}
		}
		d.DeviceCountStr.Set(fmt.Sprintf("%d / %d", online, len(d.State.Devices)))
	}
}

func (d *DashboardApp) refreshData() {
	d.mu.Lock()
	d.State.LastUpdate.Set(time.Now().Format("15:04:05"))
	d.syncDevices()

	var totalHR float64
	var totalPower int
	for _, miner := range d.State.Devices {
		if miner.Status == "online" {
			totalHR += miner.Stats.Hashrate
			totalPower += miner.Stats.Power
			go func(ip string, hr float64) {
				safeName := strings.ReplaceAll(ip, ".", "_")
				safeName = strings.ReplaceAll(safeName, ":", "_")
				fname := filepath.Join("device_log", fmt.Sprintf("%s.csv", safeName))
				appendToCSV(fname, []string{time.Now().Format("2006-01-02 15:04:05"), strconv.FormatFloat(hr, 'f', 2, 64)})
			}(miner.ID, miner.Stats.Hashrate)
		}
	}
	d.State.TotalHashrate.Set(totalHR)

	go func(hr float64) {
		fname := filepath.Join("device_log", "total_hashrate.csv")
		appendToCSV(fname, []string{time.Now().Format("2006-01-02 15:04:05"), strconv.FormatFloat(hr, 'f', 2, 64)})
		if d.Chart != nil {
			d.Chart.UpdateFromCSV()
		}
	}(totalHR)

	d.updateSelectedDevice()
	d.State.UpdateDeviceCounts()
	d.mu.Unlock()

	if d.DeviceList != nil {
		fyne.Do(func() { d.DeviceList.Refresh() })
	}
}

func (d *DashboardApp) updateSelectedDevice() {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.State.Devices) == 0 {
		return
	}

	if d.State.SelectedIndex == -1 || d.State.SelectedIndex >= len(d.State.Devices) {
		var totalHR float64
		var totalPower int
		var maxLen int
		for _, miner := range d.State.Devices {
			if miner.Status == "online" {
				totalHR += miner.Stats.Hashrate
				totalPower += miner.Stats.Power
			}
			if len(miner.Stats.HashrateHistory) > maxLen {
				maxLen = len(miner.Stats.HashrateHistory)
			}
		}
		d.State.Hashrate.Set(totalHR)
		d.State.Power.Set(totalPower)

		var agg []float64
		if maxLen > 0 {
			agg = make([]float64, maxLen)
			for _, miner := range d.State.Devices {
				offset := maxLen - len(miner.Stats.HashrateHistory)
				for i, h := range miner.Stats.HashrateHistory {
					agg[offset+i] += h
				}
			}
		} else {
			agg = d.State.HashrateHistory
		}
		if d.Chart != nil {
			d.Chart.UpdateData(agg)
		}
		return
	}

	sel := d.State.Devices[d.State.SelectedIndex]
	d.State.Hashrate.Set(sel.Stats.Hashrate)
	d.State.Power.Set(sel.Stats.Power)
	d.State.SelectedHashrate.Set(sel.Stats.Hashrate)
	d.State.SelectedTemp.Set(sel.Stats.Temperature)
	d.State.SelectedPower.Set(sel.Stats.Power)
	d.State.SelectedEfficiency.Set(sel.Stats.Efficiency)
	d.State.SelectedErrors.Set(sel.Stats.Errors)

	if d.Chart != nil {
		d.Chart.UpdateData(sel.Stats.HashrateHistory)
	}
}

// =============================================================================
// Utilities
// =============================================================================

// appendToCSV appends a record row to a CSV file, writing a header on creation.
func appendToCSV(filename string, record []string) {
	fileInfo, err := os.Stat(filename)
	isNew := os.IsNotExist(err) || (err == nil && fileInfo.Size() == 0)

	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if isNew {
		_ = writer.Write([]string{"Time", "Hashrate"})
	}
	_ = writer.Write(record)
	writer.Flush()
}

// Ensure math/image are used (gauge pixel computation references them).
var _ = math.Pi
var _ = image.Point{}
