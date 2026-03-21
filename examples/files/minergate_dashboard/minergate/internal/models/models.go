// Package models defines the data structures used throughout MinerGate.
package models

import "time"

// ─────────────────────────────────────────────────────────────────────────────
// Pool
// ─────────────────────────────────────────────────────────────────────────────

// Pool represents one mining-pool entry returned by the pools CGI / API.
type Pool struct {
	Index    int    `json:"POOL"`
	URL      string `json:"url"`
	User     string `json:"user"`
	Status   string `json:"status"`   // "Alive" | "Dead"
	Priority int    `json:"priority"`
	Accepted uint   `json:"accepted"`
	Rejected uint   `json:"rejected"`
	Stale    uint   `json:"stale"`
	DiffA    string `json:"diffa"`
	DiffR    string `json:"diffr"`
	LSDiff   int64  `json:"lsdiff"`
	LSTime   string `json:"lstime"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Chain (hash-board)
// ─────────────────────────────────────────────────────────────────────────────

// Chain represents one hash-board as returned by the stats CGI / API.
type Chain struct {
	Index        int       `json:"index"`
	AsicNum      int       `json:"asic_num"`
	HW           int       `json:"hw"`
	FreqAvg      int       `json:"freq_avg"`
	RateReal     float64   `json:"rate_real"`
	RateIdeal    float64   `json:"rate_ideal"`
	RateUnit     string    `json:"rate_unit"`
	TempPic      string    `json:"temp_pic"`
	TempChip     string    `json:"temp_chip"`
	PCBState     string    `json:"pcb_state"`
	PCBStateXNum int       `json:"pcb_stateXnum"`
	Asic         string    `json:"asic"`     // space-separated "oooo" pattern
	TPL          [][]int   `json:"tpl"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Warning
// ─────────────────────────────────────────────────────────────────────────────

// Warning is one entry from the warning CGI / API.
type Warning struct {
	Code       string `json:"code"`
	Cause      string `json:"cause"`
	Suggestion string `json:"suggestion"`
	Timestamp  string `json:"timestamp"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Summary
// ─────────────────────────────────────────────────────────────────────────────

// StatusCard is one entry in the SUMMARY[0].status array used by the dashboard.
type StatusCard struct {
	Type   string  `json:"type"`   // "rate" | "network" | "fans" | "temp"
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Status string  `json:"status"` // "s" = normal | "u" = abnormal
	Fan1   int     `json:"fan1"`
	Fan2   int     `json:"fan2"`
	Fan3   int     `json:"fan3"`
	Fan4   int     `json:"fan4"`
}

// ElapsedTime holds the decomposed uptime used by the dashboard.
type ElapsedTime struct {
	Day  int `json:"day"`
	Hour int `json:"hour"`
	Min  int `json:"min"`
	Sec  int `json:"sec"`
}

// Summary holds the parsed SUMMARY[0] block.
type Summary struct {
	// Uptime in seconds (raw from API)
	Elapsed int `json:"elapsed"`

	// Hashrate
	Rate5s   float64 `json:"rate_5s"`
	RateAvg  float64 `json:"rate_avg"`
	RateUnit string  `json:"rate_unit"`

	// Shares
	RejectRatio string `json:"reject_ratio"`
	Accepted    uint   `json:"Accepted"`
	Rejected    uint   `json:"Rejected"`
	HWErrors    int    `json:"Hardware Errors"`

	// Temperature / fans
	Temperature float64 `json:"Temperature"`
	Fan1        int     `json:"fan1"`
	Fan2        int     `json:"fan2"`
	Fan3        int     `json:"fan3"`
	Fan4        int     `json:"fan4"`
	FanNum      int     `json:"fan_num"`

	// Power
	Power float64 `json:"Power"`

	// Status cards for the four overview tiles
	StatusCards []StatusCard `json:"status"`

	// Miner identity
	MinerType string `json:"miner_type"`
}

// ─────────────────────────────────────────────────────────────────────────────
// StatsResponse
// ─────────────────────────────────────────────────────────────────────────────

// StatsResponse is the top-level object returned by the stats CGI / API.
type StatsResponse struct {
	Status struct {
		STATUS string `json:"STATUS"`
	} `json:"STATUS"`
	Info struct {
		Type          string `json:"type"`
		MinerVersion  string `json:"miner_version"`
		CompileTime   string `json:"CompileTime"`
	} `json:"INFO"`
	Stats []struct {
		// Common fields
		Elapsed int     `json:"elapsed"`
		GHS5s   float64 `json:"GHS 5s"`
		GHSAv   float64 `json:"GHS av"`
		Rate5s  float64 `json:"rate_5s"`
		RateAvg float64 `json:"rate_avg"`
		RateUnit string `json:"rate_unit"`
		TempMax int     `json:"temp_max"`
		TempAvg int     `json:"temp_avg"`
		FanNum  int     `json:"fan_num"`
		Fan     []int   `json:"fan"`
		Fan1    int     `json:"fan1"`
		Fan2    int     `json:"fan2"`
		Fan3    int     `json:"fan3"`
		Fan4    int     `json:"fan4"`
		Chain   []Chain `json:"chain"`
		Power   float64 `json:"power"`
	} `json:"STATS"`
}

// ─────────────────────────────────────────────────────────────────────────────
// SummaryResponse
// ─────────────────────────────────────────────────────────────────────────────

// SummaryResponse is the top-level object returned by the summary CGI / API.
type SummaryResponse struct {
	Status struct {
		STATUS string `json:"STATUS"`
	} `json:"STATUS"`
	Summary []struct {
		Elapsed     int          `json:"elapsed"`
		Rate5s      float64      `json:"rate_5s"`
		RateAvg     float64      `json:"rate_avg"`
		RateUnit    string       `json:"rate_unit"`
		RejectRatio float64      `json:"reject_ratio"`
		Accepted    uint         `json:"Accepted"`
		Rejected    uint         `json:"Rejected"`
		HWErrors    int          `json:"Hardware Errors"`
		Temperature float64      `json:"Temperature"`
		Power       float64      `json:"Power"`
		Fan1        int          `json:"fan1"`
		Fan2        int          `json:"fan2"`
		Fan3        int          `json:"fan3"`
		Fan4        int          `json:"fan4"`
		FanNum      int          `json:"fan_num"`
		MinerType   string       `json:"miner_type"`
		StatusCards []StatusCard `json:"status"`
	} `json:"SUMMARY"`
}

// ─────────────────────────────────────────────────────────────────────────────
// PoolsResponse
// ─────────────────────────────────────────────────────────────────────────────

// PoolsResponse is the top-level object returned by pools CGI / API.
type PoolsResponse struct {
	Status struct {
		STATUS string `json:"STATUS"`
	} `json:"STATUS"`
	Pools []Pool `json:"POOLS"`
}

// ─────────────────────────────────────────────────────────────────────────────
// WarningResponse
// ─────────────────────────────────────────────────────────────────────────────

// WarningResponse is the top-level object returned by warning CGI / API.
type WarningResponse struct {
	Status struct {
		STATUS string `json:"STATUS"`
	} `json:"STATUS"`
	Warnings []Warning `json:"WARNING"`
}

// ─────────────────────────────────────────────────────────────────────────────
// SystemInfo
// ─────────────────────────────────────────────────────────────────────────────

// SystemInfo is returned by get_system_info.cgi.
type SystemInfo struct {
	MinerType              string `json:"minertype"`
	NetType                string `json:"nettype"`
	MacAddr                string `json:"macaddr"`
	Hostname               string `json:"hostname"`
	IPAddress              string `json:"ipaddress"`
	NetMask                string `json:"netmask"`
	Gateway                string `json:"gateway"`
	DNSServers             string `json:"dnsservers"`
	SystemMode             string `json:"system_mode"`
	SystemKernelVersion    string `json:"system_kernel_version"`
	SystemFilesystemVersion string `json:"system_filesystem_version"`
	FirmwareType           string `json:"firmware_type"`
	SerialNumber           string `json:"serinum"`
}

// ─────────────────────────────────────────────────────────────────────────────
// RateResponse  (chart.cgi)
// ─────────────────────────────────────────────────────────────────────────────

// RateSeries is one series inside the chart response.
type RateSeries struct {
	Name string    `json:"name"`
	Type string    `json:"type"`
	Data []float64 `json:"data"`
}

// RateResponse is the top-level object returned by chart.cgi / rate command.
type RateResponse struct {
	Status struct {
		STATUS string `json:"STATUS"`
	} `json:"STATUS"`
	Rate []struct {
		Unit   string       `json:"unit"`
		XAxis  []string     `json:"xAxis"`
		Series []RateSeries `json:"series"`
	} `json:"RATE"`
}

// ─────────────────────────────────────────────────────────────────────────────
// HashRatePoint  (CSV persistence)
// ─────────────────────────────────────────────────────────────────────────────

// HashRatePoint is one time-stamped hashrate sample stored in the CSV file.
type HashRatePoint struct {
	Timestamp time.Time
	Total     float64   // TH/s
	Unit      string    // "TH/s" | "GH/s"
	Chains    []float64 // per-chain values (same unit)
}

// ─────────────────────────────────────────────────────────────────────────────
// MinerDevice  (aggregated device)
// ─────────────────────────────────────────────────────────────────────────────

// MinerDevice represents one miner being monitored.
type MinerDevice struct {
	// Identity
	ID         string
	Host       string
	Port       int
	MinerType  string
	Serial     string
	Firmware   string

	// Connection
	Online    bool
	LastSeen  time.Time
	ErrorMsg  string

	// Live stats
	Summary  *Summary
	Stats    *StatsResponse
	Pools    []Pool
	Warnings []Warning
	SysInfo  *SystemInfo

	// Derived
	Uptime    time.Duration
	HasWarning bool
}

// TotalHashrate returns the current 5-second hashrate in TH/s.
func (m *MinerDevice) TotalHashrate() float64 {
	if m.Summary == nil {
		return 0
	}
	v := m.Summary.Rate5s
	if m.Summary.RateUnit == "GH/s" || m.Summary.RateUnit == "GH" {
		v /= 1000
	}
	return v
}

// AverageHashrate returns the average hashrate in TH/s.
func (m *MinerDevice) AverageHashrate() float64 {
	if m.Summary == nil {
		return 0
	}
	v := m.Summary.RateAvg
	if m.Summary.RateUnit == "GH/s" || m.Summary.RateUnit == "GH" {
		v /= 1000
	}
	return v
}

// MaxTemp returns the highest temperature reported by any chain.
func (m *MinerDevice) MaxTemp() float64 {
	if m.Summary == nil {
		return 0
	}
	return m.Summary.Temperature
}

// StatusLabel returns "Online", "Warning" or "Offline".
func (m *MinerDevice) StatusLabel() string {
	if !m.Online {
		return "Offline"
	}
	if m.HasWarning {
		return "Warning"
	}
	return "Online"
}
