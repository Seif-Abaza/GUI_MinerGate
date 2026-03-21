// =============================================================================
// Package goasic - مدير أجهزة التعدين ASIC
// =============================================================================
// هذا الملف يوفر تكامل GoASIC مع التطبيق:
// - مسح الشبكة لأجهزة التعدين
// - جمع البيانات من الأجهزة
// - إرسال البيانات إلى API
// - إدارة الأجهزة المكتشفة
// =============================================================================
package goasic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"minergate/internal/config"
	"minergate/internal/models"

	"github.com/goasic/goasic"
)

var (
	// hooks for unit tests
	scanSubnetFn  = goasic.ScanSubnet
	detectMinerFn = goasic.Detect
)

// =============================================================================
// هيئات البيانات
// =============================================================================

// DeviceStatus حالة الجهاز
type DeviceStatus string

const (
	StatusOnline   DeviceStatus = "online"
	StatusOffline  DeviceStatus = "offline"
	StatusError    DeviceStatus = "error"
	StatusScanning DeviceStatus = "scanning"
)

// DiscoveredDevice جهاز مكتشف
type DiscoveredDevice struct {
	IP       string             `json:"ip"`
	Model    string             `json:"model"`
	Make     string             `json:"make"`
	Status   DeviceStatus       `json:"status"`
	LastSeen time.Time          `json:"last_seen"`
	Data     *models.DeviceInfo `json:"data,omitempty"`
}

// Manager مدير أجهزة GoASIC
type Manager struct {
	// التكوين
	cfg *config.Config

	// الأجهزة المكتشفة
	devices   map[string]*DiscoveredDevice
	devicesMu sync.RWMutex

	// القنوات
	stopChan chan struct{}
	scanChan chan string

	// مسجل CSV
	csvLogger *CSVLogger

	// callbacks
	onDeviceDiscovered func(device *DiscoveredDevice)
	onDeviceLost       func(ip string)
	onDeviceUpdate     func(device *DiscoveredDevice)
	onError            func(error)

	// حالة المسح
	scanning   bool
	scanActive bool
	lastScan   time.Time
}

// NewManager ينشئ مدير أجهزة جديد
func NewManager(cfg *config.Config) *Manager {
	// إنشاء مسجل CSV
	csvLogger, err := NewCSVLogger(DefaultLogDir)
	if err != nil {
		fmt.Printf("⚠️ WARNING: Failed to create CSV logger: %v\n", err)
	}
	
	return &Manager{
		cfg:       cfg,
		devices:   make(map[string]*DiscoveredDevice),
		stopChan:  make(chan struct{}),
		scanChan:  make(chan string, 100),
		csvLogger: csvLogger,
	}
}

// =============================================================================
// العمليات الأساسية
// =============================================================================

// Start يبدأ مدير الأجهزة
func (m *Manager) Start(ctx context.Context) error {
	if !m.cfg.GoASICEnabled {
		return nil // GoASIC غير مفعل
	}

	// بدء المسح الدوري
	go m.scanLoop(ctx)

	// بدء جمع البيانات
	go m.dataCollectionLoop(ctx)

	return nil
}

// Stop يوقف مدير الأجهزة
func (m *Manager) Stop() {
	select {
	case <-m.stopChan:
		//Already closed
	default:
		close(m.stopChan)
	}
}

// =============================================================================
// المسح والاكتشاف
// =============================================================================

// scanLoop حلقة المسح الدوري
func (m *Manager) scanLoop(ctx context.Context) {
	// مسح أولي
	m.ScanNetwork(ctx)

	// مسح دوري
	ticker := time.NewTicker(time.Duration(m.cfg.GoASICScanInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.ScanNetwork(ctx)
		}
	}
}

// ScanNetwork يمسح الشبكة للأجهزة
func (m *Manager) ScanNetwork(ctx context.Context) error {
	m.devicesMu.Lock()
	m.scanning = true
	m.scanActive = true
	m.devicesMu.Unlock()

	defer func() {
		m.devicesMu.Lock()
		m.scanning = false
		m.scanActive = false
		m.lastScan = time.Now()
		m.devicesMu.Unlock()
	}()

	// استخدام goasic.ScanSubnet لمسح الشبكة
	maxConcurrent := 50 // يمكن تعديله حسب التكوين
	miners, err := scanSubnetFn(ctx, m.cfg.GoASICNetworkRange, maxConcurrent)
	if err != nil {
		if m.onError != nil {
			m.onError(fmt.Errorf("failed to scan subnet: %w", err))
		}
		return err
	}

	// معالجة الأجهزة المكتشفة
	for _, miner := range miners {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// الحصول على بيانات الجهاز
		data, err := miner.GetData(ctx)
		if err != nil {
			fmt.Printf("⚠️ DEBUG: Failed to get data for miner %s: %v\n", miner.IP(), err)
			// تسجيل الخطأ لكن الاستمرار
			if m.onError != nil {
				m.onError(fmt.Errorf("failed to get data for miner %s: %w", miner.IP(), err))
			}
			// Add it anyway with Error status
			errDevice := &DiscoveredDevice{
				IP:       miner.IP(),
				Model:    "Unknown",
				Make:     "Unknown",
				Status:   StatusError,
				LastSeen: time.Now(),
			}
			m.addOrUpdateDevice(errDevice)
			continue
		}

		// إنشاء DiscoveredDevice من البيانات
		device := &DiscoveredDevice{
			IP:       data.IP,
			Model:    data.Model,
			Make:     data.Make,
			Status:   StatusOnline,
			LastSeen: time.Now(),
			Data:     convertMinerDataToDeviceInfo(data),
		}

		m.addOrUpdateDevice(device)
	}

	return nil
}

// convertMinerDataToDeviceInfo يحول MinerData إلى DeviceInfo
func convertMinerDataToDeviceInfo(data *goasic.MinerData) *models.DeviceInfo {
	deviceInfo := &models.DeviceInfo{
		IP:           data.IP,
		Model:        data.Model,
		Make:         data.Make,
		Manufacturer: data.Make, // استخدام Make كـ Manufacturer
		Firmware:     data.Firmware,
		Algorithm:    data.Algorithm,
		Hashrate:     data.Hashrate,
		ExpectedHR:   data.ExpectedHashrate,
		Temperature:  data.Temperature,
		FanSpeeds:    data.FanSpeeds,
		Wattage:      data.Wattage,
		WattageLimit: data.WattageLimit,
		Efficiency:   data.Efficiency,
		Pool1URL:     data.Pool1URL,
		Pool1User:    data.Pool1User,
		Hostname:     data.Hostname,
		Errors:       data.Errors,
		FaultLight:   data.FaultLight,
		IsMining:     data.IsMining,
		Uptime:       data.Uptime,
		ChipCount:    data.ChipCount,
		FansCount:    data.FansCount,
		Cooling:      data.Cooling,
		LastScan:     time.Now(),
	}

	return deviceInfo
}

// addOrUpdateDevice يضيف أو يحدث جهاز
func (m *Manager) addOrUpdateDevice(device *DiscoveredDevice) {
	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	existing, exists := m.devices[device.IP]
	if exists {
		// تحديث الجهاز الموجود
		existing.Model = device.Model
		existing.Make = device.Make
		existing.Status = device.Status
		existing.LastSeen = device.LastSeen
		if device.Data != nil {
			existing.Data = device.Data
		}
		if m.onDeviceUpdate != nil {
			m.onDeviceUpdate(existing)
		}
	} else {
		// إضافة جهاز جديد
		m.devices[device.IP] = device
		if m.onDeviceDiscovered != nil {
			m.onDeviceDiscovered(device)
		}
	}
}

// =============================================================================
// جمع البيانات
// =============================================================================

// dataCollectionLoop حلقة جمع البيانات
func (m *Manager) dataCollectionLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectAllDeviceData(ctx)
		}
	}
}

// collectAllDeviceData يجمع بيانات جميع الأجهزة
func (m *Manager) collectAllDeviceData(ctx context.Context) {
	m.devicesMu.RLock()
	devices := make([]*DiscoveredDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	m.devicesMu.RUnlock()

	var wg sync.WaitGroup
	for _, device := range devices {
		wg.Add(1)
		go func(d *DiscoveredDevice) {
			defer wg.Done()
			m.collectDeviceData(ctx, d)
		}(device)
	}
	wg.Wait()
}

// collectDeviceData يجمع بيانات جهاز واحد
func (m *Manager) collectDeviceData(ctx context.Context, device *DiscoveredDevice) {
	// استخدام goasic.Detect للحصول على miner
	miner, err := detectMinerFn(ctx, device.IP)
	if err != nil {
		if m.onError != nil {
			m.onError(fmt.Errorf("failed to detect miner at %s: %w", device.IP, err))
		}
		// تحديث حالة الجهاز إلى error
		m.devicesMu.Lock()
		if existing, ok := m.devices[device.IP]; ok {
			existing.Status = StatusError
		}
		m.devicesMu.Unlock()
		return
	}

	// جمع البيانات من الجهاز
	data, err := miner.GetData(ctx)
	if err != nil {
		if m.onError != nil {
			m.onError(fmt.Errorf("failed to get data for miner %s: %w", device.IP, err))
		}
		return
	}

	// تحديث بيانات الجهاز
	deviceInfo := convertMinerDataToDeviceInfo(data)

	m.devicesMu.Lock()
	if existing, ok := m.devices[device.IP]; ok {
		existing.Data = deviceInfo
		existing.LastSeen = time.Now()
		existing.Status = StatusOnline
	}
	m.devicesMu.Unlock()

	if m.onDeviceUpdate != nil {
		m.onDeviceUpdate(device)
	}
}

// =============================================================================
// الحصول على الأجهزة
// =============================================================================

// GetDevices يعيد جميع الأجهزة المكتشفة
func (m *Manager) GetDevices() []*DiscoveredDevice {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	devices := make([]*DiscoveredDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetDevice يعيد جهاز محدد
func (m *Manager) GetDevice(ip string) *DiscoveredDevice {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()
	return m.devices[ip]
}

// GetDeviceCount يعيد عدد الأجهزة
func (m *Manager) GetDeviceCount() int {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()
	return len(m.devices)
}

// GetOnlineCount يعيد عدد الأجهزة المتصلة
func (m *Manager) GetOnlineCount() int {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	count := 0
	for _, d := range m.devices {
		if d.Status == StatusOnline {
			count++
		}
	}
	return count
}

// =============================================================================
// التقارير
// =============================================================================

// GenerateDeviceReports يولد تقارير الأجهزة
func (m *Manager) GenerateDeviceReports() []models.DeviceReport {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()

	reports := make([]models.DeviceReport, 0, len(m.devices))
	for ip, device := range m.devices {
		report := models.DeviceReport{
			DeviceID:   ip,
			ReportTime: time.Now(),
			Status:     string(device.Status),
		}

		if device.Data != nil {
			if device.Data.Hashrate != nil {
				report.Hashrate = *device.Data.Hashrate
			}
			if device.Data.Temperature != nil && len(device.Data.Temperature) > 0 {
				report.Temperature = device.Data.Temperature[0]
			}
			if device.Data.Wattage != nil {
				report.Power = *device.Data.Wattage
			}
			if device.Data.Efficiency != nil {
				report.Efficiency = *device.Data.Efficiency
			}
			if device.Data.Uptime != nil {
				report.Uptime = *device.Data.Uptime
			}
			report.IsMining = device.Data.IsMining
			report.FanSpeeds = device.Data.FanSpeeds
		}

		reports = append(reports, report)
	}

	return reports
}

// =============================================================================
// Callbacks
// =============================================================================

// OnDeviceDiscovered يضبط callback اكتشاف جهاز
func (m *Manager) OnDeviceDiscovered(callback func(device *DiscoveredDevice)) {
	m.onDeviceDiscovered = callback
}

// OnDeviceLost يضبط callback فقدان جهاز
func (m *Manager) OnDeviceLost(callback func(ip string)) {
	m.onDeviceLost = callback
}

// OnDeviceUpdate يضبط callback تحديث جهاز
func (m *Manager) OnDeviceUpdate(callback func(device *DiscoveredDevice)) {
	m.onDeviceUpdate = callback
}

// OnError يضبط callback الأخطاء
func (m *Manager) OnError(callback func(error)) {
	m.onError = callback
}

// =============================================================================
// حالة المسح
// =============================================================================

// IsScanning يتحقق مما إذا كان المسح جارياً
func (m *Manager) IsScanning() bool {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()
	return m.scanning
}

// GetLastScanTime يعيد وقت آخر مسح
func (m *Manager) GetLastScanTime() time.Time {
	m.devicesMu.RLock()
	defer m.devicesMu.RUnlock()
	return m.lastScan
}
