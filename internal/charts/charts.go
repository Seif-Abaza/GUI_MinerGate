// =============================================================================
// Package charts - الرسوم البيانية التفاعلية
// =============================================================================
// هذا الملف يوفر رسوم بيانية تفاعلية باستخدام Fyne:
// - رسم بياني لمعدل التجزئة (Hashrate) على مدار 24 ساعة
// - رسم بياني لدرجة الحرارة والطاقة (Temperature & Power)
// - تحديث تلقائي للبيانات
// - دعم التفاعل مع المستخدم
// =============================================================================
package charts

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"minergate/internal/models"
)

// =============================================================================
// ألوان الرسوم البيانية
// =============================================================================

var (
	// ChartBackgroundColor لون خلفية الرسم
	ChartBackgroundColor = color.RGBA{R: 30, G: 30, B: 46, A: 255}
	// ChartGridColor لون شبكة الرسم
	ChartGridColor = color.RGBA{R: 69, G: 71, B: 90, A: 255}
	// HashrateLineColor لون خط معدل التجزئة
	HashrateLineColor = color.RGBA{R: 137, G: 180, B: 250, A: 255} // أزرق
	// TemperatureLineColor لون خط درجة الحرارة
	TemperatureLineColor = color.RGBA{R: 249, G: 226, B: 175, A: 255} // أصفر
	// PowerLineColor لون خط الطاقة
	PowerLineColor = color.RGBA{R: 243, G: 139, B: 168, A: 255} // أحمر
	// TextColor لون النص
	TextColor = color.RGBA{R: 205, G: 214, B: 244, A: 255}
	// AxisColor لون المحاور
	AxisColor = color.RGBA{R: 166, G: 173, B: 200, A: 255}
)

// =============================================================================
// عنصر الرسم البياني لمعدل التجزئة
// =============================================================================

// HashrateChartWidget عنصر رسم معدل التجزئة
type HashrateChartWidget struct {
	widget.BaseWidget

	// البيانات
	data     []models.HistoricalPoint
	minValue float64
	maxValue float64
	avgValue float64
	period   string

	// الإعدادات
	title     string
	unit      string
	lineColor color.Color

	// الأبعاد
	width  float32
	height float32

	// التفاعل
	hoveredIndex int
	onHover      func(index int, point models.HistoricalPoint)
}

// NewHashrateChartWidget ينشئ رسم معدل التجزئة جديد
func NewHashrateChartWidget(title, unit string, lineColor color.Color) *HashrateChartWidget {
	c := &HashrateChartWidget{
		title:        title,
		unit:         unit,
		lineColor:    lineColor,
		period:       "1d",
		hoveredIndex: -1,
		width:        600,
		height:       200,
	}
	c.ExtendBaseWidget(c)
	return c
}

// SetData يضبط بيانات الرسم
func (c *HashrateChartWidget) SetData(data []models.HistoricalPoint) {
	c.data = data
	c.calculateStats()
	c.Refresh()
}

// SetPeriod يضبط فترة الرسم
func (c *HashrateChartWidget) SetPeriod(period string) {
	c.period = period
	c.Refresh()
}

// calculateStats يحسب الإحصائيات
func (c *HashrateChartWidget) calculateStats() {
	if len(c.data) == 0 {
		return
	}

	c.minValue = c.data[0].Hashrate
	c.maxValue = c.data[0].Hashrate
	sum := 0.0

	for _, p := range c.data {
		if p.Hashrate < c.minValue {
			c.minValue = p.Hashrate
		}
		if p.Hashrate > c.maxValue {
			c.maxValue = p.Hashrate
		}
		sum += p.Hashrate
	}

	c.avgValue = sum / float64(len(c.data))

	// إضافة هامش
	margin := (c.maxValue - c.minValue) * 0.1
	c.minValue -= margin
	c.maxValue += margin

	if c.minValue < 0 {
		c.minValue = 0
	}
}

// CreateRenderer ينشئ معرض الرسم
func (c *HashrateChartWidget) CreateRenderer() fyne.WidgetRenderer {
	return &hashrateChartRenderer{
		widget: c,
	}
}

// MinSize يعيد الحد الأدنى للحجم
func (c *HashrateChartWidget) MinSize() fyne.Size {
	return fyne.NewSize(c.width, c.height)
}

// hashrateChartRenderer معرض رسم معدل التجزئة
type hashrateChartRenderer struct {
	widget  *HashrateChartWidget
	objects []fyne.CanvasObject
}

// Layout يرتب العناصر
func (r *hashrateChartRenderer) Layout(size fyne.Size) {
	// إعادة بناء العناصر عند تغيير الحجم
	r.objects = r.buildChart(size)
}

// MinSize يعيد الحد الأدنى للحجم
func (r *hashrateChartRenderer) MinSize() fyne.Size {
	return r.widget.MinSize()
}

// Refresh يحدث العرض
func (r *hashrateChartRenderer) Refresh() {
	r.objects = r.buildChart(fyne.NewSize(r.widget.width, r.widget.height))
}

// Objects يعيد العناصر
func (r *hashrateChartRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// buildChart يبني الرسم البياني
func (r *hashrateChartRenderer) buildChart(size fyne.Size) []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}

	// الهوامش
	marginLeft := float32(50)
	marginRight := float32(20)
	marginTop := float32(30)
	marginBottom := float32(30)

	chartWidth := size.Width - marginLeft - marginRight
	chartHeight := size.Height - marginTop - marginBottom

	// رسم الخلفية
	bg := canvas.NewRectangle(ChartBackgroundColor)
	bg.SetMinSize(size)
	bg.Resize(size)
	objects = append(objects, bg)

	if len(r.widget.data) == 0 {
		// عرض رسالة عدم وجود بيانات
		noDataText := canvas.NewText("No data available", TextColor)
		noDataText.TextSize = 14
		noDataText.Move(fyne.NewPos(size.Width/2-50, size.Height/2-10))
		objects = append(objects, noDataText)
		return objects
	}

	// رسم المحاور
	r.drawAxes(objects, size, marginLeft, marginTop, chartWidth, chartHeight)

	// رسم الخط
	r.drawLine(objects, size, marginLeft, marginTop, chartWidth, chartHeight)

	// رسم العنوان
	title := canvas.NewText(r.widget.title, TextColor)
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Move(fyne.NewPos(marginLeft, 5))
	objects = append(objects, title)

	return objects
}

// drawAxes يرسم المحاور
func (r *hashrateChartRenderer) drawAxes(objects []fyne.CanvasObject, size fyne.Size, ml, mt, cw, ch float32) {
	// محور Y
	yAxis := canvas.NewLine(AxisColor)
	yAxis.Position1 = fyne.NewPos(ml, mt)
	yAxis.Position2 = fyne.NewPos(ml, mt+ch)
	yAxis.StrokeWidth = 1
	objects = append(objects, yAxis)

	// محور X
	xAxis := canvas.NewLine(AxisColor)
	xAxis.Position1 = fyne.NewPos(ml, mt+ch)
	xAxis.Position2 = fyne.NewPos(ml+cw, mt+ch)
	xAxis.StrokeWidth = 1
	objects = append(objects, xAxis)

	// خطوط الشبكة الأفقية
	numGridLines := 5
	for i := 0; i <= numGridLines; i++ {
		y := mt + ch*float32(i)/float32(numGridLines)
		line := canvas.NewLine(ChartGridColor)
		line.Position1 = fyne.NewPos(ml, y)
		line.Position2 = fyne.NewPos(ml+cw, y)
		line.StrokeWidth = 0.5
		objects = append(objects, line)

		// قيم المحور Y
		value := r.widget.maxValue - (r.widget.maxValue-r.widget.minValue)*float64(i)/float64(numGridLines)
		label := canvas.NewText(formatValue(value, r.widget.unit), AxisColor)
		label.TextSize = 10
		label.Move(fyne.NewPos(5, y-5))
		objects = append(objects, label)
	}
}

// drawLine يرسم خط البيانات
func (r *hashrateChartRenderer) drawLine(objects []fyne.CanvasObject, size fyne.Size, ml, mt, cw, ch float32) {
	if len(r.widget.data) < 2 {
		return
	}

	// حساب النقاط
	points := make([]fyne.Position, len(r.widget.data))
	for i, p := range r.widget.data {
		x := ml + cw*float32(i)/float32(len(r.widget.data)-1)
		y := mt + ch - ch*float32((p.Hashrate-r.widget.minValue)/(r.widget.maxValue-r.widget.minValue))
		points[i] = fyne.NewPos(x, y)
	}

	// رسم الخط
	for i := 0; i < len(points)-1; i++ {
		line := canvas.NewLine(r.widget.lineColor)
		line.Position1 = points[i]
		line.Position2 = points[i+1]
		line.StrokeWidth = 2
		objects = append(objects, line)
	}

	// رسم المنطقة تحت الخط (Area chart effect)
	// النقطة الأخيرة على المحور X
	for i := 0; i < len(points)-1; i++ {
		area := canvas.NewRectangle(color.RGBA{
			R: r.widget.lineColor.(color.RGBA).R,
			G: r.widget.lineColor.(color.RGBA).G,
			B: r.widget.lineColor.(color.RGBA).B,
			A: 30, // شفافية
		})
		area.Resize(fyne.NewSize(points[i+1].X-points[i].X, mt+ch-points[i].Y))
		area.Move(fyne.NewPos(points[i].X, points[i].Y))
		objects = append(objects, area)
	}
}

// Destroy يدمر المعرض
func (r *hashrateChartRenderer) Destroy() {}

// =============================================================================
// عنصر الرسم البياني لدرجة الحرارة والطاقة
// =============================================================================

// TempPowerChartWidget عنصر رسم درجة الحرارة والطاقة
type TempPowerChartWidget struct {
	widget.BaseWidget

	// البيانات
	tempData  []models.HistoricalPoint
	powerData []models.HistoricalPoint
	tempMin   float64
	tempMax   float64
	powerMin  float64
	powerMax  float64
	period    string

	// الأبعاد
	width  float32
	height float32
}

// NewTempPowerChartWidget ينشئ رسم درجة الحرارة والطاقة
func NewTempPowerChartWidget() *TempPowerChartWidget {
	c := &TempPowerChartWidget{
		period:   "1d",
		width:    600,
		height:   200,
		tempMin:  math.MaxFloat64,
		tempMax:  0,
		powerMin: math.MaxFloat64,
		powerMax: 0,
	}
	c.ExtendBaseWidget(c)
	return c
}

// SetData يضبط بيانات الرسم
func (c *TempPowerChartWidget) SetData(tempData, powerData []models.HistoricalPoint) {
	c.tempData = tempData
	c.powerData = powerData
	c.calculateStats()
	c.Refresh()
}

// calculateStats يحسب الإحصائيات
func (c *TempPowerChartWidget) calculateStats() {
	for _, p := range c.tempData {
		if p.Temperature < c.tempMin {
			c.tempMin = p.Temperature
		}
		if p.Temperature > c.tempMax {
			c.tempMax = p.Temperature
		}
	}

	for _, p := range c.powerData {
		if float64(p.Power) < c.powerMin {
			c.powerMin = float64(p.Power)
		}
		if float64(p.Power) > c.powerMax {
			c.powerMax = float64(p.Power)
		}
	}

	// إضافة هامش
	tempMargin := (c.tempMax - c.tempMin) * 0.1
	c.tempMin -= tempMargin
	c.tempMax += tempMargin

	powerMargin := (c.powerMax - c.powerMin) * 0.1
	c.powerMin -= powerMargin
	c.powerMax += powerMargin
}

// CreateRenderer ينشئ معرض الرسم
func (c *TempPowerChartWidget) CreateRenderer() fyne.WidgetRenderer {
	return &tempPowerChartRenderer{
		widget: c,
	}
}

// MinSize يعيد الحد الأدنى للحجم
func (c *TempPowerChartWidget) MinSize() fyne.Size {
	return fyne.NewSize(c.width, c.height)
}

// tempPowerChartRenderer معرض رسم درجة الحرارة والطاقة
type tempPowerChartRenderer struct {
	widget  *TempPowerChartWidget
	objects []fyne.CanvasObject
}

// Layout يرتب العناصر
func (r *tempPowerChartRenderer) Layout(size fyne.Size) {
	r.objects = r.buildChart(size)
}

// MinSize يعيد الحد الأدنى للحجم
func (r *tempPowerChartRenderer) MinSize() fyne.Size {
	return r.widget.MinSize()
}

// Refresh يحدث العرض
func (r *tempPowerChartRenderer) Refresh() {
	r.objects = r.buildChart(fyne.NewSize(r.widget.width, r.widget.height))
}

// Objects يعيد العناصر
func (r *tempPowerChartRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// buildChart يبني الرسم البياني
func (r *tempPowerChartRenderer) buildChart(size fyne.Size) []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}

	// الهوامش
	marginLeft := float32(50)
	marginRight := float32(50) // أكبر للمحور Y الثاني
	marginTop := float32(30)
	marginBottom := float32(30)

	chartWidth := size.Width - marginLeft - marginRight
	chartHeight := size.Height - marginTop - marginBottom

	// رسم الخلفية
	bg := canvas.NewRectangle(ChartBackgroundColor)
	bg.SetMinSize(size)
	bg.Resize(size)
	objects = append(objects, bg)

	// رسم المحور Y الأيسر (درجة الحرارة)
	r.drawYAxis(objects, size, marginLeft, marginTop, chartHeight, true, "°C", r.widget.tempMin, r.widget.tempMax, TemperatureLineColor)

	// رسم المحور Y الأيمن (الطاقة)
	r.drawYAxis(objects, size, marginLeft+chartWidth, marginTop, chartHeight, false, "W", r.widget.powerMin, r.widget.powerMax, PowerLineColor)

	// رسم خط درجة الحرارة
	r.drawDataLine(objects, size, marginLeft, marginTop, chartWidth, chartHeight, r.widget.tempData, r.widget.tempMin, r.widget.tempMax, TemperatureLineColor)

	// رسم خط الطاقة
	r.drawDataLine(objects, size, marginLeft, marginTop, chartWidth, chartHeight, r.widget.powerData, r.widget.powerMin, r.widget.powerMax, PowerLineColor)

	// رسم العنوان
	title := canvas.NewText("Temperature & Power (24h)", TextColor)
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Move(fyne.NewPos(marginLeft, 5))
	objects = append(objects, title)

	// رسم المفتاح
	legend := container.NewHBox(
		canvas.NewText("● Temperature", TemperatureLineColor),
		canvas.NewText("● Power", PowerLineColor),
	)
	legend.Move(fyne.NewPos(size.Width-150, 5))
	objects = append(objects, legend)

	return objects
}

// drawYAxis يرسم محور Y
func (r *tempPowerChartRenderer) drawYAxis(objects []fyne.CanvasObject, size fyne.Size, x, mt, ch float32, isLeft bool, unit string, minVal, maxVal float64, col color.Color) {
	// خط المحور
	axis := canvas.NewLine(AxisColor)
	if isLeft {
		axis.Position1 = fyne.NewPos(x, mt)
		axis.Position2 = fyne.NewPos(x, mt+ch)
	} else {
		axis.Position1 = fyne.NewPos(x, mt)
		axis.Position2 = fyne.NewPos(x, mt+ch)
	}
	axis.StrokeWidth = 1
	objects = append(objects, axis)

	// خطوط الشبكة والقيم
	numGridLines := 5
	for i := 0; i <= numGridLines; i++ {
		y := mt + ch*float32(i)/float32(numGridLines)
		value := maxVal - (maxVal-minVal)*float64(i)/float64(numGridLines)

		label := canvas.NewText(formatValue(value, unit), col)
		label.TextSize = 10

		if isLeft {
			label.Move(fyne.NewPos(5, y-5))
		} else {
			label.Move(fyne.NewPos(x+5, y-5))
		}
		objects = append(objects, label)
	}
}

// drawDataLine يرسم خط بيانات
func (r *tempPowerChartRenderer) drawDataLine(objects []fyne.CanvasObject, size fyne.Size, ml, mt, cw, ch float32, data []models.HistoricalPoint, minVal, maxVal float64, col color.Color) {
	if len(data) < 2 {
		return
	}

	points := make([]fyne.Position, len(data))
	for i, p := range data {
		x := ml + cw*float32(i)/float32(len(data)-1)
		value := p.Temperature
		if col == PowerLineColor {
			value = float64(p.Power)
		}
		y := mt + ch - ch*float32((value-minVal)/(maxVal-minVal))
		points[i] = fyne.NewPos(x, y)
	}

	for i := 0; i < len(points)-1; i++ {
		line := canvas.NewLine(col)
		line.Position1 = points[i]
		line.Position2 = points[i+1]
		line.StrokeWidth = 2
		objects = append(objects, line)
	}
}

// Destroy يدمر المعرض
func (r *tempPowerChartRenderer) Destroy() {}

// =============================================================================
// دوال مساعدة
// =============================================================================

// formatValue تنسيق القيمة للعرض
func formatValue(value float64, unit string) string {
	switch unit {
	case "TH/s":
		return fmt.Sprintf("%.1f", value)
	case "°C":
		return fmt.Sprintf("%.0f", value)
	case "W":
		return fmt.Sprintf("%.0f", value)
	case "J/TH":
		return fmt.Sprintf("%.1f", value)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}

// GenerateSampleHashrateData يولد بيانات تجريبية لمعدل التجزئة
func GenerateSampleHashrateData(hours int) []models.HistoricalPoint {
	data := make([]models.HistoricalPoint, hours)
	now := time.Now()

	for i := 0; i < hours; i++ {
		t := now.Add(-time.Duration(hours-i) * time.Hour)
		// محاكاة تقلبات واقعية
		base := 120.0
		variation := float64(i%20-10) * 2.5
		noise := float64(i%7-3) * 1.5
		hashrate := base + variation + noise

		data[i] = models.HistoricalPoint{
			Timestamp: t,
			Hashrate:  hashrate,
		}
	}

	return data
}

// GenerateSampleTempPowerData يولد بيانات تجريبية لدرجة الحرارة والطاقة
func GenerateSampleTempPowerData(hours int) (tempData, powerData []models.HistoricalPoint) {
	tempData = make([]models.HistoricalPoint, hours)
	powerData = make([]models.HistoricalPoint, hours)
	now := time.Now()

	for i := 0; i < hours; i++ {
		t := now.Add(-time.Duration(hours-i) * time.Hour)

		// درجة الحرارة
		tempBase := 65.0
		tempVariation := float64(i%10-5) * 2.0
		temp := tempBase + tempVariation
		tempData[i] = models.HistoricalPoint{
			Timestamp:   t,
			Temperature: temp,
		}

		// الطاقة
		powerBase := 2500
		powerVariation := (i%8 - 4) * 100
		power := powerBase + powerVariation
		powerData[i] = models.HistoricalPoint{
			Timestamp: t,
			Power:     power,
		}
	}

	return tempData, powerData
}
