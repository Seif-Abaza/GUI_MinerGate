// Package api provides an HTTP client for the cgminer / bmminer web interface.
//
// It talks to the same CGI endpoints the original Bitmain dashboard.html uses:
//
//	/cgi-bin/summary.cgi        – miner overview
//	/cgi-bin/stats.cgi          – chain / temperature / fan details
//	/cgi-bin/pools.cgi          – pool list  (maps to the "pools" API cmd)
//	/cgi-bin/warning.cgi        – warning list
//	/cgi-bin/chart.cgi          – hashrate chart series
//	/cgi-bin/get_system_info.cgi– firmware / network info
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"minergate/internal/models"
)

const (
	defaultHTTPTimeout = 10 * time.Second
)

// Client fetches data from one miner's HTTP interface.
type Client struct {
	base   string // e.g. "http://192.168.1.100:8081"
	http   *http.Client
}

// New creates a new Client for the miner at host:port (HTTP port, usually 80
// or 8081 for the simulator).
func New(host string, port int) *Client {
	return &Client{
		base: fmt.Sprintf("http://%s:%d", host, port),
		http: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Low-level GET
// ─────────────────────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, path string, dst interface{}) error {
	url := c.base + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request %s: %w", url, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return fmt.Errorf("read body %s: %w", url, err)
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode JSON from %s: %w\nbody: %.200s", url, err, body)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// CGI endpoint accessors
// ─────────────────────────────────────────────────────────────────────────────

// GetSummary fetches /cgi-bin/summary.cgi and returns the parsed Summary.
func (c *Client) GetSummary(ctx context.Context) (*models.Summary, error) {
	var resp models.SummaryResponse
	if err := c.get(ctx, "/cgi-bin/summary.cgi", &resp); err != nil {
		return nil, err
	}
	if len(resp.Summary) == 0 {
		return nil, fmt.Errorf("summary: empty SUMMARY array")
	}

	raw := resp.Summary[0]
	s := &models.Summary{
		Elapsed:     raw.Elapsed,
		Rate5s:      raw.Rate5s,
		RateAvg:     raw.RateAvg,
		RateUnit:    raw.RateUnit,
		Accepted:    raw.Accepted,
		Rejected:    raw.Rejected,
		HWErrors:    raw.HWErrors,
		Temperature: raw.Temperature,
		Fan1:        raw.Fan1,
		Fan2:        raw.Fan2,
		Fan3:        raw.Fan3,
		Fan4:        raw.Fan4,
		FanNum:      raw.FanNum,
		Power:       raw.Power,
		MinerType:   raw.MinerType,
		StatusCards: raw.StatusCards,
	}

	total := float64(raw.Accepted + raw.Rejected)
	if total > 0 {
		s.RejectRatio = strconv.FormatFloat(float64(raw.Rejected)/total*100, 'f', 2, 64) + "%"
	} else {
		s.RejectRatio = "0.00%"
	}

	return s, nil
}

// GetStats fetches /cgi-bin/stats.cgi and returns the raw StatsResponse.
func (c *Client) GetStats(ctx context.Context) (*models.StatsResponse, error) {
	var resp models.StatsResponse
	if err := c.get(ctx, "/cgi-bin/stats.cgi", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPools fetches /cgi-bin/pools.cgi and returns the pool list.
func (c *Client) GetPools(ctx context.Context) ([]models.Pool, error) {
	var resp models.PoolsResponse
	if err := c.get(ctx, "/cgi-bin/pools.cgi", &resp); err != nil {
		return nil, err
	}
	return resp.Pools, nil
}

// GetWarnings fetches /cgi-bin/warning.cgi and returns the warning list.
func (c *Client) GetWarnings(ctx context.Context) ([]models.Warning, error) {
	var resp models.WarningResponse
	if err := c.get(ctx, "/cgi-bin/warning.cgi", &resp); err != nil {
		return nil, err
	}
	return resp.Warnings, nil
}

// GetRateChart fetches /cgi-bin/chart.cgi and returns the hashrate chart data.
func (c *Client) GetRateChart(ctx context.Context) (*models.RateResponse, error) {
	var resp models.RateResponse
	if err := c.get(ctx, "/cgi-bin/chart.cgi", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSystemInfo fetches /cgi-bin/get_system_info.cgi.
func (c *Client) GetSystemInfo(ctx context.Context) (*models.SystemInfo, error) {
	var info models.SystemInfo
	if err := c.get(ctx, "/cgi-bin/get_system_info.cgi", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Composite poll – fetches all endpoints in one call
// ─────────────────────────────────────────────────────────────────────────────

// PollResult bundles everything gathered in one poll cycle.
type PollResult struct {
	Summary  *models.Summary
	Stats    *models.StatsResponse
	Pools    []models.Pool
	Warnings []models.Warning
	SysInfo  *models.SystemInfo
	// Errors for each sub-request (nil = success)
	SummaryErr  error
	StatsErr    error
	PoolsErr    error
	WarningsErr error
	SysInfoErr  error
}

// Poll runs all CGI requests concurrently with the supplied context.
func (c *Client) Poll(ctx context.Context) *PollResult {
	type res[T any] struct {
		v   T
		err error
	}

	chSum := make(chan res[*models.Summary], 1)
	chSta := make(chan res[*models.StatsResponse], 1)
	chPol := make(chan res[[]models.Pool], 1)
	chWar := make(chan res[[]models.Warning], 1)
	chSys := make(chan res[*models.SystemInfo], 1)

	go func() { v, e := c.GetSummary(ctx); chSum <- res[*models.Summary]{v, e} }()
	go func() { v, e := c.GetStats(ctx); chSta <- res[*models.StatsResponse]{v, e} }()
	go func() { v, e := c.GetPools(ctx); chPol <- res[[]models.Pool]{v, e} }()
	go func() { v, e := c.GetWarnings(ctx); chWar <- res[[]models.Warning]{v, e} }()
	go func() { v, e := c.GetSystemInfo(ctx); chSys <- res[*models.SystemInfo]{v, e} }()

	pr := &PollResult{}
	rSum := <-chSum
	pr.Summary, pr.SummaryErr = rSum.v, rSum.err
	rSta := <-chSta
	pr.Stats, pr.StatsErr = rSta.v, rSta.err
	rPol := <-chPol
	pr.Pools, pr.PoolsErr = rPol.v, rPol.err
	rWar := <-chWar
	pr.Warnings, pr.WarningsErr = rWar.v, rWar.err
	rSys := <-chSys
	pr.SysInfo, pr.SysInfoErr = rSys.v, rSys.err

	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// Connectivity check
// ─────────────────────────────────────────────────────────────────────────────

// Ping returns nil if the miner's HTTP server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/cgi-bin/get_system_info.cgi", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
