package goasic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"minergate/internal/config"
	"minergate/internal/models"

	"github.com/goasic/goasic"
)

// fakeMiner is a minimal goasic.Miner implementation for testing.
type fakeMiner struct {
	ip    string
	data  *goasic.MinerData
	err   error
	brand string
}

func (f *fakeMiner) GetData(ctx context.Context) (*goasic.MinerData, error) {
	return f.data, f.err
}
func (f *fakeMiner) GetConfig(ctx context.Context) (*goasic.MinerConfig, error)   { return nil, nil }
func (f *fakeMiner) SendConfig(ctx context.Context, cfg goasic.MinerConfig) error { return nil }
func (f *fakeMiner) Reboot(ctx context.Context) error                             { return nil }
func (f *fakeMiner) FaultLightOn(ctx context.Context) error                       { return nil }
func (f *fakeMiner) FaultLightOff(ctx context.Context) error                      { return nil }
func (f *fakeMiner) StopMining(ctx context.Context) error                         { return nil }
func (f *fakeMiner) ResumeMining(ctx context.Context) error                       { return nil }
func (f *fakeMiner) IsMining(ctx context.Context) (bool, error)                   { return f.data.IsMining, nil }
func (f *fakeMiner) IP() string                                                   { return f.ip }
func (f *fakeMiner) Brand() string                                                { return f.brand }

func TestConvertMinerDataToDeviceInfo(t *testing.T) {
	hash := 42.5
	temp := 75.3
	power := 3200
	eff := 9.1
	uptime := uint64(123)
	cnt := 2

	md := &goasic.MinerData{
		IP:               "10.0.0.1",
		Model:            "S19",
		Make:             "Antminer",
		Firmware:         "1.0",
		Algorithm:        "SHA-256",
		Hashrate:         &hash,
		ExpectedHashrate: &hash,
		Temperature:      []float64{temp},
		FanSpeeds:        []int{1200, 1200},
		Wattage:          &power,
		WattageLimit:     &power,
		Efficiency:       &eff,
		Pool1URL:         "stratum+tcp://pool",
		Pool1User:        "user",
		Hostname:         "miner1",
		Errors:           []string{"err"},
		FaultLight:       true,
		IsMining:         true,
		Uptime:           &uptime,
		ChipCount:        &cnt,
		FansCount:        &cnt,
		Cooling:          "Air",
	}

	info := convertMinerDataToDeviceInfo(md)
	if info.IP != md.IP {
		t.Fatalf("expected IP %s, got %s", md.IP, info.IP)
	}
	if info.Hashrate == nil || *info.Hashrate != hash {
		t.Fatalf("expected hashrate %f, got %v", hash, info.Hashrate)
	}
	if len(info.Temperature) != 1 || info.Temperature[0] != temp {
		t.Fatalf("expected temperature %v, got %v", temp, info.Temperature)
	}
	if info.Wattage == nil || *info.Wattage != power {
		t.Fatalf("expected power %d, got %v", power, info.Wattage)
	}
}

func TestAddOrUpdateDevice(t *testing.T) {
	m := NewManager(&config.Config{})

	first := &DiscoveredDevice{IP: "10.0.0.1", Model: "S19", Make: "Antminer", Status: StatusOnline, LastSeen: time.Now()}
	m.addOrUpdateDevice(first)
	if got := m.GetDevice("10.0.0.1"); got == nil {
		t.Fatal("expected device to be stored")
	}

	second := &DiscoveredDevice{IP: "10.0.0.1", Model: "S19 Pro", Make: "Antminer", Status: StatusOnline, LastSeen: time.Now()}
	m.addOrUpdateDevice(second)
	if got := m.GetDevice("10.0.0.1"); got.Model != "S19 Pro" {
		t.Fatalf("expected model updated to S19 Pro, got %s", got.Model)
	}
}

func TestScanNetwork_Success(t *testing.T) {
	oldScan := scanSubnetFn
	defer func() { scanSubnetFn = oldScan }()

	called := int32(0)
	scanSubnetFn = func(ctx context.Context, cidr string, max int) ([]goasic.Miner, error) {
		atomic.AddInt32(&called, 1)
		return []goasic.Miner{&fakeMiner{ip: "10.0.0.1", data: &goasic.MinerData{IP: "10.0.0.1", Model: "S19", Make: "Antminer", IsMining: true}}}, nil
	}

	m := NewManager(&config.Config{GoASICEnabled: true, GoASICNetworkRange: "10.0.0.0/24"})
	if err := m.ScanNetwork(context.Background()); err != nil {
		t.Fatalf("ScanNetwork failed: %v", err)
	}
	if atomic.LoadInt32(&called) == 0 {
		t.Fatal("expected scan subnet to be called")
	}
	if got := m.GetDevice("10.0.0.1"); got == nil {
		t.Fatal("expected device found")
	}
}

func TestScanNetwork_Error(t *testing.T) {
	oldScan := scanSubnetFn
	defer func() { scanSubnetFn = oldScan }()

	scanSubnetFn = func(ctx context.Context, cidr string, max int) ([]goasic.Miner, error) {
		return nil, errors.New("boom")
	}

	var gotErr error
	m := NewManager(&config.Config{GoASICEnabled: true, GoASICNetworkRange: "10.0.0.0/24"})
	m.OnError(func(err error) { gotErr = err })

	if err := m.ScanNetwork(context.Background()); err == nil {
		t.Fatal("expected ScanNetwork to return error")
	}
	if gotErr == nil {
		t.Fatal("expected onError to be called")
	}
}

func TestCollectDeviceData_UpdatesData(t *testing.T) {
	oldDetect := detectMinerFn
	defer func() { detectMinerFn = oldDetect }()

	m := NewManager(&config.Config{})
	m.devices["10.0.0.1"] = &DiscoveredDevice{IP: "10.0.0.1", Status: StatusOffline}

	detectMinerFn = func(ctx context.Context, ip string) (goasic.Miner, error) {
		return &fakeMiner{ip: ip, data: &goasic.MinerData{IP: ip, Model: "S19", Make: "Antminer", IsMining: true}}, nil
	}

	m.collectDeviceData(context.Background(), m.devices["10.0.0.1"])
	dev := m.GetDevice("10.0.0.1")
	if dev == nil || dev.Data == nil {
		t.Fatal("expected device data to be updated")
	}
	if dev.Status != StatusOnline {
		t.Fatalf("expected status online, got %s", dev.Status)
	}
}

func TestCollectDeviceData_DetectError(t *testing.T) {
	oldDetect := detectMinerFn
	defer func() { detectMinerFn = oldDetect }()

	m := NewManager(&config.Config{})
	m.devices["10.0.0.1"] = &DiscoveredDevice{IP: "10.0.0.1", Status: StatusOnline}

	detectMinerFn = func(ctx context.Context, ip string) (goasic.Miner, error) {
		return nil, errors.New("detect fail")
	}

	var gotErr error
	m.OnError(func(err error) { gotErr = err })
	m.collectDeviceData(context.Background(), m.devices["10.0.0.1"])
	if gotErr == nil {
		t.Fatal("expected onError to be called")
	}
	if got := m.GetDevice("10.0.0.1"); got == nil || got.Status != StatusError {
		t.Fatalf("expected status error, got %v", got)
	}
}

func TestGettersCount(t *testing.T) {
	m := NewManager(&config.Config{})
	m.devices["a"] = &DiscoveredDevice{Status: StatusOnline}
	m.devices["b"] = &DiscoveredDevice{Status: StatusOffline}
	m.devices["c"] = &DiscoveredDevice{Status: StatusOnline}

	if m.GetDeviceCount() != 3 {
		t.Fatalf("expected 3 devices, got %d", m.GetDeviceCount())
	}
	if m.GetOnlineCount() != 2 {
		t.Fatalf("expected 2 online, got %d", m.GetOnlineCount())
	}
}

func TestGenerateDeviceReports(t *testing.T) {
	m := NewManager(&config.Config{})
	hash := 123.0
	temp := 55.0
	power := 2200
	eff := 10.0
	uptime := uint64(12345)

	m.devices["10.0.0.1"] = &DiscoveredDevice{
		IP:     "10.0.0.1",
		Status: StatusOnline,
		Data: &models.DeviceInfo{
			Hashrate:    &hash,
			Temperature: []float64{temp},
			Wattage:     &power,
			Efficiency:  &eff,
			Uptime:      &uptime,
			IsMining:    true,
			FanSpeeds:   []int{1000, 1100},
		},
	}

	reports := m.GenerateDeviceReports()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].Hashrate != hash {
		t.Fatalf("expected hashrate %f, got %f", hash, reports[0].Hashrate)
	}
}

func TestIsScanningAndLastScan(t *testing.T) {
	m := NewManager(&config.Config{})
	if m.IsScanning() {
		t.Fatal("expected not scanning")
	}
	m.scanning = true
	if !m.IsScanning() {
		t.Fatal("expected scanning")
	}

	now := time.Now()
	m.lastScan = now
	if !m.GetLastScanTime().Equal(now) {
		t.Fatalf("expected last scan time %v, got %v", now, m.GetLastScanTime())
	}
}
