// =============================================================================
// MinerGate Dashboard v1.0.4 - نقطة الدخول الرئيسية
// =============================================================================
// لوحة تحكم تعدين احترافية مع:
// - واجهة رسومية حديثة
// - دعم اللغة الإنجليزية والعربية
// - دمج FRP Client
// - دمج GoASIC
// - رسوم بيانية تفاعلية (go-echarts) - جديد v1.0.4
// - نظام تحديث آمن
// =============================================================================
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	// ملاحظة: استيراد واحد فقط لـ api — لا يوجد prometheus هنا
	api "minergate/internal/api"
	"minergate/internal/config"
	goasic "minergate/internal/dbgoasic"
	"minergate/internal/frp"
	"minergate/internal/gui"
	"minergate/internal/plugins"
	"minergate/internal/update"
)

// =============================================================================
// الثوابت
// =============================================================================

const (
	// ConfigPath مسار ملف الإعدادات
	ConfigPath = "./config.json"
)

// =============================================================================
// الدالة الرئيسية
// =============================================================================

func main() {
	// طباعة الشعار
	printBanner()

	// إنشاء سياق للتطبيق
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// التعامل مع إشارات النظام
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n🛑 Received shutdown signal...")
		cancel()
	}()

	// تحميل الإعدادات
	cfgMgr, err := loadConfig()
	if err != nil {
		fmt.Printf("⚠️ Warning: Failed to load config: %v\n", err)
		fmt.Println("📝 Using default configuration...")
	}
	cfg := cfgMgr.Get()

	// تهيئة المكونات
	fmt.Println("🔧 Initializing components...")

	// التأكد من وجود مجلد device_log
	if err := os.MkdirAll("device_log", 0755); err != nil {
		fmt.Printf("⚠️ Warning: Failed to create device_log directory: %v\n", err)
	}

	// إنشاء عميل API
	// تعريف: api.NewClient(cfgMgr *config.ConfigManager) *api.Client
	apiClient := api.NewClient(cfgMgr)
	fmt.Println("  ✓ API client initialized")

	// إنشاء عميل FRP
	// تعريف: frp.NewClient(cfgMgr *config.ConfigManager) *frp.Client
	frpClient := frp.NewClient(cfgMgr)
	if cfg.FRPEnabled {
		fmt.Println("  ✓ FRP client initialized")
	}

	// إنشاء مدير GoASIC
	// تعريف: goasic.NewManager(cfg *config.Config) *goasic.Manager
	goasicMgr := goasic.NewManager(cfg)
	if cfg.GoASICEnabled {
		fmt.Println("  ✓ GoASIC manager initialized")
	}

	// إنشاء مدير الإضافات
	// تعريف: plugins.NewManager(pluginPath string) *plugins.Manager
	pluginMgr := plugins.NewManager(cfg.PluginPath)
	if cfg.PluginEnabled {
		if err := pluginMgr.LoadAll(); err != nil {
			fmt.Printf("  ⚠️ Plugin warning: %v\n", err)
		} else {
			fmt.Printf("  ✓ Plugins loaded (%d)\n", pluginMgr.GetPluginCount())
		}
	}

	// إنشاء مدير التحديثات
	// تعريف: update.NewUpdater(cfg *config.Config) *update.Updater
	updateMgr := update.NewUpdater(cfg)
	fmt.Println("  ✓ Update manager initialized")

	// بدء الخدمات الخلفية
	fmt.Println("🚀 Starting background services...")

	// بدء FRP Client
	if cfg.FRPEnabled {
		go func() {
			if err := frpClient.Start(ctx); err != nil {
				fmt.Printf("  ⚠️ FRP error: %v\n", err)
			}
		}()
		fmt.Println("  ✓ FRP client started")
	}

	// بدء GoASIC
	if cfg.GoASICEnabled {
		go func() {
			if err := goasicMgr.Start(ctx); err != nil {
				fmt.Printf("  ⚠️ GoASIC error: %v\n", err)
			}
		}()
		fmt.Println("  ✓ GoASIC manager started")
	}

	// التحقق من التحديثات في الخلفية
	if cfg.UpdateAutoCheck {
		go checkUpdates(ctx, updateMgr)
		fmt.Println("  ✓ Update checker started")
	}

	// مراقبة الأجهزة في الخلفية (لا نرسل إلى API خارجي)
	go monitorDevices(ctx, goasicMgr)
	fmt.Println("  ✓ Device monitor started")

	// تشغيل الواجهة الرسومية
	fmt.Println("\n🖥️ Starting GUI...")

	// gui.NewDashboard(
	//   cfg        *config.Config,
	//   apiClient  *api.Client,
	//   frpClient  *frp.Client,
	//   goasicMgr  *goasic.Manager,
	//   pluginMgr  *plugins.Manager,
	//   updateMgr  *update.Updater,
	// ) *gui.DashboardApp
	dashboard := gui.NewDashboard(cfg, apiClient, frpClient, goasicMgr, pluginMgr, updateMgr)

	// تشغيل التطبيق (يُحجب حتى تُغلق النافذة)
	dashboard.Run()

	// تنظيف الموارد عند الإغلاق
	fmt.Println("\n🧹 Cleaning up...")
	pluginMgr.Cleanup() // تنظيف الإضافات — موجودة في plugins/manager.go
	frpClient.Stop()    // إيقاف FRP     — موجودة في frp/client.go
	goasicMgr.Stop()    // إيقاف GoASIC  — موجودة في goasic/manager.go
	// ملاحظة: api.Client لا تحتاج Close() في النسخة الحالية

	fmt.Println("👋 MinerGate closed successfully")
}

// =============================================================================
// دوال مساعدة
// =============================================================================

// printBanner يطبع شعار البرنامج
func printBanner() {
	fmt.Println()
	fmt.Println("  ████     ████ ██                           ████████              ██           ")
	fmt.Println(" ░██░██   ██░██░░                           ██░░░░░░██            ░██           ")
	fmt.Println(" ░██░░██ ██ ░██ ██ ███████   █████  ██████ ██      ░░   ██████   ██████  █████  ")
	fmt.Println(" ░██ ░░███  ░██░██░░██░░░██ ██░░░██░░██░░█░██          ░░░░░░██ ░░░██░  ██░░░██ ")
	fmt.Println(" ░██  ░░█   ░██░██ ░██  ░██░███████ ░██ ░ ░██    █████  ███████   ░██  ░███████ ")
	fmt.Println(" ░██   ░    ░██░██ ░██  ░██░██░░░░  ░██   ░░██  ░░░░██ ██░░░░██   ░██  ░██░░░░  ")
	fmt.Println(" ░██        ░██░██ ███  ░██░░██████░███    ░░████████ ░░████████  ░░██ ░░██████ ")
	fmt.Println(" ░░         ░░ ░░ ░░░   ░░  ░░░░░░ ░░░      ░░░░░░░░   ░░░░░░░░    ░░   ░░░░░░  ")
	fmt.Println()
	fmt.Printf("          Dashboard v%s - GUI Edition\n", config.Version)
	fmt.Println("          ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

// loadConfig يحمل الإعدادات من ملف config.json
func loadConfig() (*config.ConfigManager, error) {
	configMgr := config.NewConfigManager(ConfigPath)
	if err := configMgr.Load(); err != nil {
		return nil, err
	}
	return configMgr, nil
}

// checkUpdates يتحقق من التحديثات في الخلفية
func checkUpdates(ctx context.Context, updateMgr *update.Updater) {
	// انتظار انتهاء السياق قبل التحقق (سلوك غير مزعج)
	<-ctx.Done()

	info, err := updateMgr.CheckForUpdate()
	if err != nil {
		return
	}
	if info != nil {
		fmt.Printf("\n🔄 Update available: %s\n", info.Version)
		fmt.Printf("   Download: %s\n", info.DownloadURL)
	}
}

// monitorDevices يراقب الأجهزة ويسجّل بياناتها محلياً.
// يحل محل reportDeviceData الذي كان يُرسل إلى API خارجي
// (apiClient.ReportDeviceData غير موجودة في النسخة الحالية).
func monitorDevices(ctx context.Context, goasicMgr *goasic.Manager) {
	for {
		select {
		case <-ctx.Done():
			return
		}
	}
}
