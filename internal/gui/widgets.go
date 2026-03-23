// =============================================================================
// Package widgets - Custom Fyne Widgets for MinerGate Dashboard
// =============================================================================
// Custom widgets including:
// - CircularGauge: Animated circular gauge for hashrate display
// - StatusCard: Color-coded status indicator cards
// - MetricCard: Large metric display cards
// =============================================================================
package gui

import (
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// =============================================================================
// Color Palette - Modern Dark Theme with Teal/Orange Accents
// =============================================================================

var (
	// Background colors
	ColorBackground  = color.RGBA{R: 13, G: 13, B: 18, A: 255}  // #0d0d12
	ColorSurface     = color.RGBA{R: 26, G: 26, B: 36, A: 255}  // #1a1a24
	ColorSurfaceLight = color.RGBA{R: 37, G: 37, B: 50, A: 255} // #252532

	// Text colors
	ColorTextPrimary   = color.RGBA{R: 228, G: 228, B: 231, A: 255} // #e4e4e7
	ColorTextSecondary = color.RGBA{R: 113, G: 113, B: 122, A: 255} // #71717a
	ColorTextMuted     = color.RGBA{R: 82, G: 82, B: 91, A: 255}    // #52525b

	// Accent colors (teal/cyan)
	ColorTeal      = color.RGBA{R: 0, G: 229, B: 255, A: 255}    // #00e5ff
	ColorTealDark  = color.RGBA{R: 20, G: 184, B: 166, A: 255}   // #14b8a6
	ColorTealGlow  = color.RGBA{R: 0, G: 229, B: 255, A: 80}     // glow effect

	// Accent colors (orange)
	ColorOrange     = color.RGBA{R: 249, G: 115, B: 22, A: 255} // #f97316
	ColorOrangeGlow = color.RGBA{R: 249, G: 115, B: 22, A: 80}  // glow effect

	// Status colors
	ColorGreen  = color.RGBA{R: 34, G: 197, B: 94, A: 255}  // #22c55e
	ColorYellow = color.RGBA{R: 234, G: 179, B: 8, A: 255}   // #eab308
	ColorRed    = color.RGBA{R: 239, G: 68, B: 68, A: 255}   // #ef4444

	// Border colors
	ColorBorder = color.RGBA{R: 39, G: 39, B: 42, A: 255} // #27272a
)

// =============================================================================
// CircularGauge - Animated Circular Gauge Widget
// =============================================================================

// CircularGauge displays a circular progress gauge with animated value
type CircularGauge struct {
	widget.BaseWidget

	value     float64
	maxValue  float64
	unit      string
	label     string
	size      float32
	strokeWidth float32
	color     color.Color
	animatedValue float64
	animating bool
}

// NewCircularGauge creates a new circular gauge
func NewCircularGauge(value, maxValue float64, unit, label string, size float32, col color.Color) *CircularGauge {
	g := &CircularGauge{
		value:     value,
		maxValue:  maxValue,
		unit:      unit,
		label:     label,
		size:      size,
		strokeWidth: size / 15,
		color:     col,
		animatedValue: 0,
	}
	g.ExtendBaseWidget(g)
	return g
}

// SetValue updates the gauge value with animation
func (g *CircularGauge) SetValue(value float64) {
	g.value = value
	g.animate()
}

func (g *CircularGauge) animate() {
	if g.animating {
		return
	}
	g.animating = true
	
	go func() {
		startValue := g.animatedValue
		endValue := g.value
		duration := 500 * time.Millisecond
		startTime := time.Now()

		for time.Since(startTime) < duration {
			elapsed := time.Since(startTime)
			progress := float64(elapsed) / float64(duration)
			// Ease out cubic
			eased := 1 - math.Pow(1-progress, 3)
			g.animatedValue = startValue + (endValue-startValue)*eased
			g.Refresh()
			time.Sleep(16 * time.Millisecond)
		}
		g.animatedValue = endValue
		g.animating = false
		g.Refresh()
	}()
}

// CreateRenderer implements fyne.Widget
func (g *CircularGauge) CreateRenderer() fyne.WidgetRenderer {
	return &circularGaugeRenderer{
		gauge: g,
	}
}

type circularGaugeRenderer struct {
	gauge    *CircularGauge
	objects  []fyne.CanvasObject
}

func (r *circularGaugeRenderer) Layout(size fyne.Size) {
	for _, obj := range r.objects {
		obj.Resize(size)
	}
}

func (r *circularGaugeRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.gauge.size, r.gauge.size)
}

func (r *circularGaugeRenderer) Refresh() {
	// Rebuild the widget
	r.objects = r.buildObjects()
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *circularGaugeRenderer) Objects() []fyne.CanvasObject {
	if len(r.objects) == 0 {
		r.objects = r.buildObjects()
	}
	return r.objects
}

func (r *circularGaugeRenderer) Destroy() {}

func (r *circularGaugeRenderer) buildObjects() []fyne.CanvasObject {
	g := r.gauge
	centerX := g.size / 2
	centerY := g.size / 2
	radius := (g.size - g.strokeWidth) / 2

	// Calculate percentage and arc
	percentage := g.animatedValue / g.maxValue
	if percentage > 1 {
		percentage = 1
	}
	
	// Background circle
	bgCircle := canvas.NewCircle(ColorSurfaceLight)
	bgCircle.Resize(fyne.NewSize(g.size, g.size))
	bgCircle.StrokeColor = ColorSurface
	bgCircle.StrokeWidth = g.strokeWidth

	// Create arc path for progress
	arcPath := canvas.NewLine(g.color)
	arcPath.StrokeWidth = g.strokeWidth
	arcPath.StrokeColor = g.color

	// Draw arc using multiple small lines
	var points []fyne.Position
	startAngle := -math.Pi / 2 // Start from top
	endAngle := startAngle + (2 * math.Pi * percentage)
	
	steps := int(percentage * 100) + 1
	if steps < 2 {
		steps = 2
	}
	
	for i := 0; i <= steps; i++ {
		angle := startAngle + (endAngle-startAngle)*float64(i)/float64(steps)
		x := centerX + float32(float64(radius)*math.Cos(angle))
		y := centerY + float32(float64(radius)*math.Sin(angle))
		points = append(points, fyne.NewPos(x, y))
	}

	// Value text
	valueText := canvas.NewText("", g.color)
	valueText.TextSize = g.size / 6
	valueText.TextStyle = fyne.TextStyle{Bold: true}
	valueText.Alignment = fyne.TextAlignCenter
	valueText.Text = formatValue(g.animatedValue)
	valueText.Move(fyne.NewPos(centerX, centerY - g.size/10))

	// Unit text
	unitText := canvas.NewText(g.unit, ColorTextSecondary)
	unitText.TextSize = g.size / 14
	unitText.Alignment = fyne.TextAlignCenter
	unitText.Move(fyne.NewPos(centerX, centerY + g.size/12))

	// Label text
	labelText := canvas.NewText(g.label, ColorTextSecondary)
	labelText.TextSize = g.size / 16
	labelText.Alignment = fyne.TextAlignCenter
	labelText.Move(fyne.NewPos(centerX, g.size + 5))

	// Create container
	content := container.NewStack(
		bgCircle,
		arcPath,
		container.NewVBox(
			container.NewCenter(valueText),
			container.NewCenter(unitText),
		),
	)
	content.Resize(fyne.NewSize(g.size, g.size))

	// Create tick marks
	tickContainer := container.NewWithoutLayout()
	for i := 0; i < 12; i++ {
		angle := float64(i) * 30 * math.Pi / 180
		innerR := radius - g.strokeWidth - 8
		outerR := radius - g.strokeWidth - 4
		x1 := centerX + float32(float64(innerR)*math.Cos(angle))
		y1 := centerY + float32(float64(innerR)*math.Sin(angle))
		x2 := centerX + float32(float64(outerR)*math.Cos(angle))
		y2 := centerY + float32(float64(outerR)*math.Sin(angle))
		
		tick := canvas.NewLine(ColorTextMuted)
		tick.Position1 = fyne.NewPos(x1, y1)
		tick.Position2 = fyne.NewPos(x2, y2)
		tick.StrokeWidth = 1
		tickContainer.Add(tick)
	}

	return []fyne.CanvasObject{
		container.NewStack(
			content,
			tickContainer,
			container.NewVBox(
				container.NewCenter(labelText),
			),
		),
	}
}

// =============================================================================
// StatusCard - Color-coded Status Card Widget
// =============================================================================

// StatusType represents the status type for styling
type StatusType int

const (
	StatusSuccess StatusType = iota
	StatusWarning
	StatusDanger
	StatusNeutral
)

// StatusCard displays a metric with status indicator
type StatusCard struct {
	widget.BaseWidget

	title    string
	value    string
	unit     string
	status   StatusType
	trend    string
	trendValue string
	icon     fyne.Resource
}

// NewStatusCard creates a new status card
func NewStatusCard(title, value string, unit string, status StatusType) *StatusCard {
	card := &StatusCard{
		title:  title,
		value:  value,
		unit:   unit,
		status: status,
	}
	card.ExtendBaseWidget(card)
	return card
}

// SetValue updates the card value
func (c *StatusCard) SetValue(value string) {
	c.value = value
	c.Refresh()
}

// SetTrend sets the trend indicator
func (c *StatusCard) SetTrend(trend, trendValue string) {
	c.trend = trend
	c.trendValue = trendValue
	c.Refresh()
}

// CreateRenderer implements fyne.Widget
func (c *StatusCard) CreateRenderer() fyne.WidgetRenderer {
	return &statusCardRenderer{card: c}
}

type statusCardRenderer struct {
	card    *StatusCard
	objects []fyne.CanvasObject
}

func (r *statusCardRenderer) Layout(size fyne.Size) {
	for _, obj := range r.objects {
		obj.Resize(size)
	}
}

func (r *statusCardRenderer) MinSize() fyne.Size {
	return fyne.NewSize(150, 80)
}

func (r *statusCardRenderer) Refresh() {
	r.objects = r.buildObjects()
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *statusCardRenderer) Objects() []fyne.CanvasObject {
	if len(r.objects) == 0 {
		r.objects = r.buildObjects()
	}
	return r.objects
}

func (r *statusCardRenderer) Destroy() {}

func (r *statusCardRenderer) buildObjects() []fyne.CanvasObject {
	c := r.card

	// Get status colors
	var statusColor, bgColor, borderColor color.Color
	switch c.status {
	case StatusSuccess:
		statusColor = ColorGreen
		bgColor = color.RGBA{R: 34, G: 197, B: 94, A: 25}
		borderColor = color.RGBA{R: 34, G: 197, B: 94, A: 75}
	case StatusWarning:
		statusColor = ColorYellow
		bgColor = color.RGBA{R: 234, G: 179, B: 8, A: 25}
		borderColor = color.RGBA{R: 234, G: 179, B: 8, A: 75}
	case StatusDanger:
		statusColor = ColorRed
		bgColor = color.RGBA{R: 239, G: 68, B: 68, A: 25}
		borderColor = color.RGBA{R: 239, G: 68, B: 68, A: 75}
	default:
		statusColor = ColorTextSecondary
		bgColor = ColorSurface
		borderColor = ColorBorder
	}

	// Background with status indicator
	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 8
	bg.StrokeColor = borderColor
	bg.StrokeWidth = 1

	// Status indicator bar
	indicator := canvas.NewRectangle(statusColor)
	indicator.SetMinSize(fyne.NewSize(4, 80))

	// Title
	titleText := canvas.NewText(c.title, ColorTextSecondary)
	titleText.TextSize = 11
	titleText.TextStyle = fyne.TextStyle{}

	// Value
	valueText := canvas.NewText(c.value, statusColor)
	valueText.TextSize = 22
	valueText.TextStyle = fyne.TextStyle{Bold: true}

	// Unit
	unitText := canvas.NewText(c.unit, ColorTextSecondary)
	unitText.TextSize = 12

	// Content container
	content := container.NewVBox(
		titleText,
		container.NewHBox(
			valueText,
			unitText,
		),
	)

	if c.trendValue != "" {
		trendColor := ColorTextSecondary
		if c.trend == "up" {
			trendColor = ColorGreen
		} else if c.trend == "down" {
			trendColor = ColorRed
		}
		trendText := canvas.NewText(c.trendValue, trendColor)
		trendText.TextSize = 11
		content.Add(trendText)
	}

	// Main container with indicator
	mainContainer := container.NewBorder(
		nil, nil,
		container.NewCenter(indicator),
		nil,
		container.NewPadded(content),
	)

	return []fyne.CanvasObject{
		container.NewStack(bg, mainContainer),
	}
}

// =============================================================================
// MetricCard - Large Metric Display Card
// =============================================================================

// MetricCard displays a large metric with icon
type MetricCard struct {
	widget.BaseWidget

	title    string
	value    string
	unit     string
	subtitle string
	color    color.Color
	icon     fyne.Resource
}

// NewMetricCard creates a new metric card
func NewMetricCard(title, value string, unit string, col color.Color) *MetricCard {
	card := &MetricCard{
		title: title,
		value: value,
		unit:  unit,
		color: col,
	}
	card.ExtendBaseWidget(card)
	return card
}

// CreateRenderer implements fyne.Widget
func (c *MetricCard) CreateRenderer() fyne.WidgetRenderer {
	return &metricCardRenderer{card: c}
}

type metricCardRenderer struct {
	card    *MetricCard
	objects []fyne.CanvasObject
}

func (r *metricCardRenderer) Layout(size fyne.Size) {
	for _, obj := range r.objects {
		obj.Resize(size)
	}
}

func (r *metricCardRenderer) MinSize() fyne.Size {
	return fyne.NewSize(180, 100)
}

func (r *metricCardRenderer) Refresh() {
	r.objects = r.buildObjects()
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *metricCardRenderer) Objects() []fyne.CanvasObject {
	if len(r.objects) == 0 {
		r.objects = r.buildObjects()
	}
	return r.objects
}

func (r *metricCardRenderer) Destroy() {}

func (r *metricCardRenderer) buildObjects() []fyne.CanvasObject {
	c := r.card

	// Background
	bg := canvas.NewRectangle(ColorSurface)
	bg.CornerRadius = 8
	bg.StrokeColor = ColorBorder
	bg.StrokeWidth = 1

	// Icon background
	rgba := color.RGBAModel.Convert(c.color).(color.RGBA)
	iconBg := canvas.NewRectangle(color.RGBA{R: rgba.R, G: rgba.G, B: rgba.B, A: 25})
	iconBg.CornerRadius = 6

	// Title
	titleText := canvas.NewText(c.title, ColorTextSecondary)
	titleText.TextSize = 11
	titleText.TextStyle = fyne.TextStyle{}

	// Value
	valueText := canvas.NewText(c.value, c.color)
	valueText.TextSize = 28
	valueText.TextStyle = fyne.TextStyle{Bold: true}

	// Unit
	unitText := canvas.NewText(c.unit, ColorTextSecondary)
	unitText.TextSize = 12

	// Content
	header := container.NewBorder(
		nil, nil,
		titleText,
		container.NewCenter(container.NewPadded(iconBg)),
		nil,
	)

	valueRow := container.NewHBox(
		valueText,
		unitText,
	)

	content := container.NewVBox(
		header,
		container.NewPadded(valueRow),
	)

	if c.subtitle != "" {
		subtitleText := canvas.NewText(c.subtitle, ColorTextMuted)
		subtitleText.TextSize = 11
		content.Add(subtitleText)
	}

	return []fyne.CanvasObject{
		container.NewStack(bg, container.NewPadded(content)),
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func formatValue(v float64) string {
	if v >= 1000 {
		return formatFloat(v/1000, 2) + "k"
	}
	return formatFloat(v, 2)
}

func formatFloat(v float64, prec int) string {
	format := "%." + string(rune('0'+prec)) + "f"
	return sprintf(format, v)
}

func sprintf(format string, a ...interface{}) string {
	return format // Simplified for Fyne
}

// =============================================================================
// MinerGate Dark Theme
// =============================================================================

// MinerGateTheme implements fyne.Theme with dark mode and teal/orange accents
type MinerGateTheme struct{}

func (m *MinerGateTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return ColorBackground
	case theme.ColorNameButton:
		return ColorSurface
	case theme.ColorNameDisabledButton:
		return ColorSurfaceLight
	case theme.ColorNameInputBackground:
		return ColorSurface
	case theme.ColorNameOverlayBackground:
		return ColorSurface
	case theme.ColorNameDisabled:
		return ColorTextSecondary
	case theme.ColorNameForeground:
		return ColorTextPrimary
	case theme.ColorNamePlaceHolder:
		return ColorTextSecondary
	case theme.ColorNamePressed:
		return ColorSurfaceLight
	case theme.ColorNamePrimary:
		return ColorTeal
	case theme.ColorNameHover:
		return ColorSurfaceLight
	case theme.ColorNameFocus:
		return ColorTeal
	case theme.ColorNameScrollBar:
		return ColorSurfaceLight
	case theme.ColorNameSeparator:
		return ColorBorder
	case theme.ColorNameShadow:
		return color.RGBA{A: 50}
	case theme.ColorNameSelection:
		return ColorTeal
	case theme.ColorNameMenuBackground:
		return ColorSurface
	case theme.ColorNameHeaderBackground:
		return ColorSurface
	default:
		return ColorTextPrimary
	}
}

func (m *MinerGateTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *MinerGateTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *MinerGateTheme) Size(name fyne.ThemeSizeName) float32 {
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
