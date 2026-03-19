// =============================================================================
// Package api - عميل HTTP آمن للاتصال بالخادم
// =============================================================================
// هذا الملف يوفر عميل HTTP آمن مع:
// - تشفير البيانات الحساسة
// - التحقق من شهادات SSL
// - إعادة المحاولة التلقائية
// - حماية من هجماتrate limiting
// - تسجيل آمن للطلبات
// =============================================================================
package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"minergate/internal/config"
	"minergate/internal/models"
)

// =============================================================================
// ثوابت الأمان
// =============================================================================

const (
	// MaxRetries الحد الأقصى لمحاولات إعادة الاتصال
	MaxRetries = 3
	// RetryDelay تأخير إعادة المحاولة
	RetryDelay = 2 * time.Second
	// RequestTimeout مهلة الطلب الافتراضية
	RequestTimeout = 30 * time.Second
	// MaxBodySize الحد الأقصى لحجم الاستجابة (10MB)
	MaxBodySize = 10 * 1024 * 1024
	// RateLimitRequests عدد الطلبات المسموح بها في الدقيقة
	RateLimitRequests = 60
	// RateLimitWindow نافزة تحديد المعدل
	RateLimitWindow = time.Minute
)

// =============================================================================
// هيئات البيانات
// =============================================================================

// Client عميل API آمن
type Client struct {
	baseURL       string
	apiKey        string
	httpClient    *http.Client
	cfg           *config.Config
	rateLimiter   *RateLimiter
	retryCount    int
	retryDelay    time.Duration
	mu            sync.RWMutex
	lastRequest   time.Time
	requestsCount int
}

// RateLimiter لتقييد معدل الطلبات
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter ينشئ محدد معدل جديد
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow يتحقق مما إذا كان الطلب مسموحاً به
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	requests := rl.requests[key]

	// إزالة الطلبات القديمة
	validRequests := make([]time.Time, 0)
	for _, t := range requests {
		if now.Sub(t) < rl.window {
			validRequests = append(validRequests, t)
		}
	}

	if len(validRequests) >= rl.limit {
		rl.requests[key] = validRequests
		return false
	}

	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests
	return true
}

// SecureHTTPClient ينشئ عميل HTTP آمن
type SecureHTTPClient struct {
	client *http.Client
}

// NewSecureHTTPClient ينشئ عميل HTTP آمن مع إعدادات أمنية
func NewSecureHTTPClient(timeout time.Duration, insecureSkipVerify bool) *SecureHTTPClient {
	// إنشاء تجمع شهادات
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// إعدادات TLS
	tlsConfig := &tls.Config{
		RootCAs:            rootCAs,
		MinVersion:         tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		InsecureSkipVerify: insecureSkipVerify,
	}

	transport := &http.Transport{
		TLSClientConfig:       tlsConfig,
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		DisableCompression:    false,
		ResponseHeaderTimeout: timeout,
	}

	return &SecureHTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// =============================================================================
// إنشاء العميل
// =============================================================================

// NewClient ينشئ عميل API جديد مع إعدادات آمنة
func NewClient(cfgMgr *config.ConfigManager) *Client {
	cfg := cfgMgr.Get()
	// الحصول على مفتاح API
	apiKey, _ := cfgMgr.GetAPIKey()

	// إنشاء عميل HTTP آمن
	secureClient := NewSecureHTTPClient(
		time.Duration(cfg.APITimeout)*time.Second,
		false, // لا تتخطى التحقق من SSL
	)

	return &Client{
		baseURL:     cfg.APIEndpoint,
		apiKey:      apiKey,
		httpClient:  secureClient.client,
		cfg:         cfg,
		rateLimiter: NewRateLimiter(RateLimitRequests, RateLimitWindow),
		retryCount:  cfg.APIRetryCount,
		retryDelay:  time.Duration(cfg.APIRetryDelay) * time.Second,
	}
}

// =============================================================================
// العمليات الأساسية
// =============================================================================

// doRequest ينفذ طلب HTTP مع إعادة المحاولة والأمان
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}, headers map[string]string) (*http.Response, error) {
	// التحقق من معدل الطلبات
	if !c.rateLimiter.Allow(c.baseURL) {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// تحويل الجسم إلى JSON إذا وجد
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	// بناء URL الكامل
	fullURL, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// إنشاء الطلب
	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// إضافة الرؤوس الآمنة
	c.setSecureHeaders(req, headers)

	// تنفيذ الطلب مع إعادة المحاولة
	var resp *http.Response
	var lastErr error

	for i := 0; i <= c.retryCount; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay):
			}
		}

		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil && resp.StatusCode < 500 {
			break
		}

		// إغلاق الاستجابة السابقة
		if resp != nil {
			resp.Body.Close()
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.retryCount, lastErr)
	}

	return resp, nil
}

// setSecureHeaders يضبط الرؤوس الآمنة للطلب
func (c *Client) setSecureHeaders(req *http.Request, customHeaders map[string]string) {
	// الرؤوس الأساسية
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MinerGate/"+config.Version)
	req.Header.Set("X-Client-Version", config.Version)

	// مفتاح API (إذا وجد)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// الرؤوس المخصصة
	for key, value := range customHeaders {
		// تجنب الرؤوس الخطرة
		if !isDangerousHeader(key) {
			req.Header.Set(key, value)
		}
	}

	// منع التخزين المؤقت للبيانات الحساسة
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set("Pragma", "no-cache")
}

// isDangerousHeader يتحقق مما إذا كان الرأس خطراً
func isDangerousHeader(key string) bool {
	dangerous := []string{
		"Host", "Content-Length", "Transfer-Encoding",
		"Connection", "Upgrade", "TE", "Trailer",
	}
	keyLower := strings.ToLower(key)
	for _, d := range dangerous {
		if strings.ToLower(d) == keyLower {
			return true
		}
	}
	return false
}

// readResponse reads the response body with size limit
func (c *Client) readResponse(resp *http.Response) ([]byte, error) {
	// محدود حجم القراءة
	limitedReader := io.LimitReader(resp.Body, MaxBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return body, nil
}

// =============================================================================
// عمليات المزرعة والأجهزة
// =============================================================================

// GetFarm يجلب بيانات المزرعة
func (c *Client) GetFarm(ctx context.Context, farmID string) (*models.Farm, error) {
	endpoint := fmt.Sprintf("/farms/%s", url.PathEscape(farmID))
	resp, err := c.doRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	// تحويل البيانات
	farmData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var farm models.Farm
	if err := json.Unmarshal(farmData, &farm); err != nil {
		return nil, fmt.Errorf("failed to parse farm data: %w", err)
	}

	return &farm, nil
}

// GetMiners يجلب قائمة الأجهزة
func (c *Client) GetMiners(ctx context.Context, farmID string) ([]models.Miner, error) {
	endpoint := fmt.Sprintf("/farms/%s/miners", url.PathEscape(farmID))
	resp, err := c.doRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	minersData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var miners []models.Miner
	if err := json.Unmarshal(minersData, &miners); err != nil {
		return nil, fmt.Errorf("failed to parse miners data: %w", err)
	}

	return miners, nil
}

// GetMinerStats يجلب إحصائيات جهاز محدد
func (c *Client) GetMinerStats(ctx context.Context, minerID string) (*models.MinerStats, error) {
	endpoint := fmt.Sprintf("/miners/%s/stats", url.PathEscape(minerID))
	resp, err := c.doRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	statsData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var stats models.MinerStats
	if err := json.Unmarshal(statsData, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats data: %w", err)
	}

	return &stats, nil
}

// ExecuteAction ينفذ إجراء على جهاز
func (c *Client) ExecuteAction(ctx context.Context, req models.ActionRequest) (*models.ActionResponse, error) {
	// التحقق من صحة الإجراء
	if !isValidAction(string(req.Action)) {
		return nil, fmt.Errorf("invalid action: %s", req.Action)
	}

	// التحقق من صحة معرف الجهاز
	if req.MinerID == "" {
		return nil, fmt.Errorf("miner ID is required")
	}

	endpoint := fmt.Sprintf("/miners/%s/action", url.PathEscape(req.MinerID))
	resp, err := c.doRequest(ctx, "POST", endpoint, req, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	actionData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var actionResp models.ActionResponse
	if err := json.Unmarshal(actionData, &actionResp); err != nil {
		return nil, fmt.Errorf("failed to parse action response: %w", err)
	}

	return &actionResp, nil
}

// isValidAction يتحقق من صحة الإجراء
func isValidAction(action string) bool {
	validActions := map[string]bool{
		string(models.ActionRestart):     true,
		string(models.ActionReboot):      true,
		string(models.ActionStopMining):  true,
		string(models.ActionStartMining): true,
		string(models.ActionGetErrors):   true,
		string(models.ActionGetPools):    true,
		string(models.ActionFanStatus):   true,
		string(models.ActionPowerInfo):   true,
	}
	return validActions[action]
}

// =============================================================================
// إرسال بيانات الأجهزة
// =============================================================================

// ReportDeviceData يرسل بيانات الجهاز إلى API_TARGET_DEVICE
func (c *Client) ReportDeviceData(ctx context.Context, reports []models.DeviceReport) error {
	if c.cfg.APITargetDevice == "" {
		return fmt.Errorf("API target device URL not configured")
	}

	// إنشاء طلب مستقل لعنوان APITargetDevice
	reqBody, err := json.Marshal(map[string]interface{}{
		"reports":   reports,
		"timestamp": time.Now(),
		"version":   config.Version,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal device reports: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.APITargetDevice, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setSecureHeaders(req, map[string]string{
		"X-Report-Type": "device-stats",
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send device reports: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// =============================================================================
// البيانات التاريخية
// =============================================================================

// GetHistoricalData يجلب البيانات التاريخية للرسم البياني
func (c *Client) GetHistoricalData(ctx context.Context, minerID, period string) (*models.ChartData, error) {
	endpoint := fmt.Sprintf("/miners/%s/history?period=%s", url.PathEscape(minerID), url.QueryEscape(period))
	resp, err := c.doRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	chartData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var chart models.ChartData
	if err := json.Unmarshal(chartData, &chart); err != nil {
		return nil, fmt.Errorf("failed to parse chart data: %w", err)
	}

	return &chart, nil
}

// GetHashrateChart يجلب بيانات رسم hashrate
func (c *Client) GetHashrateChart(ctx context.Context, minerID string) (*models.HashrateChart, error) {
	endpoint := fmt.Sprintf("/miners/%s/charts/hashrate", url.PathEscape(minerID))
	resp, err := c.doRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	chartData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var chart models.HashrateChart
	if err := json.Unmarshal(chartData, &chart); err != nil {
		return nil, fmt.Errorf("failed to parse hashrate chart: %w", err)
	}

	return &chart, nil
}

// GetTempPowerChart يجلب بيانات رسم درجة الحرارة والطاقة
func (c *Client) GetTempPowerChart(ctx context.Context, minerID string) (*models.TempPowerChart, error) {
	endpoint := fmt.Sprintf("/miners/%s/charts/temp-power", url.PathEscape(minerID))
	resp, err := c.doRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	chartData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var chart models.TempPowerChart
	if err := json.Unmarshal(chartData, &chart); err != nil {
		return nil, fmt.Errorf("failed to parse temp-power chart: %w", err)
	}

	return &chart, nil
}

// =============================================================================
// التحديثات
// =============================================================================

// CheckUpdate يتحقق من وجود تحديثات
func (c *Client) CheckUpdate(ctx context.Context) (*models.UpdateInfo, error) {
	endpoint := "/update/check"
	resp, err := c.doRequest(ctx, "GET", endpoint, nil, map[string]string{
		"X-Current-Version": config.Version,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil // لا يوجد تحديث
	}

	body, err := c.readResponse(resp)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	updateData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}

	var info models.UpdateInfo
	if err := json.Unmarshal(updateData, &info); err != nil {
		return nil, fmt.Errorf("failed to parse update info: %w", err)
	}

	return &info, nil
}

// DownloadUpdate يحمل ملف التحديث
func (c *Client) DownloadUpdate(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500*1024*1024)) // حد أقصى 500MB
	if err != nil {
		return nil, "", fmt.Errorf("failed to read update file: %w", err)
	}

	// الحصول على المجموع الاختباري من الرؤوس
	checksum := resp.Header.Get("X-SHA256")

	return body, checksum, nil
}

// =============================================================================
// دوال مساعدة
// =============================================================================

// SetAPIKey يضبط مفتاح API
func (c *Client) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = key
}

// SetBaseURL يضبط عنوان URL الأساسي
func (c *Client) SetBaseURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = url
}

// SetTimeout يضبط مهلة الاتصال
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// Close يغلق اتصالات العميل
func (c *Client) Close() {
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}
