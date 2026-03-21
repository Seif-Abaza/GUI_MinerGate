// =============================================================================
// Package config - إدارة إعدادات التطبيق
// =============================================================================
// هذا الملف يدير جميع إعدادات التطبيق بما في ذلك:
// - دعم اللغة الإنجليزية والعربية
// - إعدادات API والتحديث
// - إعدادات FRP Client
// - إعدادات GoASIC
// =============================================================================
package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// =============================================================================
// الثوابت والإصدار
// =============================================================================

// Version إصدار التطبيق الحالي
// v1.0.4: إضافة رسم بياني حقيقي بـ fynesimplechart مُضمَّن داخل ChartWidget
const Version = "1.0.4"

// AppName اسم التطبيق
const AppName = "MinerGate Dashboard"

// Language اللغة المدعومة
type Language string

const (
	// LangEnglish اللغة الإنجليزية
	LangEnglish Language = "en"
	// LangArabic اللغة العربية
	LangArabic Language = "ar"
)

// =============================================================================
// هيئات البيانات الرئيسية
// =============================================================================

// Config الإعدادات الرئيسية للتطبيق
// يتم تحميلها من ملف config.json وتحتوي على جميع الإعدادات المطلوبة
type Config struct {
	// إعدادات عامة
	ApplicationName string   `json:"application_name"` // اسم التطبيق
	FarmUUID       string   `json:"farm_uuid"`       // معرّف المزرعة
	Language       Language `json:"language"`      // لغة الواجهة (en/ar)
	AutoRefresh    bool     `json:"auto_refresh"`  // التحديث التلقائي
	RefreshRate    int      `json:"refresh_rate"`  // معدل التحديث بالثواني
	Theme          string   `json:"theme"`         // السمة (dark/light)
	WindowWidth    int      `json:"window_width"`  // عرض النافذة
	WindowHeight int      `json:"window_height"` // ارتفاع النافذة

	// إعدادات API
	APIEndpoint     string `json:"api_endpoint"`      // نقطة نهاية API الرئيسية
	APITargetDevice string `json:"api_target_device"` // نقطة إرسال بيانات الأجهزة
	APIKey          string `json:"api_key"`           // مفتاح API المشفر
	APITimeout      int    `json:"api_timeout"`       // مهلة الاتصال بالثواني
	APIRetryCount   int    `json:"api_retry_count"`   // عدد محاولات إعادة الاتصال
	APIRetryDelay   int    `json:"api_retry_delay"`   // تأخير إعادة المحاولة بالثواني

	// إعدادات FRP Client
	FRPEnabled    bool   `json:"frp_enabled"`    // تفعيل FRP Client
	FRPServerAddr string `json:"frp_server"`     // عنوان خادم FRP
	FRPServerPort int    `json:"frp_port"`       // منفذ خادم FRP
	FRPToken      string `json:"frp_token"`      // رمز المصادقة
	FRPLocalPort  int    `json:"frp_local_port"` // المنفذ المحلي
	FRPProxyName  string `json:"frp_proxy_name"` // اسم الوكيل

	// إعدادات GoASIC
	GoASICEnabled      bool   `json:"goasic_enabled"`       // تفعيل GoASIC
	GoASICScanInterval int    `json:"goasic_scan_interval"` // فترة المسح بالثواني
	GoASICNetworkRange string `json:"goasic_network_range"` // نطاق الشبكة للمسح

	// إعدادات التحديث
	UpdateCheckURL  string `json:"update_check_url"`  // عنوان التحقق من التحديثات
	UpdateAutoCheck bool   `json:"update_auto_check"` // التحقق التلقائي من التحديثات
	UpdateInterval  int    `json:"update_interval"`   // فترة التحقق بالساعات
	UpdateChannel   string `json:"update_channel"`    // قناة التحديث (stable/beta)

	// إعدادات الإضافات
	PluginPath    string `json:"plugin_path"`    // مسار مجلد الإضافات
	PluginEnabled bool   `json:"plugin_enabled"` // تفعيل نظام الإضافات

	// إعدادات التسجيل
	LogLevel   string `json:"log_level"`    // مستوى التسجيل (debug/info/warn/error)
	LogPath    string `json:"log_path"`     // مسار ملف السجل
	LogMaxSize int    `json:"log_max_size"` // الحجم الأقصى للسجل بالميجابايت

	// إعدادات الأمان
	SecureMode    bool   `json:"secure_mode"`     // وضع الأمان
	EncryptAPIKey bool   `json:"encrypt_api_key"` // تشفير مفتاح API
	EncryptionKey string `json:"encryption_key"`  // مفتاح التشفير (غير محفوظ)

	// مسار ملف الإعدادات (داخلي)
	configPath string `json:"-"`
}

// FRPConfig إعدادات FRP Client المستخرجة
// يتم استخدامها لإنشاء ملف تكوين FRP في الذاكرة
type FRPConfig struct {
	ServerAddr string `toml:"serverAddr"`
	ServerPort int    `toml:"serverPort"`
	Token      string `toml:"auth.token"`
	LocalPort  int    `toml:"-"`
	ProxyName  string `toml:"-"`
}

// =============================================================================
// مدير الإعدادات
// =============================================================================

// ConfigManager يدير إعدادات التطبيق
// يدعم التحميل والحفظ والتشفير بشكل آمن
type ConfigManager struct {
	config     *Config
	configPath string
	mu         sync.RWMutex
}

// NewConfigManager ينشئ مدير إعدادات جديد
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		config:     getDefaultConfig(),
		configPath: configPath,
	}
}

// getDefaultConfig يعيد الإعدادات الافتراضية
func getDefaultConfig() *Config {
	return &Config{
		ApplicationName:   "MinerGate",
		FarmUUID:          "12345678-90ab-cdef-1234-567890abcdef",
		Language:          LangEnglish,
		AutoRefresh:       true,
		RefreshRate:       10,
		Theme:             "dark",
		WindowWidth:       1400,
		WindowHeight:      900,
		APIEndpoint:        "http://[IP_ADDRESS]/api/v1",
		APITargetDevice:    "http://[IP_ADDRESS]/api/v1/devices/report",
		APITimeout:         30,
		APIRetryCount:      3,
		APIRetryDelay:      5,
		FRPEnabled:         false,
		FRPServerAddr:      "",
		FRPServerPort:      7000,
		FRPLocalPort:       7400,
		FRPProxyName:       "minergate",
		GoASICEnabled:      true,
		GoASICScanInterval: 60,
		GoASICNetworkRange: "[IP_ADDRESS]",
		UpdateCheckURL:     "http://[IP_ADDRESS]/update/check",
		UpdateAutoCheck:    true,
		UpdateInterval:     24,
		UpdateChannel:      "stable",
		PluginPath:         "./plugins",
		PluginEnabled:      true,
		LogLevel:           "info",
		LogPath:            "./logs",
		LogMaxSize:         10,
		SecureMode:         true,
		EncryptAPIKey:      true,
	}
}

// Load يحمل الإعدادات من الملف
func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// التحقق من وجود الملف
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		// إنشاء ملف بالإعدادات الافتراضية
		return cm.saveUnlocked()
	}

	// قراءة الملف
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// تحويل JSON إلى هيكل الإعدادات
	if err := json.Unmarshal(data, cm.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	cm.config.configPath = cm.configPath

	return nil
}

// Save يحفظ الإعدادات إلى الملف
func (cm *ConfigManager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.saveUnlocked()
}

// saveUnlocked يحفظ الإعدادات بدون قفل
func (cm *ConfigManager) saveUnlocked() error {
	// التأكد من وجود المجلد
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// تحويل الإعدادات إلى JSON
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// كتابة الملف
	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Get يعيد الإعدادات الحالية
func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// Set يحدث الإعدادات
func (cm *ConfigManager) Set(config *Config) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config = config
}

// =============================================================================
// دوال تحديث الإعدادات
// =============================================================================

// SetLanguage يضبط لغة الواجهة
func (cm *ConfigManager) SetLanguage(lang Language) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.Language = lang
}

// SetAPIKey يضبط مفتاح API مع التشفير
func (cm *ConfigManager) SetAPIKey(key string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.config.EncryptAPIKey && cm.config.EncryptionKey != "" {
		encrypted, err := encryptString(key, cm.config.EncryptionKey)
		if err != nil {
			return err
		}
		cm.config.APIKey = encrypted
	} else {
		cm.config.APIKey = key
	}
	return nil
}

// GetAPIKey يحصل على مفتاح API مع فك التشفير
func (cm *ConfigManager) GetAPIKey() (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.config.EncryptAPIKey && cm.config.EncryptionKey != "" && cm.config.APIKey != "" {
		return decryptString(cm.config.APIKey, cm.config.EncryptionKey)
	}
	return cm.config.APIKey, nil
}

// SetFRPConfig يضبط إعدادات FRP
func (cm *ConfigManager) SetFRPConfig(serverAddr string, serverPort int, token string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.FRPServerAddr = serverAddr
	cm.config.FRPServerPort = serverPort
	cm.config.FRPToken = token
	cm.config.FRPEnabled = true
}

// GetFRPConfig يعيد إعدادات FRP
func (cm *ConfigManager) GetFRPConfig() *FRPConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.config.FRPEnabled {
		return nil
	}

	return &FRPConfig{
		ServerAddr: cm.config.FRPServerAddr,
		ServerPort: cm.config.FRPServerPort,
		Token:      cm.config.FRPToken,
		LocalPort:  cm.config.FRPLocalPort,
		ProxyName:  cm.config.FRPProxyName,
	}
}

// ToggleAutoRefresh يبدل حالة التحديث التلقائي
func (cm *ConfigManager) ToggleAutoRefresh() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.AutoRefresh = !cm.config.AutoRefresh
}

// SetRefreshRate يضبط معدل التحديث
func (cm *ConfigManager) SetRefreshRate(rate int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.RefreshRate = rate
}

// =============================================================================
// دوال التشفير الآمنة
// =============================================================================

// encryptString يشفر نص باستخدام AES-GCM
func encryptString(plaintext, key string) (string, error) {
	// إنشاء مفتاح 32 بايت
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptString يفك تشفير نص باستخدام AES-GCM
func decryptString(ciphertext, key string) (string, error) {
	// إنشاء مفتاح 32 بايت
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// =============================================================================
// نظام الرسائل متعدد اللغات
// =============================================================================

// Messages يحتوي على جميع الرسائل باللغتين
var Messages = map[Language]map[string]string{
	LangEnglish: {
		// العناوين
		"app_title":      "Mining Dashboard",
		"dashboard":      "Dashboard",
		"mining_pool":    "Mining Pool",
		"hardware":       "Hardware",
		"energy":         "Energy",
		"hashrate_index": "Hashrate Index",
		"settings":       "Settings",

		// المقاييس
		"online_devices":     "Online Devices",
		"total_hashrate":     "Total Hashrate",
		"total_power":        "Total Power",
		"est_daily_btc":      "Est. Daily BTC",
		"est_daily_profit":   "Est. Daily Profit",
		"est_monthly_profit": "Est. Monthly Profit",
		"hashrate_24h":       "Hashrate (TH/s) 24h",
		"temperature_power":  "Temperature & Power 24h",
		"miner_actions":      "Miner Actions",
		"restart":            "Restart",
		"get_errors":         "Get Errors",
		"fan_status":         "Fan Status",
		"get_pools":          "Get Pools",
		"power_info":         "Power Info",

		// حالة الجهاز
		"hashrate":    "Hashrate",
		"temperature": "Temperature",
		"power":       "Power",
		"uptime":      "Uptime",
		"efficiency":  "Efficiency",
		"errors":      "Errors",
		"online":      "Online",
		"offline":     "Offline",

		// التحديث
		"auto_refresh": "Auto-refresh",
		"refresh":      "Refresh",
		"updated":      "Updated",

		// الأجهزة
		"devices": "Devices",

		// FRP
		"frp_connected":    "FRP Connected",
		"frp_disconnected": "FRP Disconnected",
		"frp_connecting":   "FRP Connecting...",

		// التحديثات
		"update_available": "Update Available",
		"update_now":       "Update Now",
		"checking_updates": "Checking for updates...",

		// الرسوم البيانية
		"chart_hashrate":   "Hashrate History (24h)",
		"chart_temp_power": "Temperature & Power (24h)",
		"chart_1d":         "1D",
		"chart_1w":         "1W",
	},
	LangArabic: {
		// العناوين
		"app_title":      "لوحة تحكم التعدين",
		"dashboard":      "لوحة التحكم",
		"mining_pool":    "مجمع التعدين",
		"hardware":       "الأجهزة",
		"energy":         "الطاقة",
		"hashrate_index": "مؤشر معدل التجزئة",
		"settings":       "الإعدادات",

		// المقاييس
		"online_devices":     "الأجهزة المتصلة",
		"total_hashrate":     "معدل التجزئة الإجمالي",
		"total_power":        "الطاقة الإجمالية",
		"est_daily_btc":      "التقدير اليومي BTC",
		"est_daily_profit":   "الربح اليومي التقديري",
		"est_monthly_profit": "الربح الشهري التقديري",
		"hashrate_24h":       "معدل التجزئة (TH/s) 24س",
		"temperature_power":  "درجة الحرارة والطاقة 24س",
		"miner_actions":      "إجراءات المُعدّن",
		"restart":            "إعادة التشغيل",
		"get_errors":         "الحصول على الأخطاء",
		"fan_status":         "حالة المراوح",
		"get_pools":          "الحصول على المجمعات",
		"power_info":         "معلومات الطاقة",

		// حالة الجهاز
		"hashrate":    "معدل التجزئة",
		"temperature": "درجة الحرارة",
		"power":       "الطاقة",
		"uptime":      "وقت التشغيل",
		"efficiency":  "الكفاءة",
		"errors":      "الأخطاء",
		"online":      "متصل",
		"offline":     "غير متصل",

		// التحديث
		"auto_refresh": "تحديث تلقائي",
		"refresh":      "تحديث",
		"updated":      "تم التحديث",

		// الأجهزة
		"devices": "الأجهزة",

		// FRP
		"frp_connected":    "FRP متصل",
		"frp_disconnected": "FRP غير متصل",
		"frp_connecting":   "FRP جاري الاتصال...",

		// التحديثات
		"update_available": "تحديث متاح",
		"update_now":       "تحديث الآن",
		"checking_updates": "جاري التحقق من التحديثات...",

		// الرسوم البيانية
		"chart_hashrate":   "سجل معدل التجزئة (24س)",
		"chart_temp_power": "درجة الحرارة والطاقة (24س)",
		"chart_1d":         "1ي",
		"chart_1w":         "1أ",
	},
}

// GetMessage يحصل على رسالة باللغة المحددة
func GetMessage(lang Language, key string) string {
	if msgs, ok := Messages[lang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	// fallback to English
	if msgs, ok := Messages[LangEnglish]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	return key
}

// T دالة مختصرة للحصول على رسالة باللغة الحالية
func (cm *ConfigManager) T(key string) string {
	return GetMessage(cm.Get().Language, key)
}

// =============================================================================
// دوال مساعدة
// =============================================================================

// GenerateEncryptionKey يولد مفتاح تشفير جديد
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// ValidateConfig يتحقق من صحة الإعدادات
func (cm *ConfigManager) ValidateConfig() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cfg := cm.config

	// التحقق من معدل التحديث
	if cfg.RefreshRate < 1 || cfg.RefreshRate > 3600 {
		return fmt.Errorf("invalid refresh rate: %d (must be 1-3600)", cfg.RefreshRate)
	}

	// التحقق من مهلة API
	if cfg.APITimeout < 1 || cfg.APITimeout > 300 {
		return fmt.Errorf("invalid API timeout: %d (must be 1-300)", cfg.APITimeout)
	}

	// التحقق من إعدادات FRP إذا كانت مفعلة
	if cfg.FRPEnabled {
		if cfg.FRPServerAddr == "" {
			return fmt.Errorf("FRP server address is required when FRP is enabled")
		}
		if cfg.FRPServerPort < 1 || cfg.FRPServerPort > 65535 {
			return fmt.Errorf("invalid FRP server port: %d", cfg.FRPServerPort)
		}
	}

	return nil
}

// GetLastModified يعيد تاريخ آخر تعديل لملف الإعدادات
func (cm *ConfigManager) GetLastModified() (time.Time, error) {
	info, err := os.Stat(cm.configPath)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
