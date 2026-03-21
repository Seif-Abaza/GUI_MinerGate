// =============================================================================
// Package gui - واجهة المستخدم الرسومية
// =============================================================================
// لوحة تحكم التعدين الاحترافية المبنية باستخدام Fyne
// تتضمن:
// - عرض الأجهزة والمقاييس
// - رسوم بيانية تفاعلية
// - دعم متجاوب للأحجام المختلفة
// - التحديث التلقائي
// - أزرار التحكم في الأجهزة
// =============================================================================
package gui

import (
	"encoding/csv"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
)

// =============================================================================
// ألوان السمة
// =============================================================================

// Theme colors (inspired by Catppuccin Mocha palette)
var (
	// Background colors
	colorBackground  = color.RGBA{R: 30, G: 30, B: 46, A: 255} // #1e1e2e
	colorSurface     = color.RGBA{R: 49, G: 50, B: 68, A: 255} // #313244
	colorSurfaceHigh = color.RGBA{R: 69, G: 71, B: 90, A: 255} // #45475a

	// Text colors
	colorTextPrimary   = color.RGBA{R: 205, G: 214, B: 244, A: 255} // #cdd6f4
	colorTextSecondary = color.RGBA{R: 166, G: 173, B: 200, A: 255} // #a6adc8

	// Accent colors
	colorYellow = color.RGBA{R: 249, G: 226, B: 175, A: 255} // #f9e2af
	colorBlue   = color.RGBA{R: 137, G: 180, B: 250, A: 255} // #89b4fa
	colorGreen  = color.RGBA{R: 166, G: 227, B: 161, A: 255} // #a6e3a1
	colorRed    = color.RGBA{R: 243, G: 139, B: 168, A: 255} // #f38ba8
	colorOrange = color.RGBA{R: 250, G: 179, B: 135, A: 255} // #fab387
	colorPurple = color.RGBA{R: 203, G: 166, B: 247, A: 255} // #cba6f7
)

// =============================================================================
// سمة التعدين
// =============================================================================

// MiningTheme سمة التعدين المخصصة
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
		return colorSurfaceHigh
	case theme.ColorNamePrimary:
		return colorBlue
	case theme.ColorNameHover:
		return colorSurfaceHigh
	case theme.ColorNameFocus:
		return colorBlue
	case theme.ColorNameScrollBar:
		return colorSurfaceHigh
	case theme.ColorNameSeparator:
		return colorSurface
	case theme.ColorNameShadow:
		return color.RGBA{A: 50}
	case theme.ColorNameSelection:
		return colorBlue
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
// حالة التطبيق
// =============================================================================

// AppState حالة التطبيق
// هذا الهيكل يحمل جميع البيانات والمقاييس التي تُعرض وتُحدَّث في الواجهة.
// يتم استخدام binding حتى يتغيّر العرض تلقائياً عند تغيير القيم.
type AppState struct {
	// المقاييس الرئيسية
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

	// مقاييس الجهاز المحدد
	SelectedHashrate   binding.Float
	SelectedTemp       binding.Float
	SelectedPower      binding.Int
	SelectedUptime     binding.Float
	SelectedEfficiency binding.Float
	SelectedErrors     binding.Int

	// الأجهزة
	Devices       []*models.Miner
	Workers       []Worker
	SelectedIndex int

	// الرسوم البيانية
	HashrateChart  *charts.HashrateChartWidget
	TempPowerChart *charts.TempPowerChartWidget
	// Chart data
	HashrateHistory []float64
	// الإعدادات
	AutoRefresh binding.Bool
	RefreshRate int

	// مديري المكونات
	APIClient     *api.Client
	FRPClient     *frp.Client
	GoASICManager *goasic.Manager
	PluginManager *plugins.Manager
	UpdateManager *update.Updater

	// التكوين
	Config *config.Config
}

// Worker represents a worker entry in the UI table.
type Worker struct {
	Name       string
	Subaccount string
	Hashrate   float64
	Efficiency float64
	Status     string
}

// NewAppState ينشئ حالة جديدة
// يعيد بنية AppState مع تهيئة كل الحقول كـ bindings جاهزة للتحديث.
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

// SetDefaults يضبط القيم الافتراضية
// يُستخدم لتعبئة العرض ببيانات تجريبية/افتراضية عند بدء التشغيل.
func (s *AppState) SetDefaults() {
	s.OnlineDevices.Set(6)
	s.TotalDevices.Set(6)
	s.TotalHashrate.Set(496.8)
	s.Power.Set(14867)
	s.DailyBTC.Set(0.00027945)
	s.DailyProfit.Set(-16.96)
	s.MonthlyProfit.Set(-508.73)
	s.LastUpdate.Set(time.Now().Format("3:06:09 PM"))

	s.SelectedHashrate.Set(122.1)
	s.SelectedTemp.Set(74.2)
	s.SelectedPower.Set(2626)
	s.SelectedUptime.Set(38.5)
	s.SelectedEfficiency.Set(26.9)
	s.SelectedErrors.Set(2)

	// Additional dashboard metrics
	s.Hashrate.Set(518.4)
	s.ActiveWorkers.Set(6)
	s.Efficiency.Set(26.5)
	s.Power.Set(14867)
	s.Balance.Set(0.00234)
	s.Revenue24h.Set(12.50)
	s.Uptime.Set(99.7)
	s.Hashprice.Set(0.00000453)

	s.AutoRefresh.Set(true)

	// Compute offline devices based on demo data (will be recalculated after devices are loaded)
	// This is updated again after populating s.Devices.

	// Sample workers (for the workers table)
	s.Workers = []Worker{
		{Name: "Worker-01", Subaccount: "main", Hashrate: 78.5, Efficiency: 95.2, Status: "online"},
		{Name: "Worker-02", Subaccount: "backup", Hashrate: 74.3, Efficiency: 92.7, Status: "online"},
		{Name: "Worker-03", Subaccount: "test", Hashrate: 65.8, Efficiency: 90.1, Status: "offline"},
	}

	// Empty initial slice; will be populated by refreshData/syncDevices
	s.Devices = make([]*models.Miner, 0)

	// Update counts based on device status
	s.UpdateDeviceCounts()

	// Initialize sample hashrate history for the chart
	s.HashrateHistory = []float64{480, 492, 505, 499, 510, 518, 525, 520, 515, 522, 530, 527, 518, 512, 520, 526, 532, 528, 523, 519, 517, 514, 510, 508}
}

// UpdateDeviceCounts recalculates device counts and updates bindings.
// This helps keep the dashboard metrics in sync with the current device list.
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
// التطبيق الرئيسي
// =============================================================================

// DashboardApp التطبيق الرئيسي
// يحتوي على حالة التطبيق، نافذة Fyne، ومنطق التحديث التلقائي.
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
	// v1.0.4: سيرفر الرسم البياني التفاعلي (go-echarts)
	echartsServer *charts.EChartsServer
}

// NewDashboard ينشئ لوحة تحكم جديدة
// يتهيئ التطبيق، يربط مدراء الخدمات (API, FRP, Plugin, Update) ويهيئ الحالة.
func NewDashboard(cfg *config.Config, apiClient *api.Client, frpClient *frp.Client,
	goasicMgr *goasic.Manager, pluginMgr *plugins.Manager, updateMgr *update.Updater) *DashboardApp {

	// إنشاء تطبيق Fyne
	a := app.NewWithID("io.minergate.dashboard")
	a.Settings().SetTheme(&MiningTheme{})
	// إنشاء النافذة
	w := a.NewWindow(fmt.Sprintf("Mining Dashboard for NewUser — Performance | v%s", config.Version))
	w.Resize(fyne.NewSize(float32(cfg.WindowWidth), float32(cfg.WindowHeight)))
	w.SetFullScreen(false)
	w.CenterOnScreen()
	// إنشاء الحالة
	state := NewAppState(cfg)
	state.APIClient = apiClient
	state.FRPClient = frpClient
	state.GoASICManager = goasicMgr
	state.PluginManager = pluginMgr
	state.UpdateManager = updateMgr
	state.SetDefaults()

	app := &DashboardApp{
		App:      a,
		Window:   w,
		State:    state,
		StopChan: make(chan bool),
	}

	// إعداد Callbacks لتحديث قائمة الأجهزة فور اكتشافها
	// FIX (Bug 3): wrap in goroutines so they don't deadlock with refreshData's mutex.
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

	// v1.0.4: تشغيل سيرفر الرسم البياني التفاعلي (go-echarts)
	if srv, err := charts.NewEChartsServer(); err == nil {
		app.echartsServer = srv
		// تحميل أي بيانات CSV موجودة مسبقاً
		_ = srv.UpdateFromCSV(
			filepath.Join("device_log", "total_hashrate.csv"),
			"سجل إجمالي معدل التجزئة",
		)
	}

	return app
}

// Run يشغل التطبيق
// ينشئ واجهة المستخدم، يبدأ التحديث التلقائي ويعرض النافذة.
func (d *DashboardApp) Run() {
	content := d.buildUI(d.State.Config.ApplicationName, d.State.Config.FarmUUID)
	d.Window.SetContent(content)
	d.startAutoRefresh()

	// FIX (Bug 1): The first GoASIC scan can take many seconds (one probe per host
	// in the subnet). Wait until scanning is no longer active, then immediately
	// refresh so discovered devices appear without having to wait for the next
	// auto-refresh tick.
	if d.State.GoASICManager != nil {
		go func() {
			// Wait until the manager has started scanning (may not be immediate).
			for !d.State.GoASICManager.IsScanning() {
				time.Sleep(200 * time.Millisecond)
			}
			// Now wait for the first scan to finish.
			for d.State.GoASICManager.IsScanning() {
				time.Sleep(500 * time.Millisecond)
			}
			// Trigger a refresh as soon as the scan results are available.
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
// بناء الواجهة
// =============================================================================

// buildUI يبني واجهة المستخدم
// يجمع قائمة الأجهزة، المحتوى الرئيسي، الرأس والتذييل في تخطيط واحد.
func (d *DashboardApp) buildUI(application_name string, farm_uuid string) fyne.CanvasObject {

	deviceList := d.createDeviceList()
	content := d.createMainContent()
	// Create horizontal split
	split := container.NewHSplit(deviceList, content)
	split.SetOffset(0.15) // Device list takes 15% of width

	// Wrap in border layout with header
	header := d.createHeader(application_name, farm_uuid)
	footer := d.createFooter()

	mainContainer := container.NewBorder(
		header, // top
		footer, // bottom
		nil,    // left
		nil,    // right
		split,  // center
	)

	return mainContainer
}

// createHeader creates the top header bar
// يعرض الشعار والتبويبات وحالة الـ workspace في أعلى التطبيق.
func (m *DashboardApp) createHeader(application_name string, farm_uuid string) fyne.CanvasObject {
	// Logo
	logoText := canvas.NewText("⬡ "+application_name, colorYellow)
	logoText.TextSize = 20
	logoText.TextStyle = fyne.TextStyle{Bold: true}

	// Tabs
	// tabs := container.NewHBox(
	// 	m.createTab("Mining", true),
	// 	m.createTab("Reports", false),
	// 	m.createTab("Subaccounts", false),
	// )

	// Workspace info
	workspaceText := canvas.NewText("◉ Farm UUID: "+farm_uuid, colorTextSecondary)
	workspaceText.TextSize = 12

	// Header container - وضع الشعار والمزرعة في نفس الصف مع مسافة شفافة
	header := container.NewBorder(
		container.NewHBox(logoText, layout.NewSpacer(), workspaceText), // top: شعار + مسافة شفافة + رقم المزرعة
		nil, // bottom
		nil, // left
		nil, // right
		canvas.NewRectangle(colorSurface),
	)

	// Add padding
	return container.NewPadded(header)
}

// createTab creates a clickable tab
// يستخدم في الشريط العلوي للتنقل بين الصفحات (محاكاة علامات التبويب).
func (m *DashboardApp) createTab(text string, active bool) fyne.CanvasObject {
	var col color.Color
	if active {
		col = colorYellow
	} else {
		col = colorTextSecondary
	}

	label := canvas.NewText(text, col)
	label.TextSize = 14
	if active {
		label.TextStyle = fyne.TextStyle{Bold: true}
	}

	return container.NewPadded(label)
}

// sin دالة جيبية مبسطة
func sin(x float64) float64 {
	return x - (x*x*x)/6 + (x*x*x*x*x)/120
}

// createDeviceList creates a list of devices in the sidebar area
// يعرض قائمة بالأجهزة وعند الضغط على جهاز يحدث البيانات المعروضة.
func (d *DashboardApp) createDeviceList() fyne.CanvasObject {
	// Title
	title := canvas.NewText("⚙ Devices", colorBlue)
	title.TextSize = 16
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Device count using data binding for thread safety
	d.DeviceCountStr = binding.NewString()
	d.DeviceCountStr.Set(fmt.Sprintf("Total: %d", len(d.State.Devices)))

	deviceCount := widget.NewLabelWithData(d.DeviceCountStr)
	deviceCount.TextStyle = fyne.TextStyle{Bold: false}

	header := container.NewHBox(title, layout.NewSpacer(), deviceCount)

	// Device list
	list := widget.NewList(
		func() int {
			d.mu.RLock()
			defer d.mu.RUnlock()
			return len(d.State.Devices)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.ComputerIcon()),
				canvas.NewText("Device Name", colorTextPrimary),
				layout.NewSpacer(),
				canvas.NewText("online", colorGreen),
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

			hbox := item.(*fyne.Container)

			// Update name
			nameLabel := hbox.Objects[1].(*canvas.Text)
			nameLabel.Text = device.Name
			nameLabel.Refresh()

			// Update status
			statusLabel := hbox.Objects[3].(*canvas.Text)
			if device.Status == "online" {
				statusLabel.Text = "● Online"
				statusLabel.Color = colorGreen
			} else {
				statusLabel.Text = "● Offline"
				statusLabel.Color = colorRed
			}
			statusLabel.Refresh()
		},
	)

	// Handle selection
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

	// Container
	content := container.NewBorder(header, nil, nil, nil, list)

	// Background
	bg := canvas.NewRectangle(colorBackground)
	bg.SetMinSize(fyne.NewSize(250, 0))

	return container.NewStack(bg, container.NewPadded(content))
}

// createMenuItem creates a sidebar menu item
// ينشئ عنصر في الشريط الجانبي مع أيقونة ونص وحالة نشطة.
func (m *DashboardApp) createMenuItem(name, icon, badge string, active bool) fyne.CanvasObject {
	var bgColor color.Color
	var textColor color.Color

	if active {
		bgColor = colorSurface
		textColor = colorYellow
	} else {
		bgColor = colorBackground
		textColor = colorTextSecondary
	}

	// Icon + name
	label := canvas.NewText(fmt.Sprintf("%s %s", icon, name), textColor)
	label.TextSize = 13

	content := container.NewHBox(label)

	// Add badge if present
	if badge != "" {
		badgeLabel := canvas.NewText(badge, colorGreen)
		badgeLabel.TextSize = 10
		badgeLabel.TextStyle = fyne.TextStyle{Bold: true}
		content.Add(badgeLabel)
	}

	// Background
	bg := canvas.NewRectangle(bgColor)

	return container.NewStack(bg, container.NewPadded(content))
}

// createMainContent creates the main content area
// يجمع مقاييس الأداء، الرسم البياني، وجدول العمال معاً في المحتوى الرئيسي.
func (m *DashboardApp) createMainContent() fyne.CanvasObject {
	// Metrics grid
	metrics := m.createMetricsGrid()

	// Chart
	chart := m.createChart()

	// Workers table
	// workers := m.createWorkersTable()

	// Stack vertically
	content := container.NewVBox(
		metrics,
		widget.NewSeparator(),
		chart,
		// workers,
	)

	// Scroll container
	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(900, 700))

	// Background
	bg := canvas.NewRectangle(colorBackground)

	return container.NewStack(bg, container.NewPadded(scroll))
}

// createMetricsGrid creates the metrics cards grid
// يجمع بطاقات المقاييس الرئيسية في شبكة منظمة.
func (m *DashboardApp) createMetricsGrid() fyne.CanvasObject {
	// Row 1: Hashrate, Miners, Efficiency, Power
	m.State.Hashrate.Set(0.0)
	m.State.Power.Set(0)
	m.State.ActiveWorkers.Set(0)
	m.State.OfflineDevices.Set(0)
	row1 := container.NewGridWithColumns(4,
		m.createMetricCard("Total Hashrate (5 min)", m.State.Hashrate, "TH/s", colorBlue),
		m.createMetricCard("Total Power", m.State.Power, "W", colorOrange),
		m.createMetricCard("Total Online Miners", m.State.ActiveWorkers, "", colorGreen),
		m.createMetricCard("Total Offline Miners", m.State.OfflineDevices, "", colorRed),
	)

	// Row 2: Balance, Revenue, Uptime, Hashprice
	// row2 := container.NewGridWithColumns(4,
	// 	m.createMetricCard("Balance", m.State.Balance, "BTC", colorOrange),
	// 	m.createMetricCard("Revenue (24h)", m.State.Revenue24h, "$", colorBlue),
	// 	m.createMetricCard("Efficiency (5 min)", m.State.Efficiency, "%", colorGreen),
	// 	m.createMetricCard("Hashprice", m.State.Hashprice, "$", colorTextSecondary),
	// )

	return container.NewVBox(row1)
}

// createMetricCard creates a single metric display card
// يعرض قيمة مترابطة (binding) مع عنوان ووحدة لون مخصص.
func (m *DashboardApp) createMetricCard(title string, value binding.DataItem, unit string, valueColor color.Color) fyne.CanvasObject {
	// Title
	titleLabel := canvas.NewText(title, colorTextSecondary)
	titleLabel.TextSize = 11

	// Value - use appropriate binding type
	var valueLabel *widget.Label
	switch v := value.(type) {
	case binding.Float:
		valueLabel = widget.NewLabelWithData(binding.FloatToStringWithFormat(v, "%.2f"))
	case binding.Int:
		valueLabel = widget.NewLabelWithData(binding.IntToString(v))
	default:
		valueLabel = widget.NewLabel("N/A")
	}
	valueLabel.Alignment = fyne.TextAlignLeading
	valueLabel.TextStyle = fyne.TextStyle{Bold: true}
	valueLabel.Importance = widget.HighImportance

	// Create custom styled value display
	valueText := canvas.NewText("", valueColor)
	valueText.TextSize = 22
	valueText.TextStyle = fyne.TextStyle{Bold: true}

	// Update value when binding changes
	switch v := value.(type) {
	case binding.Float:
		v.AddListener(binding.NewDataListener(func() {
			f, _ := v.Get()
			if unit == "BTC" {
				valueText.Text = fmt.Sprintf("%.6f %s", f, unit)
			} else if unit == "$" {
				valueText.Text = fmt.Sprintf("%s%.2f", unit, f)
			} else if unit == "%" {
				valueText.Text = fmt.Sprintf("%.1f%s", f, unit)
			} else {
				valueText.Text = fmt.Sprintf("%.1f %s", f, unit)
			}
			valueText.Refresh()
		}))
		f, _ := v.Get()
		if unit == "BTC" {
			valueText.Text = fmt.Sprintf("%.6f %s", f, unit)
		} else if unit == "$" {
			valueText.Text = fmt.Sprintf("%s%.2f", unit, f)
		} else if unit == "%" {
			valueText.Text = fmt.Sprintf("%.1f%s", f, unit)
		} else {
			valueText.Text = fmt.Sprintf("%.1f %s", f, unit)
		}

	case binding.Int:
		v.AddListener(binding.NewDataListener(func() {
			i, _ := v.Get()
			if unit != "" {
				valueText.Text = fmt.Sprintf("%d %s", i, unit)
			} else {
				valueText.Text = fmt.Sprintf("%d", i)
			}
			valueText.Refresh()
		}))
		i, _ := v.Get()
		if unit != "" {
			valueText.Text = fmt.Sprintf("%d %s", i, unit)
		} else {
			valueText.Text = fmt.Sprintf("%d", i)
		}
	}

	// Card content
	content := container.NewVBox(
		titleLabel,
		valueText,
	)

	// Card background
	bg := canvas.NewRectangle(colorSurface)
	bg.CornerRadius = 8

	return container.NewStack(bg, container.NewPadded(content))
}

// createChart creates the hashrate chart
// ينشئ واجهة الرسم البياني (حاليًا تمثيل نصي) مع عناصر التحكم في نطاق الوقت.
func (m *DashboardApp) createChart() fyne.CanvasObject {
	// Title
	title := canvas.NewText("📊 Hashrate History (24h)", colorBlue)
	title.TextSize = 16
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Time range buttons
	range1D := widget.NewButton("1D", func() {
		print("1D in Chart")
	})
	range1D.Importance = widget.HighImportance
	// range1W := widget.NewButton("1W", func() {
	// 	print("1W in Chart")
	// })

	// v1.0.4: زر فتح الرسم البياني التفاعلي في المتصفح
	openChartBtn := widget.NewButton("📈 Open Interactive Chart", func() {
		if m.echartsServer != nil {
			openBrowserURL(m.echartsServer.URL())
		}
	})
	openChartBtn.Importance = widget.HighImportance

	webchart := container.NewHBox(
		openChartBtn, // v1.0.4
	)

	// Chart placeholder (using custom rendering)
	chartWidget := m.createChartWidget()

	// Header
	header := container.NewBorder(nil, nil, title, webchart)

	// Chart container
	chartContainer := container.NewBorder(header, chartWidget, nil, nil, nil)

	// Background
	bg := canvas.NewRectangle(colorSurface)
	bg.CornerRadius = 8

	return container.NewStack(bg, container.NewPadded(chartContainer))
}

// ChartWidget is a custom widget for displaying the hashrate chart
type ChartWidget struct {
	widget.BaseWidget
	data     []float64
	minVal   float64
	maxVal   float64
	appState *AppState
}

// UpdateData updates the chart data and refreshes the display
func (c *ChartWidget) UpdateData(newData []float64) {
	c.data = newData
	c.calculateRange()
	fyne.Do(func() {
		c.Refresh()
	})
}

// createChartWidget creates a new chart widget
func (m *DashboardApp) createChartWidget() fyne.CanvasObject {
	var data []float64
	if len(m.State.Devices) > 0 {
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
		data = m.State.HashrateHistory // fallback to global history
	}

	chart := &ChartWidget{
		data:     data,
		appState: m.State,
	}
	chart.ExtendBaseWidget(chart)
	chart.calculateRange()
	chart.Refresh() // ensure initial display is populated

	m.Chart = chart // store reference for updates
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
	return &chartRenderer{widget: c}
}

type chartRenderer struct {
	widget    *ChartWidget
	chartText *canvas.Text
}

func (r *chartRenderer) Layout(size fyne.Size) {
	if r.chartText == nil {
		return
	}
	// Stretch the placeholder text to fill the available space
	r.chartText.Resize(size)
	r.chartText.Move(fyne.NewPos(0, 0))
}

func (r *chartRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 150)
}

func (r *chartRenderer) Refresh() {
	if r.chartText == nil {
		return
	}

	if len(r.widget.data) == 0 {
		r.chartText.Text = "No data"
		r.chartText.Color = colorTextSecondary
		return
	}

	// Display a simple summary of the current selection
	latest := r.widget.data[len(r.widget.data)-1]
	minVal := r.widget.minVal
	maxVal := r.widget.maxVal
	if latest != 0 {
		r.chartText.Text = fmt.Sprintf("Latest: %.1f TH/s  (min: %.1f, max: %.1f)", latest, minVal, maxVal)
		r.chartText.Color = colorTextPrimary
	}
}

func (r *chartRenderer) Destroy() {}

func (r *chartRenderer) Objects() []fyne.CanvasObject {
	if r.chartText == nil {
		r.chartText = canvas.NewText("Chart rendering...", colorTextSecondary)
		r.chartText.TextSize = 12
	}
	return []fyne.CanvasObject{r.chartText}
}

// createFooter creates the status bar
// يعرض اختصارات لوحة المفاتيح وآخر وقت لتحديث البيانات.
func (m *DashboardApp) createFooter() fyne.CanvasObject {
	// Keyboard shortcuts
	shortcuts := canvas.NewText("for support please visit your dashboard -> support", colorTextSecondary)
	shortcuts.TextSize = 11

	// Last update
	updateLabel := widget.NewLabelWithData(m.State.LastUpdate)
	updateLabel.Alignment = fyne.TextAlignTrailing

	// Container
	footer := container.NewBorder(
		container.NewHBox(shortcuts, layout.NewSpacer(), updateLabel),
		nil,
		nil,
		nil,
		canvas.NewRectangle(colorSurface),
	)

	return container.NewPadded(footer)
}

// =============================================================================
// التحديث
// =============================================================================

// startAutoRefresh يبدأ التحديث التلقائي
// يقوم بتشغيل مؤقت (Ticker) ويستدعي refreshData بشكل دوري.
func (d *DashboardApp) startAutoRefresh() {
	d.Ticker = time.NewTicker(time.Duration(d.State.RefreshRate) * time.Second)

	go func() {
		for {
			select {
			case <-d.Ticker.C:
				if autoRefresh, _ := d.State.AutoRefresh.Get(); autoRefresh {
					d.refreshData()
				}
			case <-d.StopChan:
				d.Ticker.Stop()
				return
			}
		}
	}()
}

// syncDevices synchronizes real physical devices from GoASIC to the Dashboard.
func (d *DashboardApp) syncDevices() {
	if d.State.GoASICManager == nil {
		return
	}
	discovered := d.State.GoASICManager.GetDevices()

	// FIX (Bug 2): If the manager is still scanning AND has found nothing yet,
	// keep whatever devices are already in the list instead of wiping them with
	// an empty slice. This prevents the list from flickering to "0 devices"
	// during the initial scan or between scan cycles.
	if len(discovered) == 0 && d.State.GoASICManager.IsScanning() {
		return
	}

	existingDevices := make(map[string]*models.Miner)
	for _, d := range d.State.Devices {
		existingDevices[d.ID] = d
	}

	newDevices := make([]*models.Miner, 0, len(discovered))
	for _, dev := range discovered {
		miner := &models.Miner{
			ID:           dev.IP,
			Name:         dev.IP,
			Model:        dev.Model,
			Manufacturer: dev.Make,
			IPAddress:    dev.IP,
			Status:       string(dev.Status),
			LastSeen:     dev.LastSeen,
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

		// Restore history and append current hashrate
		if existing, found := existingDevices[miner.ID]; found {
			miner.Stats.HashrateHistory = append([]float64(nil), existing.Stats.HashrateHistory...)
		}

		// Only track if online
		if miner.Status == "online" {
			miner.Stats.HashrateHistory = append(miner.Stats.HashrateHistory, miner.Stats.Hashrate)
		} else {
			miner.Stats.HashrateHistory = append(miner.Stats.HashrateHistory, 0.0)
		}

		// Cap max history length for the chart
		maxHistory := 288 // e.g. 24 hours at 5-min intervals
		if len(miner.Stats.HashrateHistory) > maxHistory {
			miner.Stats.HashrateHistory = miner.Stats.HashrateHistory[len(miner.Stats.HashrateHistory)-maxHistory:]
		}

		newDevices = append(newDevices, miner)
	}
	d.State.Devices = newDevices

	if d.DeviceCountStr != nil {
		d.DeviceCountStr.Set(fmt.Sprintf("Total: %d", len(d.State.Devices)))
	}
}

// refreshData يحدث البيانات
// يتم استدعاؤه من مؤقت التحديث لتحديث القيم العرضية وحفظ تزامن الواجهة.
func (d *DashboardApp) refreshData() {
	d.mu.Lock()

	// تحديث الوقت
	d.State.LastUpdate.Set(time.Now().Format("3:04:05 PM"))

	// جلب الأجهزة الحقيقية وتحديث الواجهة
	d.syncDevices()

	var totalHR float64
	var totalPower int
	for _, miner := range d.State.Devices {
		if miner.Status == "online" {
			totalHR += miner.Stats.Hashrate
			totalPower += miner.Stats.Power

			// Log individual device hashrate
			go func(ip string, hr float64) {
				safeName := strings.ReplaceAll(ip, ".", "_")
				safeName = strings.ReplaceAll(safeName, ":", "_")
				filename := filepath.Join("device_log", fmt.Sprintf("%s.csv", safeName))
				appendToCSV(filename, []string{time.Now().Format("2006-01-02 15:04:05"), strconv.FormatFloat(hr, 'f', 2, 64)})
			}(miner.ID, miner.Stats.Hashrate)
		}
	}
	d.State.TotalHashrate.Set(totalHR)

	// Log total hashrate
	go func(hr float64) {
		filename := filepath.Join("device_log", "total_hashrate.csv")
		appendToCSV(filename, []string{time.Now().Format("2006-01-02 15:04:05"), strconv.FormatFloat(hr, 'f', 2, 64)})
		// v1.0.4: تحديث سيرفر الرسم البياني التفاعلي بعد كل كتابة
		if d.echartsServer != nil {
			_ = d.echartsServer.UpdateFromCSV(filename, "سجل إجمالي معدل التجزئة")
		}
	}(totalHR)

	// تحديث بيانات الجهاز المحدد
	d.updateSelectedDevice()

	// تحديث حسابات الأجهزة (مثلاً عندما تتغير حالة أحد الأجهزة)
	d.State.UpdateDeviceCounts()
	d.mu.Unlock()

	if d.DeviceList != nil {
		fyne.Do(func() {
			d.DeviceList.Refresh()
		})
	}
}

// updateSelectedDevice يحدث بيانات الجهاز المحدد
// يأخذ البيانات من الجهاز المحدد في القائمة ويعرضها في الواجهة.
func (d *DashboardApp) updateSelectedDevice() {
	// Assumes caller holds d.mu.RLock() or Lock()
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

		var aggregatedHistory []float64
		if maxLen > 0 {
			aggregatedHistory = make([]float64, maxLen)
			for _, miner := range d.State.Devices {
				offset := maxLen - len(miner.Stats.HashrateHistory)
				for i, h := range miner.Stats.HashrateHistory {
					aggregatedHistory[offset+i] += h
				}
			}
		} else {
			aggregatedHistory = d.State.HashrateHistory
		}

		if d.Chart != nil {
			d.Chart.UpdateData(aggregatedHistory)
		}
		return
	}

	selectedDevice := d.State.Devices[d.State.SelectedIndex]

	// Update global Hashrate and Power to show only selected device
	d.State.Hashrate.Set(selectedDevice.Stats.Hashrate)
	d.State.Power.Set(selectedDevice.Stats.Power)

	// Update selected device metrics
	d.State.SelectedHashrate.Set(selectedDevice.Stats.Hashrate)
	d.State.SelectedTemp.Set(selectedDevice.Stats.Temperature)
	d.State.SelectedPower.Set(selectedDevice.Stats.Power)
	d.State.SelectedEfficiency.Set(selectedDevice.Stats.Efficiency)
	d.State.SelectedErrors.Set(selectedDevice.Stats.Errors)

	// Update chart with selected device's data
	if d.Chart != nil {
		d.Chart.UpdateData(selectedDevice.Stats.HashrateHistory)
	}
}

// appendToCSV helper to append records to CSV file
func appendToCSV(filename string, record []string) {
	fileInfo, err := os.Stat(filename)
	isNew := os.IsNotExist(err) || (err == nil && fileInfo.Size() == 0)

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

// =============================================================================
// v1.0.4 — دوال مساعدة جديدة
// =============================================================================

// openBrowserURL يفتح URL في المتصفح الافتراضي للنظام.
// يدعم Linux (xdg-open) و macOS (open) و Windows (rundll32).
func openBrowserURL(rawURL string) {
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
	_ = exec.Command(cmd, args...).Start()
}

// StopEChartsServer يُوقف سيرفر الرسم البياني عند إغلاق التطبيق.
// استدعِها من d.Window.SetCloseIntercept إذا أردت إيقافاً صريحاً.
func (d *DashboardApp) StopEChartsServer() {
	if d.echartsServer != nil {
		d.echartsServer.Stop()
	}
}
