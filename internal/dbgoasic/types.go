// =============================================================================
// Package goasic - نماذج البيانات
// =============================================================================
package goasic

import (
	"time"
)

// DeviceData بيانات الجهاز من GoASIC
type DeviceData struct {
	IP           string    `json:"ip"`
	Model        string    `json:"model"`
	Make         string    `json:"make"`
	Manufacturer string    `json:"manufacturer"`
	Firmware     string    `json:"firmware"`
	Algorithm    string    `json:"algorithm"`
	Hashrate     float64   `json:"hashrate"`
	ExpectedHR   float64   `json:"expected_hashrate"`
	Temperature  float64   `json:"temperature"`
	Temperatures []float64 `json:"temperatures"`
	FanSpeeds    []int     `json:"fan_speeds"`
	Wattage      int       `json:"wattage"`
	WattageLimit int       `json:"wattage_limit"`
	Efficiency   float64   `json:"efficiency"`
	Pool1URL     string    `json:"pool1_url"`
	Pool1User    string    `json:"pool1_user"`
	Hostname     string    `json:"hostname"`
	Errors       []string  `json:"errors"`
	ErrorCount   int       `json:"error_count"`
	FaultLight   bool      `json:"fault_light"`
	IsMining     bool      `json:"is_mining"`
	Uptime       uint64    `json:"uptime"`
	ChipCount    int       `json:"chip_count"`
	FansCount    int       `json:"fans_count"`
	Cooling      string    `json:"cooling"`
	LastScan     time.Time `json:"last_scan"`
}

// ScanResult نتيجة مسح الشبكة
type ScanResult struct {
	IP           string        `json:"ip"`
	Found        bool          `json:"found"`
	Model        string        `json:"model"`
	Make         string        `json:"make"`
	Status       DeviceStatus  `json:"status"`
	ResponseTime time.Duration `json:"response_time"`
}

// MinerCommand أمر للمُعدّن
type MinerCommand struct {
	Command string                 `json:"command"`
	Params  map[string]interface{} `json:"params"`
}

// MinerResponse استجابة من المُعدّن
type MinerResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   string      `json:"error,omitempty"`
}
