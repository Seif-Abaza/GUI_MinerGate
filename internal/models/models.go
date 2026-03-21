// =============================================================================
// Package models - نماذج البيانات الرئيسية
// =============================================================================
// هذا الملف يحتوي على جميع هياكل البيانات المستخدمة في التطبيق
// بما في ذلك بيانات الأجهزة والمزرعة والإحصائيات والرسوم البيانية
// =============================================================================
package models

import (
	"fmt"
	"time"
)

// =============================================================================
// بيانات المزرعة والأجهزة
// =============================================================================

// Farm يمثل مزرعة التعدين الكاملة
type Farm struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	Location  string    `json:"location"`
	Stats     FarmStats `json:"stats"`
	Miners    []Miner   `json:"miners"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FarmStats إحصائيات المزرعة الإجمالية
type FarmStats struct {
	OnlineDevices          int     `json:"online_devices"`
	TotalDevices           int     `json:"total_devices"`
	TotalHashrate          float64 `json:"total_hashrate"`     // TH/s
	TotalHashrate24h       float64 `json:"total_hashrate_24h"` // TH/s avg 24h
	TotalPower             int     `json:"total_power"`        // Watts
	EstimatedDailyBTC      float64 `json:"estimated_daily_btc"`
	EstimatedDailyProfit   float64 `json:"estimated_daily_profit"`   // USD
	EstimatedMonthlyProfit float64 `json:"estimated_monthly_profit"` // USD
	AverageEfficiency      float64 `json:"average_efficiency"`       // J/TH
	AverageTemperature     float64 `json:"average_temperature"`      // Celsius
}

// Miner يمثل جهاز تعدين واحد
type Miner struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Model        string      `json:"model"`
	Manufacturer string      `json:"manufacturer"`
	IPAddress    string      `json:"ip_address"`
	MACAddress   string      `json:"mac_address"`
	Status       string      `json:"status"` // online, offline, error, warning
	IsMining     bool        `json:"is_mining"`
	Algorithm    string      `json:"algorithm"`
	Stats        MinerStats  `json:"stats"`
	Config       MinerConfig `json:"config"`
	LastSeen     time.Time   `json:"last_seen"`
	StartTime    time.Time   `json:"start_time"`
	Uptime       uint64      `json:"uptime"` // seconds
}

// MinerStats إحصائيات جهاز التعدين الحالية
type MinerStats struct {
	Hashrate         float64   `json:"hashrate"`          // TH/s
	Hashrate24h      float64   `json:"hashrate_24h"`      // TH/s avg
	ExpectedHashrate float64   `json:"expected_hashrate"` // TH/s
	HashratePercent  float64   `json:"hashrate_percent"`  // actual/expected * 100
	Temperature      float64   `json:"temperature"`       // Celsius
	Temperatures     []float64 `json:"temperatures"`      // Multiple temp sensors
	FanSpeeds        []int     `json:"fan_speeds"`        // RPM for each fan
	Power            int       `json:"power"`             // Watts
	PowerLimit       int       `json:"power_limit"`       // Watts
	Efficiency       float64   `json:"efficiency"`        // J/TH
	Errors           int       `json:"errors"`
	ErrorCount       int       `json:"error_count"`
	HWErrors         int       `json:"hw_errors"`
	FaultLight       bool      `json:"fault_light"`
	FanSpeedAvg      int       `json:"fan_speed_avg"`
	ChipCount        int       `json:"chip_count"`
	PoolLatency      int       `json:"pool_latency"` // ms
	SharesAccepted   int64     `json:"shares_accepted"`
	SharesRejected   int64     `json:"shares_rejected"`
	SharesStale      int64     `json:"shares_stale"`
	ShareEfficiency  float64   `json:"share_efficiency"` // %
	HashrateHistory  []float64 `json:"hashrate_history"` // Historical hashrate data for charts
}

// MinerConfig إعدادات جهاز التعدين
type MinerConfig struct {
	Pools           []PoolConfig `json:"pools"`
	PowerLimit      int          `json:"power_limit"`
	FanSpeed        int          `json:"fan_speed"`
	FanMode         string       `json:"fan_mode"` // auto, manual
	Hostname        string       `json:"hostname"`
	Firmware        string       `json:"firmware"`
	FirmwareVersion string       `json:"firmware_version"`
}

// PoolConfig إعدادات مجمع التعدين
type PoolConfig struct {
	URL      string `json:"url"`
	User     string `json:"user"`
	Password string `json:"password"`
	Priority int    `json:"priority"`
}

// =============================================================================
// البيانات التاريخية للرسوم البيانية
// =============================================================================

// HistoricalPoint نقطة بيانات تاريخية واحدة
type HistoricalPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Hashrate    float64   `json:"hashrate"`    // TH/s
	Temperature float64   `json:"temperature"` // Celsius
	Power       int       `json:"power"`       // Watts
	Efficiency  float64   `json:"efficiency"`  // J/TH
}

// ChartSeries سلسلة بيانات للرسم البياني
type ChartSeries struct {
	Name   string            `json:"name"`
	Color  string            `json:"color"`
	Points []HistoricalPoint `json:"points"`
	Unit   string            `json:"unit"`
}

// ChartData بيانات الرسم البياني الكاملة
type ChartData struct {
	Title  string        `json:"title"`
	Series []ChartSeries `json:"series"`
	XAxis  string        `json:"x_axis_label"`
	YAxis  string        `json:"y_axis_label"`
	Y2Axis string        `json:"y2_axis_label"` // للرسم البياني ثنائي المحور
	Period string        `json:"period"`        // 1d, 1w, 1m
}

// HashrateChart بيانات رسم بياني لمعدل التجزئة
type HashrateChart struct {
	MinerID    string            `json:"miner_id"`
	MinerName  string            `json:"miner_name"`
	DataPoints []HistoricalPoint `json:"data_points"`
	MinValue   float64           `json:"min_value"`
	MaxValue   float64           `json:"max_value"`
	AvgValue   float64           `json:"avg_value"`
	Period     string            `json:"period"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
}

// TempPowerChart بيانات رسم بياني لدرجة الحرارة والطاقة
type TempPowerChart struct {
	MinerID     string            `json:"miner_id"`
	MinerName   string            `json:"miner_name"`
	TempPoints  []HistoricalPoint `json:"temp_points"`
	PowerPoints []HistoricalPoint `json:"power_points"`
	TempMin     float64           `json:"temp_min"`
	TempMax     float64           `json:"temp_max"`
	PowerMin    int               `json:"power_min"`
	PowerMax    int               `json:"power_max"`
	Period      string            `json:"period"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
}

// =============================================================================
// استجابات API
// =============================================================================

// APIResponse الاستجابة العامة من API
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ErrorResponse استجابة الخطأ
type ErrorResponse struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// ActionRequest طلب تنفيذ إجراء
type ActionRequest struct {
	MinerID string                 `json:"miner_id"`
	Action  string                 `json:"action"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// ActionResponse استجابة تنفيذ الإجراء
type ActionResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// =============================================================================
// معلومات الأجهزة (من GoASIC)
// =============================================================================

// DeviceInfo معلومات الجهاز من GoASIC
type DeviceInfo struct {
	IP           string    `json:"ip"`
	Model        string    `json:"model"`
	Make         string    `json:"make"`
	Manufacturer string    `json:"manufacturer"`
	Firmware     string    `json:"firmware"`
	Algorithm    string    `json:"algorithm"`
	Hashrate     *float64  `json:"hashrate_ths,omitempty"`
	ExpectedHR   *float64  `json:"expected_hashrate_ths,omitempty"`
	Temperature  []float64 `json:"temperature,omitempty"`
	FanSpeeds    []int     `json:"fan_speeds,omitempty"`
	Wattage      *int      `json:"wattage_w,omitempty"`
	WattageLimit *int      `json:"wattage_limit_w,omitempty"`
	Efficiency   *float64  `json:"efficiency_jth,omitempty"`
	Pool1URL     string    `json:"pool1_url,omitempty"`
	Pool1User    string    `json:"pool1_user,omitempty"`
	Hostname     string    `json:"hostname,omitempty"`
	Errors       []string  `json:"errors,omitempty"`
	FaultLight   bool      `json:"fault_light"`
	IsMining     bool      `json:"is_mining"`
	Uptime       *uint64   `json:"uptime_s,omitempty"`
	ChipCount    *int      `json:"chip_count,omitempty"`
	FansCount    *int      `json:"fans_count,omitempty"`
	Cooling      string    `json:"cooling,omitempty"`
	LastScan     time.Time `json:"last_scan"`
}

// DeviceReport تقرير الجهاز للإرسال إلى API
type DeviceReport struct {
	DeviceID    string    `json:"device_id"`
	ReportTime  time.Time `json:"report_time"`
	Status      string    `json:"status"`
	Hashrate    float64   `json:"hashrate"`
	Temperature float64   `json:"temperature"`
	Power       int       `json:"power"`
	Efficiency  float64   `json:"efficiency"`
	Uptime      uint64    `json:"uptime"`
	Errors      int       `json:"errors"`
	IsMining    bool      `json:"is_mining"`
	FanSpeeds   []int     `json:"fan_speeds"`
	PoolLatency int       `json:"pool_latency"`
}

// =============================================================================
// معلومات التحديث
// =============================================================================

// UpdateInfo معلومات التحديث المتاح
type UpdateInfo struct {
	Version      string    `json:"version"`
	DownloadURL  string    `json:"download_url"`
	Checksum     string    `json:"checksum"` // SHA256
	Size         int64     `json:"size"`     // bytes
	ReleaseNotes string    `json:"release_notes"`
	Mandatory    bool      `json:"mandatory"`
	ReleaseDate  time.Time `json:"release_date"`
	Channel      string    `json:"channel"`     // stable, beta
	MinVersion   string    `json:"min_version"` // الحد الأدنى للتحديث منه
}

// UpdateStatus حالة التحديث
type UpdateStatus struct {
	CurrentVersion   string      `json:"current_version"`
	LatestVersion    string      `json:"latest_version"`
	UpdateAvailable  bool        `json:"update_available"`
	UpdateInfo       *UpdateInfo `json:"update_info,omitempty"`
	LastChecked      time.Time   `json:"last_checked"`
	Downloading      bool        `json:"downloading"`
	DownloadProgress float64     `json:"download_progress"` // 0-100
}

// =============================================================================
// معلومات الإضافات
// =============================================================================

// PluginInfo معلومات الإضافة
type PluginInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Path        string `json:"path"`
	Enabled     bool   `json:"enabled"`
	Loaded      bool   `json:"loaded"`
}

// =============================================================================
// حالة FRP
// =============================================================================

// FRPStatus حالة اتصال FRP
type FRPStatus struct {
	Connected     bool      `json:"connected"`
	ServerAddr    string    `json:"server_addr"`
	ServerPort    int       `json:"server_port"`
	ProxyName     string    `json:"proxy_name"`
	LocalPort     int       `json:"local_port"`
	LastConnected time.Time `json:"last_connected"`
	BytesSent     int64     `json:"bytes_sent"`
	BytesReceived int64     `json:"bytes_received"`
}

// =============================================================================
// حالة لوحة التحكم الكاملة
// =============================================================================

// DashboardState حالة لوحة التحكم الكاملة
type DashboardState struct {
	Farm            Farm            `json:"farm"`
	SelectedMiner   *Miner          `json:"selected_miner,omitempty"`
	HashrateChart   *HashrateChart  `json:"hashrate_chart,omitempty"`
	TempPowerChart  *TempPowerChart `json:"temp_power_chart,omitempty"`
	LastUpdate      time.Time       `json:"last_update"`
	AutoRefresh     bool            `json:"auto_refresh"`
	RefreshInterval int             `json:"refresh_interval"`
	FRPStatus       *FRPStatus      `json:"frp_status,omitempty"`
	UpdateStatus    *UpdateStatus   `json:"update_status,omitempty"`
	Plugins         []PluginInfo    `json:"plugins"`
}

// =============================================================================
// أنواع الإجراءات
// =============================================================================

// MinerActionType أنواع الإجراءات المتاحة
type MinerActionType string

const (
	ActionRestart     MinerActionType = "restart"
	ActionReboot      MinerActionType = "reboot"
	ActionStopMining  MinerActionType = "stop_mining"
	ActionStartMining MinerActionType = "start_mining"
	ActionGetErrors   MinerActionType = "get_errors"
	ActionGetPools    MinerActionType = "get_pools"
	ActionFanStatus   MinerActionType = "fan_status"
	ActionPowerInfo   MinerActionType = "power_info"
	ActionLightOn     MinerActionType = "fault_light_on"
	ActionLightOff    MinerActionType = "fault_light_off"
)

// MinerAction معلومات إجراء متاح
type MinerAction struct {
	Type        MinerActionType `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Icon        string          `json:"icon"`
	Dangerous   bool            `json:"dangerous"`
	Enabled     bool            `json:"enabled"`
}

// =============================================================================
// دوال مساعدة
// =============================================================================

// GetMinerActions يعيد قائمة الإجراءات المتاحة
func GetMinerActions() []MinerAction {
	return []MinerAction{
		{Type: ActionRestart, Name: "Restart", Description: "Restart the miner", Icon: "restart", Dangerous: true, Enabled: true},
		{Type: ActionGetErrors, Name: "Get Errors", Description: "Get error logs", Icon: "errors", Dangerous: false, Enabled: true},
		{Type: ActionFanStatus, Name: "Fan Status", Description: "Get fan status", Icon: "fan", Dangerous: false, Enabled: true},
		{Type: ActionGetPools, Name: "Get Pools", Description: "Get pool configuration", Icon: "pools", Dangerous: false, Enabled: true},
		{Type: ActionPowerInfo, Name: "Power Info", Description: "Get power information", Icon: "power", Dangerous: false, Enabled: true},
	}
}

// IsOnline يتحقق مما إذا كان الجهاز متصلاً
func (m *Miner) IsOnline() bool {
	return m.Status == "online"
}

// GetUptimeString يعيد وقت التشغيل كنص مقروء
func (m *Miner) GetUptimeString() string {
	uptime := m.Uptime
	days := uptime / 86400
	hours := (uptime % 86400) / 3600
	minutes := (uptime % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// CalculateEfficiency يحسب الكفاءة (J/TH)
func (s *MinerStats) CalculateEfficiency() float64 {
	if s.Hashrate > 0 {
		return float64(s.Power) / s.Hashrate
	}
	return 0
}

// CalculateHashratePercent يحسب نسبة معدل التجزئة من المتوقع
func (s *MinerStats) CalculateHashratePercent() float64 {
	if s.ExpectedHashrate > 0 {
		return (s.Hashrate / s.ExpectedHashrate) * 100
	}
	return 0
}
