// =============================================================================
// MinerGate Dashboard v1.0.4 - نقطة الدخول الرئيسية
// =============================================================================
// لوحة تحكم تعدين احترافية مع:
// - واجهة رسومية حديثة
// - دعم اللغة الإنجليزية والعربية
// - دمج FRP Client
// - دمج GoASIC
// - رسوم بيانية تفاعلية
// - نظام تحديث آمن
// =============================================================================
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"minergate/internal/api"
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

	// Make sure device_log directory exists
	if err := os.MkdirAll("device_log", 0755); err != nil {
		fmt.Printf("⚠️ Warning: Failed to create device_log directory: %v\n", err)
	}

	// إنشاء عميل API
	apiClient := api.NewClient(cfgMgr)
	fmt.Println("  ✓ API client initialized")

	// إنشاء عميل FRP
	frpClient := frp.NewClient(cfgMgr)
	if cfg.FRPEnabled {
		fmt.Println("  ✓ FRP client initialized")
	}

	// إنشاء مدير GoASIC
	goasicMgr := goasic.NewManager(cfg)
	if cfg.GoASICEnabled {
		fmt.Println("  ✓ GoASIC manager initialized")
	}

	// إنشاء مدير الإضافات
	pluginMgr := plugins.NewManager(cfg.PluginPath)
	if cfg.PluginEnabled {
		if err := pluginMgr.LoadAll(); err != nil {
			fmt.Printf("  ⚠️ Plugin warning: %v\n", err)
		} else {
			fmt.Printf("  ✓ Plugins loaded (%d)\n", pluginMgr.GetPluginCount())
		}
	}

	// إنشاء محدث التحديثات
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
		// // Scan the network for miners
		// err := goasicMgr.ScanNetwork(ctx)
		// if err != nil {
		// 	fmt.Printf("  ⚠️ Failed to scan network: %v\n", err)
		// } else {
		// 	devices := goasicMgr.GetDevices()
		// 	if len(devices) == 0 {
		// 		fmt.Println("  ⚠️ No miners found on the network")
		// 	} else {
		// 		fmt.Printf("  ✓ Found %d miner(s) on the network:\n", len(devices))
		// 		for _, device := range devices {
		// 			fmt.Printf("    - IP: %s | Model: %s | Make: %s | Status: %s\n", device.IP, device.Model, device.Make, device.Status)
		// 		}
		// 	}
		// }
	}

	// التحقق من التحديثات في الخلفية
	if cfg.UpdateAutoCheck {
		go checkUpdates(ctx, updateMgr)
		fmt.Println("  ✓ Update checker started")
	}

	// بدء إرسال بيانات الأجهزة
	go reportDeviceData(ctx, apiClient, goasicMgr)
	fmt.Println("  ✓ Device reporter started")

	// تشغيل الواجهة الرسومية
	fmt.Println("\n🖥️ Starting GUI...")
	dashboard := gui.NewDashboard(cfg, apiClient, frpClient, goasicMgr, pluginMgr, updateMgr)

	// تشغيل التطبيق
	dashboard.Run()

	// تنظيف الموارد
	fmt.Println("\n🧹 Cleaning up...")
	pluginMgr.Cleanup()
	apiClient.Close()
	frpClient.Stop()
	goasicMgr.Stop()

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

// loadConfig يحمل الإعدادات
func loadConfig() (*config.ConfigManager, error) {
	configMgr := config.NewConfigManager(ConfigPath)
	if err := configMgr.Load(); err != nil {
		return nil, err
	}
	return configMgr, nil
}

// checkUpdates يتحقق من التحديثات
func checkUpdates(ctx context.Context, updateMgr *update.Updater) {
	// انتظار قصير قبل التحقق
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

// reportDeviceData يرسل بيانات الأجهزة بشكل دوري
func reportDeviceData(ctx context.Context, apiClient *api.Client, goasicMgr *goasic.Manager) {
	// قناة للإرسال الدوري
	ticker := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker:
				// جمع التقارير
				reports := goasicMgr.GenerateDeviceReports()
				if len(reports) > 0 {
					if err := apiClient.ReportDeviceData(ctx, reports); err != nil {
						// تسجيل الخطأ بشكل صامت
					}
				}
			}
		}
	}()

	// إرسال أولي
	ticker <- struct{}{}
}
