// =============================================================================
// Package charts - سيرفر الرسم البياني التفاعلي (v1.0.4)
// =============================================================================
// هذا الملف يُضاف إلى حزمة charts الموجودة بدون أي تعارض مع:
//   - HashrateChartWidget (الموجود في الحزمة)
//   - TempPowerChartWidget (الموجود في الحزمة)
//
// ما يضيفه هذا الملف:
//   - EChartsServer: سيرفر HTTP محلي يخدم رسماً بيانياً تفاعلياً بـ go-echarts
//   - ReadHashrateCSV: قراءة ملفات CSV المحفوظة تلقائياً من refreshData()
//   - GenerateHashrateHTML: توليد صفحة HTML كاملة بـ go-echarts
//
// تدفق البيانات (الحالي في v1.0.3):
//
//	refreshData() → appendToCSV() → device_log/total_hashrate.csv
//
// تدفق البيانات (الجديد في v1.0.4):
//
//	device_log/total_hashrate.csv → ReadHashrateCSV() → GenerateHashrateHTML()
//	                             → EChartsServer → http://localhost:<port>/chart
//	                             → زر "Open Chart" في الواجهة → المتصفح
//
// =============================================================================
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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gecharts "github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	etypes "github.com/go-echarts/go-echarts/v2/types"
)

// =============================================================================
// هيكل نقطة البيانات
// =============================================================================

// CSVPoint نقطة بيانات مقروءة من ملف CSV
type CSVPoint struct {
	// الطابع الزمني
	Timestamp time.Time
	// قيمة معدل التجزئة بـ TH/s
	Value float64
}

// =============================================================================
// قراءة ملفات CSV
// =============================================================================

// ReadHashrateCSV يقرأ ملف CSV ويُرجع النقاط مرتبةً زمنياً.
// يتعامل مع الصيغة التي يكتبها appendToCSV في dashboard.go:
//   - العمود الأول: "2006-01-02 15:04:05"
//   - العمود الثاني: قيمة معدل التجزئة (float64)
//   - الصف الأول قد يكون رأساً: "Time,Hashrate"
//
// يُرجع nil بدون خطأ إذا كان الملف غير موجود.
func ReadHashrateCSV(path string) ([]CSVPoint, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("فشل في فتح CSV: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	var points []CSVPoint
	firstRow := true

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // تجاهل الصفوف التالفة
		}
		if len(row) < 2 {
			continue
		}

		// تخطي صف الرأس
		if firstRow {
			firstRow = false
			if strings.EqualFold(strings.TrimSpace(row[0]), "time") ||
				strings.EqualFold(strings.TrimSpace(row[0]), "timestamp") {
				continue
			}
		}

		// تحليل الطابع الزمني
		ts, err := time.ParseInLocation("2006-01-02 15:04:05",
			strings.TrimSpace(row[0]), time.Local)
		if err != nil {
			continue
		}

		// تحليل القيمة
		val, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err != nil {
			continue
		}

		points = append(points, CSVPoint{Timestamp: ts, Value: val})
	}

	// ترتيب تصاعدي حسب الوقت
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	// الاحتفاظ بآخر 288 نقطة فقط (24 ساعة بمعدل 5 دقائق)
	const maxPoints = 288
	if len(points) > maxPoints {
		points = points[len(points)-maxPoints:]
	}

	return points, nil
}

// FilterCSVByDuration يُصفّي النقاط ليُرجع فقط ما ضمن المدة الأخيرة d.
// إذا كان d == 0 يُرجع جميع النقاط. إذا لم تُطابق أي نقطة يُرجع الكل.
func FilterCSVByDuration(pts []CSVPoint, d time.Duration) []CSVPoint {
	if d == 0 || len(pts) == 0 {
		return pts
	}
	cutoff := time.Now().Add(-d)
	var out []CSVPoint
	for _, p := range pts {
		if p.Timestamp.After(cutoff) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return pts // احتياط: لا نُرجع فراغاً أبداً
	}
	return out
}

// =============================================================================
// توليد HTML بـ go-echarts
// =============================================================================

// GenerateHashrateHTML يُنتج صفحة HTML تفاعلية بـ go-echarts.
// المميزات:
//   - سمة داكنة (ThemeDark)
//   - تكبير/تصغير (DataZoom) بالسحب والتمرير
//   - تلميح عند المرور (Tooltip)
//   - صندوق أدوات للحفظ والاستعادة (Toolbox)
//   - منحنى ناعم مع ملء المساحة
func GenerateHashrateHTML(title string, pts []CSVPoint) ([]byte, error) {
	if len(pts) == 0 {
		return noDataPage(title), nil
	}

	line := gecharts.NewLine()

	// ── الإعدادات العامة ───────────────────────────────────────────────────
	line.SetGlobalOptions(
		gecharts.WithInitializationOpts(opts.Initialization{
			Theme:           etypes.ThemeEssos, // سمة داكنة أنيقة
			Width:           "100%",
			Height:          "500px",
			BackgroundColor: "#1e1e2e",
		}),
		gecharts.WithTitleOpts(opts.Title{
			Title: title,
			Subtitle: fmt.Sprintf("%d نقطة بيانات · آخر تحديث %s",
				len(pts), time.Now().Format("15:04:05")),
			TitleStyle:    &opts.TextStyle{Color: "#cdd6f4", FontSize: 16},
			SubtitleStyle: &opts.TextStyle{Color: "#a6adc8", FontSize: 12},
		}),
		// التلميح عند التمرير
		gecharts.WithTooltipOpts(opts.Tooltip{
			Show:        opts.Bool(true),
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "cross"},
		}),
		// المفتاح في الأسفل
		gecharts.WithLegendOpts(opts.Legend{
			Show:      opts.Bool(true),
			Bottom:    "2%",
			TextStyle: &opts.TextStyle{Color: "#a6adc8"},
		}),
		// هامش الرسم
		gecharts.WithGridOpts(opts.Grid{
			Left:         "4%",
			Right:        "3%",
			Top:          "15%",
			Bottom:       "18%",
			ContainLabel: opts.Bool(true),
		}),
		// المحور الأفقي (الوقت)
		gecharts.WithXAxisOpts(opts.XAxis{
			Type:        "category",
			BoundaryGap: opts.Bool(false),
			AxisLine:    &opts.AxisLine{LineStyle: &opts.LineStyle{Color: "#45475a"}},
			AxisLabel:   &opts.AxisLabel{Color: "#a6adc8", Rotate: 30, Show: opts.Bool(true)},
			SplitLine:   &opts.SplitLine{Show: opts.Bool(false)},
		}),
		// المحور الرأسي (معدل التجزئة)
		gecharts.WithYAxisOpts(opts.YAxis{
			Type: "value",
			Name: "معدل التجزئة (TH/s)",
			AxisLabel: &opts.AxisLabel{
				Color:     "#a6adc8",
				Show:      opts.Bool(true),
				Formatter: "{value} TH/s",
			},
			SplitLine: &opts.SplitLine{
				Show:      opts.Bool(true),
				LineStyle: &opts.LineStyle{Color: "#313244", Type: "dashed"},
			},
		}),
		// أداة التكبير بالسحب والتمرير
		gecharts.WithDataZoomOpts(
			opts.DataZoom{
				Type: "inside", Start: 0, End: 100, FilterMode: "none",
			},
			opts.DataZoom{
				Type:       "slider",
				Start:      0,
				End:        100,
				FilterMode: "none",
				// Height:          "5%",
				// Bottom:          "2%",
				// BorderColor:     "#45475a",
				// BackgroundColor: "#1e1e2e",
				// FillerColor:     "rgba(137,180,250,0.2)",
			},
		),
		// صندوق الأدوات
		gecharts.WithToolboxOpts(opts.Toolbox{
			Show:  opts.Bool(true),
			Right: "3%",
			Top:   "2%",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true), Title: "حفظ",
				},
				Restore: &opts.ToolBoxFeatureRestore{
					Show: opts.Bool(true), Title: "استعادة",
				},
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show:  opts.Bool(true),
					Title: map[string]string{"zoom": "تكبير", "back": "رجوع"},
				},
			},
		}),
	)

	// ── تسميات المحور الأفقي ──────────────────────────────────────────────
	xLabels := make([]string, len(pts))
	for i, p := range pts {
		xLabels[i] = p.Timestamp.Format("01/02 15:04")
	}
	line.SetXAxis(xLabels)

	// ── سلسلة البيانات ────────────────────────────────────────────────────
	lineData := make([]opts.LineData, len(pts))
	for i, p := range pts {
		lineData[i] = opts.LineData{Value: round2(p.Value)}
	}

	line.AddSeries(
		"إجمالي معدل التجزئة (TH/s)",
		lineData,
		gecharts.WithLineChartOpts(opts.LineChart{
			Smooth: opts.Bool(true), Symbol: "none", ShowSymbol: opts.Bool(false),
		}),
		gecharts.WithAreaStyleOpts(opts.AreaStyle{
			Opacity: opts.Float(0.25),
			Color:   "rgba(137,180,250,0.6)",
		}),
		gecharts.WithItemStyleOpts(opts.ItemStyle{Color: "#89b4fa"}),
		gecharts.WithLineStyleOpts(opts.LineStyle{Width: 2.5, Color: "#89b4fa"}),
	)

	var buf bytes.Buffer
	if err := line.Render(&buf); err != nil {
		return nil, fmt.Errorf("خطأ في تصيير go-echarts: %w", err)
	}
	return buf.Bytes(), nil
}

// noDataPage صفحة HTML تُعلم المستخدم بعدم توفر البيانات.
func noDataPage(title string) []byte {
	return []byte(fmt.Sprintf(`<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head><meta charset="UTF-8"><title>%s</title></head>
<body style="margin:0;background:#1e1e2e;display:flex;align-items:center;
justify-content:center;height:100vh;font-family:sans-serif;
color:#a6adc8;flex-direction:column;gap:16px;">
<h2 style="color:#cdd6f4;margin:0">%s</h2>
<p style="margin:0;text-align:center">
لا تتوفر بيانات Hashrate بعد.<br>
ستظهر البيانات تلقائياً عند اكتشاف أجهزة التعدين.
</p>
</body></html>`, title, title))
}

// round2 يُقرب قيمة إلى منزلتين عشريتين.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// =============================================================================
// EChartsServer — سيرفر HTTP محلي لخدمة الرسوم البيانية
// =============================================================================

// EChartsServer يشغّل سيرفر HTTP داخلي ويخدم رسماً بيانياً تفاعلياً
// مبنياً بـ go-echarts. يختار منفذاً عشوائياً حراً عند البدء.
//
// الاستخدام في dashboard.go:
//
//	srv, _ := charts.NewEChartsServer()
//	defer srv.Stop()
//	srv.UpdateFromCSV("device_log/total_hashrate.csv", "سجل Hashrate")
//	// افتح srv.URL() في المتصفح
type EChartsServer struct {
	mu     sync.RWMutex
	port   int
	server *http.Server
	html   []byte // HTML الحالي المُخدَّم
}

// NewEChartsServer يُنشئ ويُشغّل EChartsServer على منفذ عشوائي حر.
// استدعِ Stop() عند إغلاق التطبيق.
func NewEChartsServer() (*EChartsServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("فشل في تشغيل سيرفر الرسم البياني: %w", err)
	}

	srv := &EChartsServer{
		port: ln.Addr().(*net.TCPAddr).Port,
		html: noDataPage("سجل معدل التجزئة"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/chart", srv.serve)

	srv.server = &http.Server{Handler: mux}
	go srv.server.Serve(ln) //nolint:errcheck

	return srv, nil
}

// serve يُرسل HTML الحالي كاستجابة HTTP.
func (s *EChartsServer) serve(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	h := s.html
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache")
	w.Write(h) //nolint:errcheck
}

// UpdateFromCSV يقرأ ملف CSV ويُحدّث HTML المُخدَّم.
// آمن للاستخدام من goroutines متعددة.
func (s *EChartsServer) UpdateFromCSV(csvPath, title string) error {
	pts, err := ReadHashrateCSV(csvPath)
	if err != nil {
		return err
	}

	html, err := GenerateHashrateHTML(title, pts)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.html = html
	s.mu.Unlock()
	return nil
}

// UpdateFromPoints يُحدّث HTML مباشرة من نقاط بيانات جاهزة.
func (s *EChartsServer) UpdateFromPoints(title string, pts []CSVPoint) error {
	html, err := GenerateHashrateHTML(title, pts)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.html = html
	s.mu.Unlock()
	return nil
}

// URL يُرجع عنوان الرسم البياني الكامل.
func (s *EChartsServer) URL() string {
	return fmt.Sprintf("http://localhost:%d/chart", s.port)
}

// Stop يُوقف السيرفر بشكل نظيف.
func (s *EChartsServer) Stop() {
	if s.server != nil {
		s.server.Close() //nolint:errcheck
	}
}
