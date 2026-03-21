// =============================================================================
// Package frp - عميل FRP المدمج
// =============================================================================
// هذا الملف يوفر تكامل FRP Client مع التطبيق:
// - اتصال تلقائي عند بدء التشغيل
// - إعادة الاتصال التلقائية
// - مراقبة حالة الاتصال
// - تكوين من ملف config/config.go
// =============================================================================
package frp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"minergate/internal/config"
	"minergate/internal/models"
)

// =============================================================================
// هيئات البيانات
// =============================================================================

// ClientState حالة عميل FRP
type ClientState string

const (
	// StateDisconnected غير متصل
	StateDisconnected ClientState = "disconnected"
	// StateConnecting جاري الاتصال
	StateConnecting ClientState = "connecting"
	// StateConnected متصل
	StateConnected ClientState = "connected"
	// StateError خطأ
	StateError ClientState = "error"
)

// Client عميل FRP المدمج
type Client struct {
	// التكوين
	cfgMgr *config.ConfigManager
	cfg    *config.Config
	frpCfg *config.FRPConfig

	// الحالة
	state     ClientState
	lastError error
	startTime time.Time

	// الإحصائيات
	bytesSent     int64
	bytesReceived int64

	// القنوات
	stopChan chan struct{}
	doneChan chan struct{}

	// القفل
	mu sync.RWMutex

	// callbacks
	onStateChange func(state ClientState)
	onError       func(error)
}

// NewClient ينشئ عميل FRP جديد
func NewClient(cfgMgr *config.ConfigManager) *Client {
	return &Client{
		cfgMgr:    cfgMgr,
		cfg:       cfgMgr.Get(),
		state:     StateDisconnected,
		stopChan:  make(chan struct{}),
		doneChan:  make(chan struct{}),
	}
}

// =============================================================================
// العمليات الأساسية
// =============================================================================

// Start يبدأ عميل FRP
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// التحقق من تفعيل FRP
	if !c.cfg.FRPEnabled {
		return nil // FRP غير مفعل
	}

	// الحصول على التكوين
	c.frpCfg = c.cfgMgr.GetFRPConfig()
	if c.frpCfg == nil {
		return fmt.Errorf("FRP configuration not found")
	}

	// التحقق من التكوين
	if err := c.validateConfig(); err != nil {
		return err
	}

	// بدء الاتصال في goroutine منفصلة
	go c.run(ctx)

	return nil
}

// validateConfig يتحقق من صحة التكوين
func (c *Client) validateConfig() error {
	if c.frpCfg.ServerAddr == "" {
		return fmt.Errorf("FRP server address is required")
	}
	if c.frpCfg.ServerPort <= 0 || c.frpCfg.ServerPort > 65535 {
		return fmt.Errorf("invalid FRP server port: %d", c.frpCfg.ServerPort)
	}
	if c.frpCfg.Token == "" {
		return fmt.Errorf("FRP token is required")
	}
	return nil
}

// run يشغل حلقة الاتصال الرئيسية
func (c *Client) run(ctx context.Context) {
	defer close(c.doneChan)

	c.setState(StateConnecting)

	// حلقة إعادة الاتصال
	retryCount := 0
	maxRetries := 10
	retryDelay := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			c.setState(StateDisconnected)
			return
		case <-c.stopChan:
			c.setState(StateDisconnected)
			return
		default:
		}

		// محاولة الاتصال
		err := c.connect(ctx)
		if err != nil {
			c.mu.Lock()
			c.lastError = err
			c.mu.Unlock()

			if c.onError != nil {
				c.onError(err)
			}

			retryCount++
			if retryCount >= maxRetries {
				c.setState(StateError)
				// انتظار أطول قبل إعادة المحاولة
				select {
				case <-time.After(60 * time.Second):
					retryCount = 0
					continue
				case <-ctx.Done():
					return
				case <-c.stopChan:
					return
				}
			}

			c.setState(StateConnecting)

			// انتظار قبل إعادة المحاولة
			select {
			case <-time.After(retryDelay):
				// زيادة التأخير تدريجياً
				retryDelay = time.Duration(float64(retryDelay) * 1.5)
				if retryDelay > 60*time.Second {
					retryDelay = 60 * time.Second
				}
			case <-ctx.Done():
				return
			case <-c.stopChan:
				return
			}

			continue
		}

		// نجح الاتصال
		retryCount = 0
		retryDelay = 5 * time.Second
		c.setState(StateConnected)
		c.mu.Lock()
		c.startTime = time.Now()
		c.mu.Unlock()
	}
}

// connect يقوم باتصال فعلي بالخادم
func (c *Client) connect(ctx context.Context) error {
	// إنشاء اتصال TCP
	serverAddr := fmt.Sprintf("%s:%d", c.frpCfg.ServerAddr, c.frpCfg.ServerPort)

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to FRP server: %w", err)
	}
	defer conn.Close()

	// إرسال رسالة المصادقة
	// ملاحظة: هذا تبسيط، في الإنتاج يجب استخدام بروتوكول FRP الكامل
	authMsg := fmt.Sprintf("AUTH:%s:%s\n", c.frpCfg.ProxyName, c.frpCfg.Token)
	_, err = conn.Write([]byte(authMsg))
	if err != nil {
		return fmt.Errorf("failed to send auth message: %w", err)
	}

	// قراءة الاستجابة
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	response := string(buf[:n])
	if response != "OK\n" && response != "OK" {
		return fmt.Errorf("authentication failed: %s", response)
	}

	// الاتصال ناجح - نعتبر العميل متصلاً
	c.setState(StateConnected)
	c.mu.Lock()
	c.startTime = time.Now()
	c.mu.Unlock()

	// في الإنتاج، هنا يجب إعداد الأنفاق والوكلاء

	// انتظار إغلاق الاتصال
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stopChan:
		return fmt.Errorf("client stopped")
	}
}

// Stop يوقف عميل FRP
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.stopChan:
		//Already closed
	default:
		close(c.stopChan)
	}
}

// =============================================================================
// الحالة والإحصائيات
// =============================================================================

// GetState يعيد الحالة الحالية
func (c *Client) GetState() ClientState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// GetStatus يعيد حالة الاتصال الكاملة
func (c *Client) GetStatus() *models.FRPStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var lastConnected time.Time
	if c.state == StateConnected {
		lastConnected = c.startTime
	}

	return &models.FRPStatus{
		Connected:     c.state == StateConnected,
		ServerAddr:    c.cfg.FRPServerAddr,
		ServerPort:    c.cfg.FRPServerPort,
		ProxyName:     c.cfg.FRPProxyName,
		LocalPort:     c.cfg.FRPLocalPort,
		LastConnected: lastConnected,
		BytesSent:     c.bytesSent,
		BytesReceived: c.bytesReceived,
	}
}

// GetLastError يعيد آخر خطأ
func (c *Client) GetLastError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

// GetUptime يعيد وقت التشغيل
func (c *Client) GetUptime() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.state != StateConnected {
		return 0
	}
	return time.Since(c.startTime)
}

// IsConnected يتحقق من الاتصال
func (c *Client) IsConnected() bool {
	return c.GetState() == StateConnected
}

// =============================================================================
// Callbacks
// =============================================================================

// OnStateChange يضبط callback تغيير الحالة
func (c *Client) OnStateChange(callback func(state ClientState)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStateChange = callback
}

// OnError يضبط callback الأخطاء
func (c *Client) OnError(callback func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onError = callback
}

// setState يضبط الحالة
func (c *Client) setState(state ClientState) {
	c.mu.Lock()
	c.state = state
	callback := c.onStateChange
	c.mu.Unlock()

	if callback != nil {
		callback(state)
	}
}

// =============================================================================
// دوال مساعدة
// =============================================================================

// GenerateFRPConfigString يولد سلسلة تكوين FRP
// هذا يمكن استخدامه لإنشاء ملف frpc.toml
func GenerateFRPConfigString(cfg *config.Config) string {
	if !cfg.FRPEnabled {
		return ""
	}

	return fmt.Sprintf(`# FRP Client Configuration
# Generated by MinerGate Dashboard v%s

serverAddr = "%s"
serverPort = %d

auth.method = "token"
auth.token = "%s"

[[proxies]]
name = "%s"
type = "tcp"
localIP = "127.0.0.1"
localPort = %d
`, config.Version, cfg.FRPServerAddr, cfg.FRPServerPort, cfg.FRPToken, cfg.FRPProxyName, cfg.FRPLocalPort)
}

// WaitForConnection ينتظر الاتصال
func (c *Client) WaitForConnection(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if c.IsConnected() {
				return nil
			}
		}
	}
}
