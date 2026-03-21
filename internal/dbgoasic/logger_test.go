// =============================================================================
// Package goasic - اختبارات مسجل CSV
// =============================================================================
package goasic

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewCSVLogger(t *testing.T) {
	// إنشاء مجلد مؤقت للاختبار
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	// التحقق من أن المجلد تم إنشاؤه بشكل صحيح
	if logger.GetLogDir() != tmpDir {
		t.Fatalf("expected log dir %s, got %s", tmpDir, logger.GetLogDir())
	}
}

func TestNewCSVLogger_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "new_log_dir")
	
	logger, err := NewCSVLogger(newDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	// التحقق من أن المجلد تم إنشاؤه
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Fatal("expected log directory to be created")
	}
}

func TestCSVLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	hashrate := 120.5
	power := 3200
	temp := 75.3
	uptime := uint64(86400)
	fanSpeeds := []int{1200, 1300}
	efficiency := 9.5
	
	record := DeviceLogRecord{
		Timestamp:   time.Now(),
		IP:          "10.0.0.1",
		Model:       "Antminer S19",
		Hashrate:    hashrate,
		Power:       power,
		Temperature: temp,
		Uptime:      uptime,
		FanSpeeds:   fanSpeeds,
		Efficiency:  efficiency,
		IsMining:    true,
		Status:      "online",
	}
	
	// تسجيل البيانات
	if err := logger.Log(record); err != nil {
		t.Fatalf("failed to log record: %v", err)
	}
	
	// التحقق من أن الملف تم إنشاؤه
	expectedFile := filepath.Join(tmpDir, "10_0_0_1.csv")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatal("expected log file to be created")
	}
	
	// قراءة الملف والتحقق من المحتوى
	file, err := os.Open(expectedFile)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()
	
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}
	
	// يجب أن يكون هناك صف الرأس + صف البيانات
	if len(records) != 2 {
		t.Fatalf("expected 2 records (header + data), got %d", len(records))
	}
	
	// التحقق من الرأس
	header := records[0]
	expectedHeader := []string{
		"Timestamp", "IP", "Model", "Hashrate_TH_s", "Power_W",
		"Temperature_C", "Uptime_S", "FanSpeeds_RPM", "Efficiency_J_TH",
		"IsMining", "Status",
	}
	if len(header) != len(expectedHeader) {
		t.Fatalf("expected header length %d, got %d", len(expectedHeader), len(header))
	}
	
	// التحقق من البيانات
	data := records[1]
	if data[0] == "" {
		t.Error("expected timestamp to be set")
	}
	if data[1] != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", data[1])
	}
	if data[2] != "Antminer S19" {
		t.Errorf("expected model Antminer S19, got %s", data[2])
	}
	if data[3] != "120.50" {
		t.Errorf("expected hashrate 120.50, got %s", data[3])
	}
	if data[4] != "3200" {
		t.Errorf("expected power 3200, got %s", data[4])
	}
	if data[5] != "75.3" {
		t.Errorf("expected temperature 75.3, got %s", data[5])
	}
	if data[6] != "86400" {
		t.Errorf("expected uptime 86400, got %s", data[6])
	}
	if !strings.Contains(data[7], "1200") || !strings.Contains(data[7], "1300") {
		t.Errorf("expected fan speeds to contain 1200 and 1300, got %s", data[7])
	}
	if data[8] != "9.50" {
		t.Errorf("expected efficiency 9.50, got %s", data[8])
	}
	if data[9] != "true" {
		t.Errorf("expected IsMining true, got %s", data[9])
	}
	if data[10] != "online" {
		t.Errorf("expected status online, got %s", data[10])
	}
}

func TestCSVLogger_LogFromDeviceInfo(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	hashrate := 100.0
	power := 3000
	temp := []float64{70.0}
	uptime := uint64(3600)
	fanSpeeds := []int{1000}
	efficiency := 10.0
	
	err = logger.LogFromDeviceInfo(
		"10.0.0.2",
		"Antminer T19",
		"online",
		&hashrate,
		&power,
		temp,
		&uptime,
		fanSpeeds,
		&efficiency,
		true,
	)
	if err != nil {
		t.Fatalf("failed to log from DeviceInfo: %v", err)
	}
	
	// التحقق من أن الملف تم إنشاؤه
	expectedFile := filepath.Join(tmpDir, "10_0_0_2.csv")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatal("expected log file to be created")
	}
}

func TestCSVLogger_LogFromDeviceInfo_NilValues(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	// اختبار مع قيم nil
	err = logger.LogFromDeviceInfo(
		"10.0.0.3",
		"Unknown",
		"offline",
		nil, // hashrate
		nil, // power
		nil, // temperature
		nil, // uptime
		nil, // fanSpeeds
		nil, // efficiency
		false,
	)
	if err != nil {
		t.Fatalf("failed to log with nil values: %v", err)
	}
}

func TestCSVLogger_MultipleLogs(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	hashrate := 120.5
	power := 3200
	temp := 75.3
	uptime := uint64(86400)
	
	// تسجيل عدة قراءات لنفس الجهاز
	for i := 0; i < 5; i++ {
		record := DeviceLogRecord{
			Timestamp:   time.Now(),
			IP:          "10.0.0.1",
			Model:       "Antminer S19",
			Hashrate:    hashrate + float64(i),
			Power:       power,
			Temperature: temp,
			Uptime:      uptime,
			FanSpeeds:   []int{1200},
			Efficiency:  9.5,
			IsMining:    true,
			Status:      "online",
		}
		
		if err := logger.Log(record); err != nil {
			t.Fatalf("failed to log record %d: %v", i, err)
		}
	}
	
	// التحقق من عدد الصفوف
	expectedFile := filepath.Join(tmpDir, "10_0_0_1.csv")
	file, err := os.Open(expectedFile)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()
	
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}
	
	// يجب أن يكون هناك صف الرأس + 5 صفوف بيانات
	if len(records) != 6 {
		t.Fatalf("expected 6 records (header + 5 data), got %d", len(records))
	}
}

func TestCSVLogger_MultipleDevices(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	
	// تسجيل بيانات لعدة أجهزة
	for _, ip := range ips {
		record := DeviceLogRecord{
			Timestamp:   time.Now(),
			IP:          ip,
			Model:       "Antminer S19",
			Hashrate:    120.5,
			Power:       3200,
			Temperature: 75.3,
			Uptime:      86400,
			FanSpeeds:   []int{1200},
			Efficiency:  9.5,
			IsMining:    true,
			Status:      "online",
		}
		
		if err := logger.Log(record); err != nil {
			t.Fatalf("failed to log record for %s: %v", ip, err)
		}
	}
	
	// التحقق من أن كل جهاز له ملف منفصل
	for _, ip := range ips {
		filename := strings.Replace(ip, ".", "_", -1) + ".csv"
		expectedFile := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("expected log file for %s to be created", ip)
		}
	}
}

func TestCSVLogger_GetLogPath(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	ip := "10.0.0.1"
	expectedPath := filepath.Join(tmpDir, "10_0_0_1.csv")
	
	gotPath := logger.GetLogPath(ip)
	if gotPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, gotPath)
	}
}

func TestCSVLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	
	// تسجيل بعض البيانات
	record := DeviceLogRecord{
		Timestamp:   time.Now(),
		IP:          "10.0.0.1",
		Model:       "Antminer S19",
		Hashrate:    120.5,
		Power:       3200,
		Temperature: 75.3,
		Uptime:      86400,
		FanSpeeds:   []int{1200},
		Efficiency:  9.5,
		IsMining:    true,
		Status:      "online",
	}
	
	if err := logger.Log(record); err != nil {
		t.Fatalf("failed to log record: %v", err)
	}
	
	// إغلاق المسجل
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}
	
	// التحقق من أن الملفات أُغلقت (يمكن قراءة الملف بنجاح)
	expectedFile := filepath.Join(tmpDir, "10_0_0_1.csv")
	file, err := os.Open(expectedFile)
	if err != nil {
		t.Fatalf("failed to open log file after close: %v", err)
	}
	defer file.Close()
}

func TestFormatFanSpeeds(t *testing.T) {
	tests := []struct {
		name     string
		speeds   []int
		expected string
	}{
		{"empty", []int{}, "N/A"},
		{"single", []int{1200}, "1200"},
		{"multiple", []int{1200, 1300, 1400}, "1200;1300;1400"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFanSpeeds(tt.speeds)
			if got != tt.expected {
				t.Errorf("formatFanSpeeds(%v) = %s, want %s", tt.speeds, got, tt.expected)
			}
		})
	}
}

func TestGetFilenameFromIP(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	tests := []struct {
		ip       string
		expected string
	}{
		{"10.0.0.1", "10_0_0_1.csv"},
		{"192.168.1.100", "192_168_1_100.csv"},
		{"1.2.3.4", "1_2_3_4.csv"},
	}
	
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := logger.getFilenameFromIP(tt.ip)
			if got != tt.expected {
				t.Errorf("getFilenameFromIP(%s) = %s, want %s", tt.ip, got, tt.expected)
			}
		})
	}
}

func TestCSVLogger_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	
	logger, err := NewCSVLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create CSV logger: %v", err)
	}
	defer logger.Close()
	
	done := make(chan bool)
	errors := make(chan error, 10)
	
	// تشغيل عدة goroutines للكتابة في نفس الوقت
	for i := 0; i < 10; i++ {
		go func(id int) {
			ip := "10.0.0." + strconv.Itoa(id%3+1)
			record := DeviceLogRecord{
				Timestamp:   time.Now(),
				IP:          ip,
				Model:       "Antminer S19",
				Hashrate:    120.5,
				Power:       3200,
				Temperature: 75.3,
				Uptime:      86400,
				FanSpeeds:   []int{1200},
				Efficiency:  9.5,
				IsMining:    true,
				Status:      "online",
			}
			
			if err := logger.Log(record); err != nil {
				errors <- err
				return
			}
			done <- true
		}(i)
	}
	
	// انتظار اكتمال جميع العمليات
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// نجاح
		case err := <-errors:
			t.Fatalf("concurrent write failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent writes")
		}
	}
}
