// =============================================================================
// Package plugins - مدير الإضافات
// =============================================================================
// نظام الإضافات يوفر:
// - تحميل الإضافات الديناميكية من ملفات .so
// - واجهة موحدة للتفاعل مع الإضافات
// - إدارة دورة حياة الإضافات
// - أمان عند تحميل الإضافات
// =============================================================================
package plugins

import (
	"fmt"
	"minergate/internal/models"
	"os"
	"path/filepath"
	"plugin"
	"sync"

	project_root_directory "github.com/golang-infrastructure/go-project-root-directory"
)

// =============================================================================
// واجهة الإضافة
// =============================================================================

// PluginInterface الواجهة التي يجب أن تنفذها جميع الإضافات
// كل إضافة يجب أن تصدر رمزاً باسم "Plugin" ينفذ هذه الواجهة
type PluginInterface interface {
	// GetName يعيد اسم الإضافة
	GetName() string

	// GetVersion يعيد إصدار الإضافة
	GetVersion() string

	// GetDescription يعيد وصف الإضافة
	GetDescription() string

	// Initialize يهيئ الإضافة مع البيانات الأساسية
	Initialize(config map[string]interface{}) error

	// OnMinerUpdate يُستدعى عند تحديث بيانات جهاز التعدين
	OnMinerUpdate(miner *models.Miner) error

	// OnFarmUpdate يُستدعى عند تحديث بيانات المزرعة
	OnFarmUpdate(farm *models.Farm) error

	// OnAction يُستدعى عند تنفيذ إجراء
	OnAction(action string, minerID string) (bool, error)

	// Cleanup ينظف الموارد عند إغلاق الإضافة
	Cleanup() error
}

// =============================================================================
// هيئات البيانات
// =============================================================================

// LoadedPlugin إضافة محملة
type LoadedPlugin struct {
	Info     models.PluginInfo
	Instance PluginInterface
}

// Manager مدير الإضافات
type Manager struct {
	pluginPath string
	plugins    map[string]*LoadedPlugin
	errors     []error
	mu         sync.RWMutex
}

// NewManager ينشئ مدير إضافات جديد
func NewManager(pluginPath string) *Manager {
	return &Manager{
		pluginPath: pluginPath,
		plugins:    make(map[string]*LoadedPlugin),
		errors:     make([]error, 0),
	}
}

// =============================================================================
// العمليات الأساسية
// =============================================================================

// LoadAll يحمل جميع الإضافات من المجلد
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	directory, err := project_root_directory.GetRootDirectory()
	// التحقق من وجود المجلد
	if _, err := os.Stat(m.pluginPath); os.IsNotExist(err) {
		if directory != "" {
			return fmt.Errorf("plugin does not export 'Plugin': %w (searched in %s)", err, directory)
		} else {
			return fmt.Errorf("plugin directory not found: %s", m.pluginPath)
		}

	}

	// البحث عن ملفات الإضافات
	files, err := filepath.Glob(filepath.Join(m.pluginPath, "*.so"))
	if err != nil {
		return fmt.Errorf("failed to search for plugins: %w", err)
	}

	// تحميل كل إضافة
	for _, file := range files {
		if err := m.loadUnlocked(file); err != nil {
			m.errors = append(m.errors, fmt.Errorf("failed to load %s: %w", file, err))
		}
	}

	return nil
}

// Load يحمل إضافة واحدة
func (m *Manager) Load(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadUnlocked(path)
}

// loadUnlocked يحمل إضافة بدون قفل
func (m *Manager) loadUnlocked(path string) error {
	// فتح ملف الإضافة
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}
	directory, _ := project_root_directory.GetRootDirectory()
	// البحث عن الرمز "Plugin"
	sym, err := p.Lookup("Plugin")
	if err != nil {
		if directory != "" {
			return fmt.Errorf("plugin does not export 'Plugin': %w (searched in %s)", err, directory)
		} else {
			return fmt.Errorf("plugin does not export 'Plugin': %w", err)
		}
	}

	// التحقق من الواجهة
	pluginInstance, ok := sym.(PluginInterface)
	if !ok {
		return fmt.Errorf("plugin 'Plugin' does not implement PluginInterface")
	}

	// تهيئة الإضافة
	if err := pluginInstance.Initialize(nil); err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// تخزين الإضافة
	loaded := &LoadedPlugin{
		Info: models.PluginInfo{
			Name:        pluginInstance.GetName(),
			Version:     pluginInstance.GetVersion(),
			Description: pluginInstance.GetDescription(),
			Path:        path,
			Enabled:     true,
			Loaded:      true,
		},
		Instance: pluginInstance,
	}

	m.plugins[loaded.Info.Name] = loaded
	return nil
}

// Unload يفرغ إضافة
func (m *Manager) Unload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// تنظيف الموارد
	if err := plugin.Instance.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup plugin: %w", err)
	}

	delete(m.plugins, name)
	return nil
}

// =============================================================================
// الحصول على الإضافات
// =============================================================================

// GetPlugin يحصل على إضافة محملة
func (m *Manager) GetPlugin(name string) (*LoadedPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}
	return plugin, nil
}

// GetAllPlugins يحصل على جميع الإضافات
func (m *Manager) GetAllPlugins() []*LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*LoadedPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// GetPluginInfos يحصل على معلومات جميع الإضافات
func (m *Manager) GetPluginInfos() []models.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]models.PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		infos = append(infos, p.Info)
	}
	return infos
}

// =============================================================================
// الإشعارات
// =============================================================================

// NotifyMinerUpdate يخطر جميع الإضافات بتحديث جهاز
func (m *Manager) NotifyMinerUpdate(miner *models.Miner) []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errors := make([]error, 0)
	for name, p := range m.plugins {
		if !p.Info.Enabled {
			continue
		}
		if err := p.Instance.OnMinerUpdate(miner); err != nil {
			errors = append(errors, fmt.Errorf("plugin %s: %w", name, err))
		}
	}
	return errors
}

// NotifyFarmUpdate يخطر جميع الإضافات بتحديث المزرعة
func (m *Manager) NotifyFarmUpdate(farm *models.Farm) []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errors := make([]error, 0)
	for name, p := range m.plugins {
		if !p.Info.Enabled {
			continue
		}
		if err := p.Instance.OnFarmUpdate(farm); err != nil {
			errors = append(errors, fmt.Errorf("plugin %s: %w", name, err))
		}
	}
	return errors
}

// NotifyAction يخطر جميع الإضافات بإجراء
func (m *Manager) NotifyAction(action, minerID string) (bool, []error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errors := make([]error, 0)
	proceed := true

	for name, p := range m.plugins {
		if !p.Info.Enabled {
			continue
		}
		allowed, err := p.Instance.OnAction(action, minerID)
		if err != nil {
			errors = append(errors, fmt.Errorf("plugin %s: %w", name, err))
		}
		if !allowed {
			proceed = false
		}
	}

	return proceed, errors
}

// =============================================================================
// التفعيل والتعطيل
// =============================================================================

// Enable يفعل إضافة
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}
	plugin.Info.Enabled = true
	return nil
}

// Disable يعطل إضافة
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}
	plugin.Info.Enabled = false
	return nil
}

// =============================================================================
// التنظيف والأخطاء
// =============================================================================

// Cleanup ينظف جميع الإضافات
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, p := range m.plugins {
		if err := p.Instance.Cleanup(); err != nil {
			m.errors = append(m.errors, fmt.Errorf("failed to cleanup %s: %w", name, err))
		}
	}
}

// GetErrors يحصل على قائمة الأخطاء
func (m *Manager) GetErrors() []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errors := make([]error, len(m.errors))
	copy(errors, m.errors)
	return errors
}

// GetPluginCount يعيد عدد الإضافات المحملة
func (m *Manager) GetPluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}

// GetEnabledCount يعيد عدد الإضافات المفعلة
func (m *Manager) GetEnabledCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, p := range m.plugins {
		if p.Info.Enabled {
			count++
		}
	}
	return count
}
