// =============================================================================
// Package goasic - مسجل بيانات الأجهزة إلى CSV
// =============================================================================
// هذا الملف يوفر وظيفة تسجيل قراءات أجهزة التعدين في ملفات CSV:
// - إنشاء ملف CSV لكل جهاز باسم IP الخاص به
// - تسجيل القراءات الرئيسية: Hashrate, Power, Temperature, Uptime
// - حفظ البيانات في مجلد device_log
// - تدوير الملفات القديمة عند الحاجة
// =============================================================================
package goasic

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultLogDir المجلد الافتراضي لسجلات الأجهزة
	DefaultLogDir = "device_log"
	// MaxLogRows الحد الأقصى للصفوف في ملف CSV قبل التدوير
	MaxLogRows = 10000
	// LogFileExtension امتداد ملفات السجل
	LogFileExtension = ".csv"
)

// CSVLogger مسجل بيانات الأجهزة إلى CSV
type CSVLogger struct {
	// المجلد الذي تُحفظ فيه الملفات
	logDir string
	
	// الملفات المفتوحة حالياً
	files   map[string]*os.File
	writers map[string]*csv.Writer
	rowCounts map[string]int
	
	// القفل لحماية الوصول المتزامن
	mu sync.RWMutex
}

// DeviceLogRecord سجل بيانات جهاز واحد
type DeviceLogRecord struct {
	Timestamp   time.Time // وقت القراءة
	IP          string    // عنوان IP للجهاز
	Model       string    // موديل الجهاز
	Hashrate    float64   // معدل التجزئة (TH/s)
	Power       int       // استهلاك الطاقة (Watts)
	Temperature float64   // درجة الحرارة (Celsius)
	Uptime      uint64    // وقت التشغيل (seconds)
	FanSpeeds   []int     // سرعات المراوح (RPM)
	Efficiency  float64   // الكفاءة (J/TH)
	IsMining    bool      // هل يعدّن حالياً
	Status      string    // حالة الجهاز
}

// NewCSVLogger ينشئ مسجل CSV جديد
func NewCSVLogger(logDir string) (*CSVLogger, error) {
	// إنشاء المجلد إذا لم يكن موجوداً
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	
	logger := &CSVLogger{
		logDir:    logDir,
		files:     make(map[string]*os.File),
		writers:   make(map[string]*csv.Writer),
		rowCounts: make(map[string]int),
	}
	
	return logger, nil
}

// Log يسجل بيانات جهاز في ملف CSV
func (l *CSVLogger) Log(record DeviceLogRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// إنشاء اسم الملف من عنوان IP
	filename := l.getFilenameFromIP(record.IP)
	filepath := filepath.Join(l.logDir, filename)
	
	// التحقق مما إذا كان الملف مفتوحاً بالفعل
	writer, exists := l.writers[filepath]
	if !exists {
		// فتح أو إنشاء الملف
		file, err := l.openOrCreateFile(filepath)
		if err != nil {
			return fmt.Errorf("failed to open file for %s: %w", record.IP, err)
		}
		
		// إنشاء CSV Writer
		writer = csv.NewWriter(file)
		
		// التحقق مما إذا كان الملف جديداً لكتابة الرأس
		isNewFile := l.rowCounts[filepath] == 0
		if isNewFile {
			// كتابة رأس CSV
			header := []string{
				"Timestamp",
				"IP",
				"Model",
				"Hashrate_TH_s",
				"Power_W",
				"Temperature_C",
				"Uptime_S",
				"FanSpeeds_RPM",
				"Efficiency_J_TH",
				"IsMining",
				"Status",
			}
			if err := writer.Write(header); err != nil {
				file.Close()
				return fmt.Errorf("failed to write CSV header: %w", err)
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				file.Close()
				return fmt.Errorf("failed to flush CSV header: %w", err)
			}
		}
		
		l.files[filepath] = file
		l.writers[filepath] = writer
	}
	
	// كتابة السجل
	recordData := []string{
		record.Timestamp.Format("2006-01-02 15:04:05"),
		record.IP,
		record.Model,
		fmt.Sprintf("%.2f", record.Hashrate),
		fmt.Sprintf("%d", record.Power),
		fmt.Sprintf("%.1f", record.Temperature),
		fmt.Sprintf("%d", record.Uptime),
		formatFanSpeeds(record.FanSpeeds),
		fmt.Sprintf("%.2f", record.Efficiency),
		fmt.Sprintf("%t", record.IsMining),
		record.Status,
	}
	
	if err := writer.Write(recordData); err != nil {
		return fmt.Errorf("failed to write CSV record: %w", err)
	}
	
	// زيادة عدد الصفوف
	l.rowCounts[filepath]++
	
	// التحقق من الحاجة للتدوير
	if l.rowCounts[filepath] >= MaxLogRows {
		if err := l.rotateFile(filepath); err != nil {
			// لا نفشل العملية، فقط نسجل الخطأ
			fmt.Printf("⚠️ WARNING: Failed to rotate log file %s: %v\n", filepath, err)
		}
	}
	
	// Flush البيانات للتأكد من كتابتها على القرص
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV data: %w", err)
	}
	
	return nil
}

// getFilenameFromIP يحول عنوان IP إلى اسم ملف صالح
func (l *CSVLogger) getFilenameFromIP(ip string) string {
	// استبدال النقاط بشرطات سفلية لإنشاء اسم ملف صالح
	safeIP := ip
	for i := 0; i < len(safeIP); i++ {
		if safeIP[i] == '.' {
			safeIP = safeIP[:i] + "_" + safeIP[i+1:]
		}
	}
	return safeIP + LogFileExtension
}

// openOrCreateFile يفتح ملفاً موجوداً أو ينشئ واحداً جديداً
func (l *CSVLogger) openOrCreateFile(filepath string) (*os.File, error) {
	// التحقق من وجود الملف
	_, err := os.Stat(filepath)
	if err == nil {
		// الملف موجود، عد الصفوف الموجودة
		count, err := l.countExistingRows(filepath)
		if err != nil {
			// إذا فشلنا في العد، نبدأ من الصفر
			l.rowCounts[filepath] = 0
		} else {
			l.rowCounts[filepath] = count
		}
	} else if os.IsNotExist(err) {
		// الملف غير موجود، نبدأ من الصفر
		l.rowCounts[filepath] = 0
	}
	
	// فتح الملف للإضافة
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	
	return file, nil
}

// countExistingRows يعد الصفوف الموجودة في ملف CSV
func (l *CSVLogger) countExistingRows(filepath string) (int, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	
	reader := csv.NewReader(file)
	count := 0
	for {
		_, err := reader.Read()
		if err != nil {
			break
		}
		count++
	}
	
	// طرح صف الرأس
	if count > 0 {
		return count - 1, nil
	}
	return 0, nil
}

// rotateFile يدور ملف السجل عند الوصول للحد الأقصى
func (l *CSVLogger) rotateFile(filepath string) error {
	// إغلاق الملف الحالي
	if file, exists := l.files[filepath]; exists {
		file.Close()
		delete(l.files, filepath)
	}
	if _, exists := l.writers[filepath]; exists {
		delete(l.writers, filepath)
	}
	
	// إعادة تسمية الملف القديم بإضافة طابع زمني
	timestamp := time.Now().Format("20060102_150405")
	rotatedPath := filepath + "." + timestamp + ".old"
	if err := os.Rename(filepath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rename old log file: %w", err)
	}
	
	// إعادة تعيين العداد
	l.rowCounts[filepath] = 0
	
	return nil
}

// formatFanSpeeds يقوم بتنسيق سرعات المراوح كنص
func formatFanSpeeds(speeds []int) string {
	if len(speeds) == 0 {
		return "N/A"
	}
	result := ""
	for i, speed := range speeds {
		if i > 0 {
			result += ";"
		}
		result += fmt.Sprintf("%d", speed)
	}
	return result
}

// Close يغلق جميع الملفات المفتوحة
func (l *CSVLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	var lastErr error
	for path, file := range l.files {
		if err := file.Close(); err != nil {
			lastErr = err
		}
		delete(l.files, path)
		delete(l.writers, path)
		delete(l.rowCounts, path)
	}
	
	return lastErr
}

// GetLogPath يعيد المسار الكامل لسجل جهاز معين
func (l *CSVLogger) GetLogPath(ip string) string {
	filename := l.getFilenameFromIP(ip)
	return filepath.Join(l.logDir, filename)
}

// GetLogDir يعيد مسار مجلد السجلات
func (l *CSVLogger) GetLogDir() string {
	return l.logDir
}

// CleanupOldLogs ينظف ملفات السجل القديمة
func (l *CSVLogger) CleanupOldLogs(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)
	
	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		// التحقق مما إذا كان ملف سجل قديم
		name := entry.Name()
		if filepath.Ext(name) == ".old" || 
		   (filepath.Ext(name) == LogFileExtension && len(name) > 20) {
			
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			if info.ModTime().Before(cutoff) {
				fullPath := filepath.Join(l.logDir, name)
				if err := os.Remove(fullPath); err != nil {
					fmt.Printf("⚠️ WARNING: Failed to remove old log %s: %v\n", fullPath, err)
				}
			}
		}
	}
	
	return nil
}

// LogFromDeviceInfo ينشئ سجل من DeviceInfo ويسجله
func (l *CSVLogger) LogFromDeviceInfo(ip, model, status string, 
	hashrate *float64, power *int, temperature []float64, 
	uptime *uint64, fanSpeeds []int, efficiency *float64, isMining bool) error {
	
	record := DeviceLogRecord{
		Timestamp: time.Now(),
		IP:        ip,
		Model:     model,
		Status:    status,
		IsMining:  isMining,
		FanSpeeds: fanSpeeds,
	}
	
	// استخراج القيم من المؤشرات
	if hashrate != nil {
		record.Hashrate = *hashrate
	}
	if power != nil {
		record.Power = *power
	}
	if len(temperature) > 0 {
		record.Temperature = temperature[0]
	}
	if uptime != nil {
		record.Uptime = *uptime
	}
	if efficiency != nil {
		record.Efficiency = *efficiency
	}
	
	return l.Log(record)
}
