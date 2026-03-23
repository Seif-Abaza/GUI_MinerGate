// =============================================================================
// Package gui - Enhanced MinerGate Dashboard UI
// =============================================================================
// Professional Mining Dashboard with modern dark theme
// Features:
// - Circular gauge meters for hashrate
// - Performance charts with gradient fills
// - Status cards with color indicators
// - Device list with real-time updates
// - System statistics panel
// - Sidebar navigation
// =============================================================================
package gui

import (
	"fmt"
	"image/color"
	"math/rand"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
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
)

// =============================================================================
// Theme Colors - Modern Dark with Teal/Orange Accents
// =============================================================================

var (
	// Background colors
	colorBackground    = color.RGBA{R: 13, G: 13, B: 18, A: 255}  // #0d0d12
	colorSurface       = color.RGBA{R: 26, G: 26, B: 36, A: 255}  // #1a1a24
	colorSurfaceLight  = color.RGBA{R: 37, G: 37, B: 50, A: 255}  // #252532
	colorSidebar       = color.RGBA{R: 17, G: 17, B: 24, A: 255}  // #111118

	// Text colors
	colorTextPrimary   = color.RGBA{R: 228, G: 228, B: 231, A: 255} // #e4e4e7
	colorTextSecondary = color.RGBA{R: 113, G: 113, B: 122, A: 255} // #71717a
	colorTextMuted     = color.RGBA{R: 82, G: 82, B: 91, A: 255}    // #52525b

	// Accent colors
	colorTeal     = color.RGBA{R: 0, G: 229, B: 255, A: 255}    // #00e5ff
	colorTealDark = color.RGBA{R: 20, G: 184, B: 166, A: 255}   // #14b8a6
	colorOrange   = color.RGBA{R: 249, G: 115, B: 22, A: 255}   // #f97316

	// Status colors
	colorGreen  = color.RGBA{R: 34, G: 197, B: 94, A: 255}  // #22c55e
	colorYellow = color.RGBA{R: 234, G: 179, B: 8, A: 255}   // #eab308
	colorRed    = color.RGBA{R: 239, G: 68, B: 68, A: 255}   // #ef4444

	// Border
	colorBorder = color.RGBA{R: 39, G: 39, B: 42, A: 255} // #27272a
)

// =============================================================================
// MiningTheme - Custom Dark Theme
// =============================================================================

type MiningTheme struct{}

func (m MiningTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorBackground
	case theme.ColorNameButton:
		return colorSurface
	case theme.ColorNameDisabledButton:
		return colorSurfaceLight
	case theme.ColorNameInputBackground:
		return colorSurface
	case theme.ColorNameOverlayBackground:
		return colorSurface
	case theme.ColorNameDisabled:
		return colorTextSecondary
	case theme.ColorNameForeground:
		return colorTextPrimary
	case theme.ColorNamePlaceHolder:
		return colorTextSecondary
	case theme.ColorNamePressed:
		return colorSurfaceLight
	case theme.ColorNamePrimary:
		return colorTeal
	case theme.ColorNameHover:
		return colorSurfaceLight
	case theme.ColorNameFocus:
		return colorTeal
	case theme.ColorNameScrollBar:
		return colorSurfaceLight
	case theme.ColorNameSeparator:
		return colorBorder
	case theme.ColorNameShadow:
		return color.RGBA{A: 50}
	case theme.ColorNameSelection:
		return colorTeal
	case theme.ColorNameMenuBackground:
		return colorSurface
	case theme.ColorNameHeaderBackground:
		return colorSurface
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
		return 6
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 14
	case theme.SizeNameScrollBarSmall:
		return 6
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputBorder:
		return 2
	case theme.SizeNameInputRadius:
		return 4
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// =============================================================================
// AppState - Application State
// =============================================================================

type AppState struct {
	// Main metrics
	TotalHashrate  binding.Float
	TotalPower     binding.Int
	OnlineMiners   binding.Int
	OfflineMiners  binding.Int
	AvgTemp        binding.Float
	AvgEfficiency  binding.Float
	PoolLatency    binding.Int
	DailyRevenue   binding.Float
	MonthlyRevenue binding.Float
	LastUpdate     binding.String

	// Selected device metrics
	SelectedHashrate   binding.Float
	SelectedTemp       binding.Float
	SelectedPower      binding.Int
	SelectedEfficiency binding.Float

	// Devices
	Devices       []*models.Miner
	SelectedIndex int

	// Chart data
	HashrateHistory []float64

	// Settings
	AutoRefresh binding.Bool
	RefreshRate int
	FocusMode   bool

	// Managers
	APIClient     *api.Client
	FRPClient     *frp.Client
	GoASICManager *goasic.Manager
	PluginManager *plugins.Manager
	UpdateManager *update.Updater
	Config        *config.Config
}

// NewAppState creates a new application state
func NewAppState(cfg *config.Config) *AppState {
	return &AppState{
		TotalHashrate:      binding.NewFloat(),
		TotalPower:         binding.NewInt(),
		OnlineMiners:       binding.NewInt(),
		OfflineMiners:      binding.NewInt(),
		AvgTemp:            binding.NewFloat(),
		AvgEfficiency:      binding.NewFloat(),
		PoolLatency:        binding.NewInt(),
		DailyRevenue:       binding.NewFloat(),
		MonthlyRevenue:     binding.NewFloat(),
		LastUpdate:         binding.NewString(),
		SelectedHashrate:   binding.NewFloat(),
		SelectedTemp:       binding.NewFloat(),
		SelectedPower:      binding.NewInt(),
		SelectedEfficiency: binding.NewFloat(),
		AutoRefresh:        binding.NewBool(),
		RefreshRate:        cfg.RefreshRate,
		Config:             cfg,
		Devices:            make([]*models.Miner, 0),
		SelectedIndex:      -1,
		HashrateHistory:    make([]float64, 24),
	}
}

// SetDefaults sets default demo values
func (s *AppState) SetDefaults() {
	s.TotalHashrate.Set(496.8)
	s.TotalPower.Set(14867)
	s.OnlineMiners.Set(6)
	s.OfflineMiners.Set(2)
	s.AvgTemp.Set(72.5)
	s.AvgEfficiency.Set(26.5)
	s.PoolLatency.Set(25)
	s.DailyRevenue.Set(45.67)
	s.MonthlyRevenue.Set(1370.10)
	s.LastUpdate.Set(time.Now().Format("3:04:05 PM"))

	s.SelectedHashrate.Set(122.1)
	s.SelectedTemp.Set(74.2)
	s.SelectedPower.Set(2626)
	s.SelectedEfficiency.Set(26.9)

	s.AutoRefresh.Set(true)

	// Initialize hashrate history
	for i := range s.HashrateHistory {
		s.HashrateHistory[i] = 480 + rand.Float64()*40
	}
}

// =============================================================================
// DashboardApp - Main Application
// =============================================================================

type DashboardApp struct {
	App               fyne.App
	Window            fyne.Window
	State             *AppState
	Chart             *ChartWidget
	Ticker            *time.Ticker
	StopChan          chan bool
	mu                sync.RWMutex
	DeviceList        *widget.List
	DeviceCountStr    binding.String
	SelectedNavItem   string
	MinersBadge       *canvas.Text
	OnlineCountLabel  *canvas.Text
	OfflineCountLabel *canvas.Text
	DevicePanel       *fyne.Container
	MainContent       *fyne.Container
	DevicePanelShown  bool
	lastSelectedID    widget.ListItemID
	lastTapTime       time.Time
}

// NewDashboard creates a new dashboard
func NewDashboard(cfg *config.Config, apiClient *api.Client, frpClient *frp.Client,
	goasicMgr *goasic.Manager, pluginMgr *plugins.Manager, updateMgr *update.Updater) *DashboardApp {

	// Create Fyne app
	a := app.NewWithID("io.minergate.dashboard")
	a.Settings().SetTheme(&MiningTheme{})

	// Create window
	w := a.NewWindow(fmt.Sprintf("MinerGate Dashboard v%s", config.Version))
	w.Resize(fyne.NewSize(float32(cfg.WindowWidth), float32(cfg.WindowHeight)))
	w.CenterOnScreen()

	// Create state
	state := NewAppState(cfg)
	state.APIClient = apiClient
	state.FRPClient = frpClient
	state.GoASICManager = goasicMgr
	state.PluginManager = pluginMgr
	state.UpdateManager = updateMgr
	state.SetDefaults()

	app := &DashboardApp{
		App:              a,
		Window:           w,
		State:            state,
		StopChan:         make(chan bool),
		SelectedNavItem:  "Dashboard",
		DevicePanelShown: false,
	}

	// Setup callbacks for device discovery
	if goasicMgr != nil {
		goasicMgr.OnDeviceDiscovered(func(device *goasic.DiscoveredDevice) {
			go app.refreshData()
		})
		goasicMgr.OnDeviceUpdate(func(device *goasic.DiscoveredDevice) {
			go app.refreshData()
		})
		goasicMgr.OnDeviceLost(func(ip string) {
			go app.refreshData()
		})
	}

	return app
}

// Run starts the application
func (d *DashboardApp) Run() {
	content := d.buildUI()
	d.Window.SetContent(content)
	d.startAutoRefresh()

	d.Window.SetCloseIntercept(func() {
		d.StopChan <- true
		d.App.Quit()
	})

	d.Window.ShowAndRun()
}

// =============================================================================
// UI Building
// =============================================================================

func (d *DashboardApp) buildUI() fyne.CanvasObject {
	// Sidebar
	sidebar := d.createSidebar()

	// Device list panel
	devicePanel := d.createDevicePanel()
	d.DevicePanel = container.NewMax(devicePanel)
	d.DevicePanelShown = false
	d.DevicePanel.Hide()

	// Main content
	mainContent := d.createMainContent()
	d.MainContent = container.NewMax(mainContent)

	centerSplit := container.NewHSplit(d.DevicePanel, d.MainContent)
	centerSplit.SetOffset(0.18)

	// Split layout: Sidebar | Device Panel/Main Content
	split := container.NewHSplit(sidebar, centerSplit)
	split.SetOffset(0.08)

	return split
}

// createSidebar creates the left sidebar navigation
func (d *DashboardApp) createSidebar() fyne.CanvasObject {
	// Logo
	logoText := canvas.NewText("⬡ MinerGate", colorTeal)
	logoText.TextSize = 18
	logoText.TextStyle = fyne.TextStyle{Bold: true}

	versionText := canvas.NewText("v"+config.Version, colorTextMuted)
	versionText.TextSize = 10

	logoContainer := container.NewVBox(
		logoText,
		versionText,
	)

	// Navigation items
	navItems := []struct {
		name   string
		icon   string
		active bool
		badge  string
	}{
		{"Dashboard", "📊", true, ""},
		{"Miners", "⛏️", false, "0"},
		{"Performance", "📈", false, ""},
		{"Analytics", "📉", false, ""},
		{"Power", "⚡", false, ""},
		{"Network", "🌐", false, ""},
		{"Alerts", "🔔", false, ""},
	}

	navContainer := container.NewVBox()
	for _, item := range navItems {
		navItem := d.createNavItem(item.name, item.icon, item.active, item.badge)
		navContainer.Add(navItem)
	}

	// Bottom items
	bottomItems := container.NewVBox(
		d.createNavItem("Settings", "⚙️", false, ""),
		d.createNavItem("Help", "❓", false, ""),
	)

	// Container
	content := container.NewBorder(
		container.NewPadded(logoContainer),
		container.NewPadded(bottomItems),
		nil, nil,
		container.NewPadded(navContainer),
	)

	// Background
	bg := canvas.NewRectangle(colorSidebar)
	bg.SetMinSize(fyne.NewSize(180, 0))

	return container.NewStack(bg, container.NewPadded(content))
}

// createNavItem creates a navigation item
func (d *DashboardApp) createNavItem(name, icon string, active bool, badge string) fyne.CanvasObject {
	button := widget.NewButton("", func() {
		d.handleNavSelection(name)
	})
	button.Importance = widget.LowImportance

	var textColor color.Color
	var bgColor color.Color

	if active {
		textColor = colorTeal
		bgColor = color.RGBA{R: 0, G: 229, B: 255, A: 25}
	} else {
		textColor = colorTextSecondary
		bgColor = colorTransparent()
	}

	label := canvas.NewText(fmt.Sprintf("%s  %s", icon, name), textColor)
	label.TextSize = 13

	content := container.NewHBox(label)

	if badge != "" {
		badgeLabel := canvas.NewText(badge, colorTeal)
		badgeLabel.TextSize = 10
		badgeLabel.TextStyle = fyne.TextStyle{Bold: true}
		if name == "Miners" {
			d.MinersBadge = badgeLabel
		}
		content.Add(layout.NewSpacer())
		content.Add(badgeLabel)
	}

	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 6

	button.SetText("")
	buttonContainer := container.NewStack(bg, container.NewPadded(content), button)
	return buttonContainer
}

// createDevicePanel creates the device list panel
func (d *DashboardApp) createDevicePanel() fyne.CanvasObject {
	title := canvas.NewText("⚙ ASIC Miners", colorTeal)
	title.TextSize = 16
	title.TextStyle = fyne.TextStyle{Bold: true}

	d.OnlineCountLabel = canvas.NewText("● Online: 0", colorGreen)
	d.OnlineCountLabel.TextSize = 11
	d.OfflineCountLabel = canvas.NewText("● Offline: 0", colorRed)
	d.OfflineCountLabel.TextSize = 11

	statusRow := container.NewHBox(
		d.OnlineCountLabel,
		d.OfflineCountLabel,
	)

	search := widget.NewEntry()
	search.SetPlaceHolder("Search devices...")
	search.Resize(fyne.NewSize(100, 36))

	deviceList := d.createDeviceList()
	d.DeviceList = deviceList

	header := container.NewVBox(
		container.NewHBox(title, layout.NewSpacer()),
		statusRow,
		search,
		widget.NewSeparator(),
	)

	content := container.NewBorder(header, nil, nil, nil, deviceList)

	bg := canvas.NewRectangle(colorSidebar)
	bg.SetMinSize(fyne.NewSize(150, 0))

	panel := container.NewStack(bg, container.NewPadded(content))
	panel.Resize(fyne.NewSize(150, 0))

	return panel
}

// createDeviceList creates the device list widget
func (d *DashboardApp) createDeviceList() *widget.List {
	list := widget.NewList(
		func() int {
			d.mu.RLock()
			defer d.mu.RUnlock()
			return len(d.State.Devices)
		},
		func() fyne.CanvasObject {
			return container.NewBorder(
				nil, nil, nil,
				container.NewHBox(
					canvas.NewText("● Online", colorGreen),
				),
				container.NewVBox(
					canvas.NewText("Device Name", colorTextPrimary),
					canvas.NewText("Model", colorTextMuted),
				),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			d.mu.RLock()
			if int(id) >= len(d.State.Devices) {
				d.mu.RUnlock()
				return
			}
			device := d.State.Devices[id]
			d.mu.RUnlock()

			border := item.(*fyne.Container)
			leftBox := border.Objects[0].(*fyne.Container)
			rightBox := border.Objects[1].(*fyne.Container)

			nameLabel := leftBox.Objects[0].(*canvas.Text)
			nameLabel.Text = d.deviceDisplayName(device)

			modelLabel := leftBox.Objects[1].(*canvas.Text)
			modelLabel.Text = device.Model

			statusLabel := rightBox.Objects[0].(*canvas.Text)
			if device.Status == "online" {
				statusLabel.Text = "● Online"
				statusLabel.Color = colorGreen
			} else {
				statusLabel.Text = "● Offline"
				statusLabel.Color = colorRed
			}
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		now := time.Now()
		if d.lastSelectedID == id && now.Sub(d.lastTapTime) <= 500*time.Millisecond {
			d.renameDevice(id)
		}
		d.lastSelectedID = id
		d.lastTapTime = now
		d.State.SelectedIndex = id
	}
	list.OnUnselected = func(id widget.ListItemID) {}

	return list
}

// createMainContent creates the main dashboard content
func (d *DashboardApp) createMainContent() fyne.CanvasObject {
	// Header
	header := d.createHeader()

	// Gauges section
	gauges := d.createGaugesSection()

	// Status cards
	statusCards := d.createStatusCards()

	// Charts section
	charts := d.createChartsSection()

	// Metrics row

	// System stats
	systemStats := d.createSystemStats()

	// Stack vertically
	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		gauges,
		statusCards,
		widget.NewSeparator(),
		charts,
		widget.NewSeparator(),
		systemStats,
	)

	// Scroll container
	scroll := container.NewScroll(content)

	// Background
	bg := canvas.NewRectangle(colorBackground)

	return container.NewStack(bg, container.NewPadded(scroll))
}

// createHeader creates the top header
func (d *DashboardApp) createHeader() fyne.CanvasObject {
	title := canvas.NewText("Mining Dashboard", colorTextPrimary)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}

	farmInfo := canvas.NewText("◉ Farm UUID: "+d.State.Config.FarmUUID, colorTextSecondary)
	farmInfo.TextSize = 11

	lastUpdate := widget.NewLabelWithData(d.State.LastUpdate)
	lastUpdate.TextStyle = fyne.TextStyle{Italic: true}

	return container.NewBorder(
		nil, nil,
		container.NewVBox(title, farmInfo),
		container.NewVBox(
			canvas.NewText("Last Update", colorTextMuted),
			lastUpdate,
		),
		nil,
	)
}

// createGaugesSection creates the circular gauges
func (d *DashboardApp) createGaugesSection() fyne.CanvasObject {
	// Main hashrate gauge
	hashrateGauge := NewCircularGauge(0, 0, "TH/s", "Total Hashrate", 0, colorTeal)

	// Temperature gauge
	tempGauge := NewCircularGauge(0, 0, "°C", "Avg Temperature", 0, colorOrange)

	// Efficiency gauge
	effGauge := NewCircularGauge(0, 0, "J/TH", "Efficiency", 0, colorGreen)

	// Container with cards
	mainCard := d.createCard(container.NewCenter(hashrateGauge))
	tempCard := d.createCard(container.NewCenter(tempGauge))
	effCard := d.createCard(container.NewCenter(effGauge))

	return container.NewGridWithColumns(4,
		container.NewGridWithRows(1, mainCard),
		container.NewGridWithRows(2, tempCard, effCard),
	)
}

// createStatusCards creates the status indicator cards
func (d *DashboardApp) createStatusCards() fyne.CanvasObject {
	powerCard := NewStatusCard("Total Power", "0", "W", StatusNeutral)
	powerCard.SetTrend("stable", "")

	onlineCard := NewStatusCard("Online Miners", "0", "", StatusSuccess)
	onlineCard.SetTrend("up", "+2 from yesterday")

	offlineCard := NewStatusCard("Offline Miners", "0", "", StatusDanger)
	offlineCard.SetTrend("down", "-1 from yesterday")

	latencyCard := NewStatusCard("Pool Latency", "0", "ms", StatusSuccess)

	return container.NewGridWithColumns(4,
		powerCard,
		onlineCard,
		offlineCard,
		latencyCard,
	)
}

// createChartsSection creates the performance charts
func (d *DashboardApp) createChartsSection() fyne.CanvasObject {
	// Hashrate chart
	hashrateChart := d.createHashrateChart()
	
	// Temperature chart
	tempChart := d.createTempChart()

	return container.NewGridWithColumns(2,
		d.createCardWithTitle("📊 Hashrate History (24h)", hashrateChart),
		d.createCardWithTitle("🌡️ Temperature & Power", tempChart),
	)
}

// createHashrateChart creates the hashrate line chart
func (d *DashboardApp) createHashrateChart() fyne.CanvasObject {
	// Time range buttons
	range1D := widget.NewButton("1D", nil)
	range1D.Importance = widget.HighImportance
	range1W := widget.NewButton("1W", nil)

	timeRange := container.NewHBox(
		widget.NewButton("<", nil),
		widget.NewLabel("Today"),
		widget.NewButton(">", nil),
		layout.NewSpacer(),
		range1D,
		range1W,
	)

	// Chart widget
	chartWidget := d.createChartWidget()

	return container.NewBorder(timeRange, nil, nil, nil, chartWidget)
}

// createTempChart creates the temperature chart
func (d *DashboardApp) createTempChart() fyne.CanvasObject {
	// Legend
	legend := container.NewHBox(
		canvas.NewText("— Hashrate", colorTeal),
		canvas.NewText("— Temperature", colorOrange),
	)

	// Placeholder chart
	chartPlaceholder := canvas.NewText("📈 Multi-line chart will render here", colorTextMuted)
	chartPlaceholder.Alignment = fyne.TextAlignCenter

	return container.NewBorder(legend, nil, nil, nil, container.NewCenter(chartPlaceholder))
}

// createSystemStats creates the system statistics panel
func (d *DashboardApp) createSystemStats() fyne.CanvasObject {
	title := canvas.NewText("🖥️ System Statistics", colorTextSecondary)
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Stats grid
	stats := container.NewGridWithColumns(3,
		d.createStatItem("Avg Temp", "0°C", "Max: 0°C", colorOrange),
		d.createStatItem("Total Power", "0 W", "Avg: 0W/miner", colorTeal),
		d.createStatItem("Pool Status", "Connected", "Latency: 0ms", colorGreen),
		d.createStatItem("Uptime", "0d 0h", "0/0 miners active", colorGreen),
		d.createStatItem("Network HR", "0.0 EH/s", "Diff: 0 T", colorTeal),
		d.createStatItem("Efficiency", "0%", "", colorTeal),
	)

	content := container.NewVBox(
		title,
		widget.NewSeparator(),
		stats,
	)

	return d.createCard(content)
}

// createStatItem creates a stat display item
func (d *DashboardApp) createStatItem(label, value, subtitle string, valueColor color.Color) fyne.CanvasObject {
	labelText := canvas.NewText(label, colorTextMuted)
	labelText.TextSize = 11

	valueText := canvas.NewText(value, valueColor)
	valueText.TextSize = 18
	valueText.TextStyle = fyne.TextStyle{Bold: true}

	subtitleText := canvas.NewText(subtitle, colorTextMuted)
	subtitleText.TextSize = 10

	return container.NewVBox(
		labelText,
		valueText,
		subtitleText,
	)
}

// createCard creates a card container
func (d *DashboardApp) createCard(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colorSurface)
	bg.CornerRadius = 8
	bg.StrokeColor = colorBorder
	bg.StrokeWidth = 1

	return container.NewStack(bg, container.NewPadded(content))
}

// createCardWithTitle creates a card with title
func (d *DashboardApp) createCardWithTitle(title string, content fyne.CanvasObject) fyne.CanvasObject {
	titleText := canvas.NewText(title, colorTextSecondary)
	titleText.TextSize = 14
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	header := container.NewHBox(titleText)

	return d.createCard(container.NewBorder(header, nil, nil, nil, content))
}

// =============================================================================
// Chart Widget
// =============================================================================

type ChartWidget struct {
	widget.BaseWidget
	data     []float64
	inner    *fyne.Container
	appState *AppState
	csvPath  string
}

func (d *DashboardApp) createChartWidget() fyne.CanvasObject {
	chart := &ChartWidget{
		data:     d.State.HashrateHistory,
		appState: d.State,
	}

	// Build initial chart
	var initialObj fyne.CanvasObject
	if len(chart.data) > 0 {
		if gw := charts.BuildGraphWidget(chart.data, "Hashrate (TH/s)"); gw != nil {
			initialObj = gw
		}
	}
	if initialObj == nil {
		placeholder := canvas.NewText("⏳ Waiting for data...", colorTextSecondary)
		placeholder.TextSize = 13
		placeholder.TextStyle = fyne.TextStyle{Italic: true}
		initialObj = placeholder
	}

	chart.inner = container.NewStack(initialObj)
	chart.ExtendBaseWidget(chart)
	d.Chart = chart

	return chart
}

func (c *ChartWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.inner)
}

func (c *ChartWidget) UpdateData(newData []float64) {
	fyne.Do(func() {
		var newObj fyne.CanvasObject
		if len(newData) > 0 {
			if gw := charts.BuildGraphWidget(newData, "Hashrate (TH/s)"); gw != nil {
				newObj = gw
			}
		}
		if newObj == nil {
			placeholder := canvas.NewText("⏳ Waiting for data...", colorTextSecondary)
			placeholder.TextSize = 13
			newObj = placeholder
		}

		c.inner.Objects = []fyne.CanvasObject{newObj}
		c.inner.Refresh()
	})
}

// =============================================================================
// Data Management
// =============================================================================

func (d *DashboardApp) startAutoRefresh() {
	interval := time.Duration(d.State.RefreshRate) * time.Second
	d.Ticker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-d.StopChan:
				d.Ticker.Stop()
				return
			case <-d.Ticker.C:
				d.refreshData()
			}
		}
	}()
}

func (d *DashboardApp) refreshData() {
	d.mu.Lock()
	defer d.mu.Unlock()

	currentHR, _ := d.State.TotalHashrate.Get()
	newHR := currentHR + (rand.Float64()*10 - 5)
	if newHR < 450 {
		newHR = 450
	}
	if newHR > 550 {
		newHR = 550
	}

	// if len(d.State.Devices) == 0 {
	// 	d.State.Devices = append(d.State.Devices,
	// 		&models.Miner{
	// 			ID:        "miner-1",
	// 			Name:      "Miner-01",
	// 			Model:     "Antminer S19 Pro",
	// 			IPAddress: "192.168.1.101",
	// 			Status:    "online",
	// 			Config:    models.MinerConfig{Hostname: "miner-01"},
	// 		},
	// 		&models.Miner{
	// 			ID:        "miner-2",
	// 			Name:      "Miner-02",
	// 			Model:     "Whatsminer M50",
	// 			IPAddress: "192.168.1.102",
	// 			Status:    "offline",
	// 			Config:    models.MinerConfig{Hostname: "miner-02"},
	// 		},
	// 	)
	// }

	if rand.Intn(3) == 0 {
		newMinerIndex := len(d.State.Devices) + 1
		newMiner := &models.Miner{
			ID:        fmt.Sprintf("miner-%d", newMinerIndex),
			Name:      fmt.Sprintf("Miner-%02d", newMinerIndex),
			Model:     "Antminer S21",
			IPAddress: fmt.Sprintf("192.168.1.%d", 100+newMinerIndex),
			Status:    "online",
			Config:    models.MinerConfig{Hostname: fmt.Sprintf("miner-%02d", newMinerIndex)},
		}
		d.State.Devices = append(d.State.Devices, newMiner)
	}

	online := 0
	offline := 0
	for _, device := range d.State.Devices {
		if device.Status == "online" {
			online++
		} else {
			offline++
		}
	}

	latency := 15 + rand.Intn(30)
	newHistory := append(d.State.HashrateHistory[1:], newHR)
	lastUpdate := time.Now().Format("3:04:05 PM")
	deviceCount := len(d.State.Devices)

	fyne.Do(func() {
		d.State.TotalHashrate.Set(newHR)
		d.State.PoolLatency.Set(latency)
		d.State.HashrateHistory = newHistory
		d.State.OnlineMiners.Set(online)
		d.State.OfflineMiners.Set(offline)
		if d.Chart != nil {
			d.Chart.UpdateData(d.State.HashrateHistory)
		}
		if d.DeviceList != nil {
			d.DeviceList.Refresh()
		}
		d.updateDevicePanelCounts(online, offline)
		d.updateMinersBadge(deviceCount)
		d.State.LastUpdate.Set(lastUpdate)
	})
}

// Helper function
func (d *DashboardApp) handleNavSelection(name string) {
	d.SelectedNavItem = name
	if name == "Miners" {
		d.DevicePanelShown = true
		if d.DevicePanel != nil {
			d.DevicePanel.Show()
			d.DevicePanel.Refresh()
		}
		d.mu.RLock()
		online := 0
		offline := 0
		for _, device := range d.State.Devices {
			if device.Status == "online" {
				online++
			} else {
				offline++
			}
		}
		count := len(d.State.Devices)
		d.mu.RUnlock()
		d.updateDevicePanelCounts(online, offline)
		d.updateMinersBadge(count)
		if d.DeviceList != nil {
			d.DeviceList.Refresh()
		}
	} else {
		d.DevicePanelShown = false
		if d.DevicePanel != nil {
			d.DevicePanel.Hide()
		}
	}
}

func (d *DashboardApp) updateMinersBadge(count int) {
	if d.MinersBadge != nil {
		d.MinersBadge.Text = fmt.Sprintf("%d", count)
		d.MinersBadge.Refresh()
	}
}

func (d *DashboardApp) updateDevicePanelCounts(online, offline int) {
	if d.OnlineCountLabel != nil {
		d.OnlineCountLabel.Text = fmt.Sprintf("● Online: %d", online)
		d.OnlineCountLabel.Refresh()
	}
	if d.OfflineCountLabel != nil {
		d.OfflineCountLabel.Text = fmt.Sprintf("● Offline: %d", offline)
		d.OfflineCountLabel.Refresh()
	}
}

func (d *DashboardApp) deviceDisplayName(device *models.Miner) string {
	if device == nil {
		return ""
	}
	if strings.TrimSpace(device.Config.Hostname) != "" {
		return device.Config.Hostname
	}
	if strings.TrimSpace(device.IPAddress) != "" {
		return device.IPAddress
	}
	return device.Name
}

func (d *DashboardApp) renameDevice(id widget.ListItemID) {
	d.mu.RLock()
	if int(id) < 0 || int(id) >= len(d.State.Devices) {
		d.mu.RUnlock()
		return
	}
	device := d.State.Devices[id]
	currentName := d.deviceDisplayName(device)
	d.mu.RUnlock()

	entry := widget.NewEntry()
	entry.SetText(currentName)

	dialog.ShowForm(
		"Rename Device",
		"Save",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", entry),
		},
		func(confirm bool) {
			if !confirm {
				return
			}
			newName := strings.TrimSpace(entry.Text)
			if newName == "" {
				return
			}
			fyne.Do(func() {
				d.mu.Lock()
				device.Name = newName
				device.Config.Hostname = newName
				d.mu.Unlock()
				if d.DeviceList != nil {
					d.DeviceList.Refresh()
				}
			})
		},
		d.Window,
	)
}

func colorTransparent() color.Color {
	return color.RGBA{A: 0}
}
