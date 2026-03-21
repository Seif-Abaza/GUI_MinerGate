// =============================================================================
// Package update - نظام التحديث التلقائي
// =============================================================================
// نظام التحديث يوفر:
// - التحقق التلقائي من التحديثات
// - تحميل التحديثات بشكل آمن مع التحقق من SHA256
// - تثبيت التحديثات مع إمكانية الاستعادة
// - التحقق من القناة (stable/beta)
// =============================================================================
//
// عملية التحديث:
// 1. يتم التحقق من الخادم عن طريق UpdateCheckURL
// 2. إذا كان هناك تحديث متاح، يتم تحميله
// 3. يتم التحقق من المجموع الاختباري SHA256
// 4. يتم تثبيت التحديث باستخدام go-update
// 5. يتم إعادة تشغيل البرنامج
//
// الأمان:
// - جميع التحديثات يجب أن تكون موقعة رقمياً (في الإنتاج)
// - يتم التحقق من SHA256 قبل التثبيت
// - يتم الاحتفاظ بنسخة احتياطية للاستعادة في حالة الفشل
// =============================================================================
package update

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/inconshreveable/go-update"

	"minergate/internal/config"
	"minergate/internal/models"
)

// =============================================================================
// هيئات البيانات
// =============================================================================

// Updater مدير التحديثات
type Updater struct {
	// التكوين
	currentVersion string
	checkURL       string
	channel        string

	// عميل HTTP
	httpClient *http.Client

	// معلومات التحديث
	availableUpdate *models.UpdateInfo
	lastChecked     time.Time

	// حالة التحميل
	downloading     bool
	downloadProgress float64
}

// UpdateResult نتيجة التحديث
type UpdateResult struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	NewVersion   string    `json:"new_version,omitempty"`
	Error        string    `json:"error,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// CheckResponse استجابة التحقق من الخادم
type CheckResponse struct {
	UpdateAvailable bool               `json:"update_available"`
	LatestVersion   string             `json:"latest_version"`
	UpdateInfo      *models.UpdateInfo `json:"update_info,omitempty"`
}

// =============================================================================
// إنشاء المحدث
// =============================================================================

// NewUpdater ينشئ محدث جديد
func NewUpdater(cfg *config.Config) *Updater {
	return &Updater{
		currentVersion: config.Version,
		checkURL:       cfg.UpdateCheckURL,
		channel:        cfg.UpdateChannel,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// =============================================================================
// التحقق من التحديثات
// =============================================================================

// CheckForUpdate يتحقق من وجود تحديث جديد
// هذه الدالة تتصل بالخادم للحصول على معلومات التحديث
func (u *Updater) CheckForUpdate() (*models.UpdateInfo, error) {
	// بناء URL للتحقق
	checkURL := fmt.Sprintf("%s?version=%s&os=%s&arch=%s&channel=%s",
		u.checkURL,
		u.currentVersion,
		runtime.GOOS,
		runtime.GOARCH,
		u.channel,
	)

	// إنشاء الطلب
	req, err := http.NewRequest("GET", checkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create check request: %w", err)
	}

	// إضافة رؤوس
	req.Header.Set("User-Agent", "MinerGate/"+u.currentVersion)
	req.Header.Set("X-Client-Version", u.currentVersion)
	req.Header.Set("X-OS", runtime.GOOS)
	req.Header.Set("X-Arch", runtime.GOARCH)

	// تنفيذ الطلب
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check for update: %w", err)
	}
	defer resp.Body.Close()

	// التحقق من حالة الاستجابة
	if resp.StatusCode == http.StatusNoContent {
		// لا يوجد تحديث
		u.lastChecked = time.Now()
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	// قراءة الاستجابة
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// تحليل الاستجابة
	var checkResp CheckResponse
	if err := json.Unmarshal(body, &checkResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	u.lastChecked = time.Now()

	if !checkResp.UpdateAvailable || checkResp.UpdateInfo == nil {
		return nil, nil
	}

	// تخزين معلومات التحديث
	u.availableUpdate = checkResp.UpdateInfo

	return checkResp.UpdateInfo, nil
}

// =============================================================================
// تحميل التحديث
// =============================================================================

// DownloadUpdate يحمل ملف التحديث
// هذه الدالة تقوم بتحميل التحديث وإرجاع البيانات والمجموع الاختباري
func (u *Updater) DownloadUpdate(progressCallback func(progress float64)) ([]byte, string, error) {
	if u.availableUpdate == nil {
		return nil, "", fmt.Errorf("no update available")
	}

	u.downloading = true
	u.downloadProgress = 0
	defer func() {
		u.downloading = false
	}()

	// إنشاء الطلب
	resp, err := u.httpClient.Get(u.availableUpdate.DownloadURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// الحصول على حجم الملف
	var totalSize int64
	if resp.ContentLength > 0 {
		totalSize = resp.ContentLength
	}

	// قراءة البيانات مع تتبع التقدم
	var buffer bytes.Buffer
	buffer.Grow(int(totalSize))

	// استخدام io.CopyBuffer لقراءة البيانات
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			buffer.Write(buf[:n])
			downloaded += int64(n)

			if totalSize > 0 && progressCallback != nil {
				u.downloadProgress = float64(downloaded) / float64(totalSize) * 100
				progressCallback(u.downloadProgress)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to read download: %w", err)
		}
	}

	data := buffer.Bytes()

	// التحقق من المجموع الاختباري
	checksum := u.availableUpdate.Checksum
	if checksum != "" {
		computedChecksum := computeSHA256(data)
		if computedChecksum != checksum {
			return nil, "", fmt.Errorf("checksum mismatch: expected %s, got %s", checksum, computedChecksum)
		}
	}

	return data, checksum, nil
}

// =============================================================================
// تثبيت التحديث
// =============================================================================

// ApplyUpdate يطبق التحديث المحمل
// هذه الدالة تستبدل الملف التنفيذي الحالي
// وتحتفظ بنسخة احتياطية للاستعادة في حالة الفشل
func (u *Updater) ApplyUpdate(data []byte, checksum string) error {
	// التحقق من المجموع الاختباري مرة أخرى
	if checksum != "" {
		computedChecksum := computeSHA256(data)
		if computedChecksum != checksum {
			return fmt.Errorf("checksum verification failed before apply")
		}
	}

	// تطبيق التحديث
	// go-update تتولى الاستعادة التلقائية في حالة الفشل
	err := update.Apply(bytes.NewReader(data), update.Options{
		// يمكن إضافة خيارات إضافية هنا
	})

	if err != nil {
		// محاولة الاستعادة
		if rerr := update.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %v, %v", err, rerr)
		}
		return fmt.Errorf("update failed (rollback successful): %w", err)
	}

	return nil
}

// =============================================================================
// التحديث الكامل
// =============================================================================

// CheckAndApply يتحقق ويطبق التحديث تلقائياً
// هذه الدالة تجمع بين جميع الخطوات
func (u *Updater) CheckAndApply(progressCallback func(progress float64)) (*UpdateResult, error) {
	// التحقق من التحديث
	info, err := u.CheckForUpdate()
	if err != nil {
		return nil, fmt.Errorf("check failed: %w", err)
	}

	if info == nil {
		return &UpdateResult{
			Success:   true,
			Message:   "No update available",
			Timestamp: time.Now(),
		}, nil
	}

	// تحميل التحديث
	data, checksum, err := u.DownloadUpdate(progressCallback)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// تطبيق التحديث
	if err := u.ApplyUpdate(data, checksum); err != nil {
		return nil, fmt.Errorf("apply failed: %w", err)
	}

	return &UpdateResult{
		Success:    true,
		Message:    "Update applied successfully",
		NewVersion: info.Version,
		Timestamp:  time.Now(),
	}, nil
}

// UpdateFromFile يطبق تحديث من ملف محلي
func (u *Updater) UpdateFromFile(filepath string) error {
	// قراءة الملف
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read update file: %w", err)
	}

	// تطبيق التحديث
	return u.ApplyUpdate(data, "")
}

// =============================================================================
// الحالة والمعلومات
// =============================================================================

// GetCurrentVersion يعيد الإصدار الحالي
func (u *Updater) GetCurrentVersion() string {
	return u.currentVersion
}

// GetAvailableUpdate يعيد معلومات التحديث المتاح
func (u *Updater) GetAvailableUpdate() *models.UpdateInfo {
	return u.availableUpdate
}

// IsUpdateAvailable يتحقق من وجود تحديث متاح
func (u *Updater) IsUpdateAvailable() bool {
	return u.availableUpdate != nil
}

// GetLastChecked يعيد وقت آخر تحقق
func (u *Updater) GetLastChecked() time.Time {
	return u.lastChecked
}

// IsDownloading يتحقق مما إذا كان التحميل جارياً
func (u *Updater) IsDownloading() bool {
	return u.downloading
}

// GetDownloadProgress يعيد تقدم التحميل
func (u *Updater) GetDownloadProgress() float64 {
	return u.downloadProgress
}

// GetUpdateStatus يعيد حالة التحديث الكاملة
func (u *Updater) GetUpdateStatus() *models.UpdateStatus {
	status := &models.UpdateStatus{
		CurrentVersion:  u.currentVersion,
		LastChecked:     u.lastChecked,
		UpdateAvailable: u.availableUpdate != nil,
		Downloading:     u.downloading,
		DownloadProgress: u.downloadProgress,
	}

	if u.availableUpdate != nil {
		status.LatestVersion = u.availableUpdate.Version
		status.UpdateInfo = u.availableUpdate
	}

	return status
}

// =============================================================================
// دوال مساعدة
// =============================================================================

// computeSHA256 يحسب المجموع الاختباري SHA256
func computeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// =============================================================================
// محدث الخلفية
// =============================================================================

// BackgroundUpdater يدير التحديثات في الخلفية
type BackgroundUpdater struct {
	updater      *Updater
	interval     time.Duration
	resultChan   chan UpdateResult
	stopChan     chan struct{}
	running      bool
}

// NewBackgroundUpdater ينشئ محدث خلفية جديد
func NewBackgroundUpdater(updater *Updater, checkIntervalHours int) *BackgroundUpdater {
	return &BackgroundUpdater{
		updater:    updater,
		interval:   time.Duration(checkIntervalHours) * time.Hour,
		resultChan: make(chan UpdateResult, 10),
		stopChan:   make(chan struct{}),
	}
}

// Start يبدأ التحقق الدوري
func (bu *BackgroundUpdater) Start() {
	if bu.running {
		return
	}

	bu.running = true
	go bu.run()
}

// run يشغل حلقة التحقق
func (bu *BackgroundUpdater) run() {
	// تحقق أولي
	bu.check()

	// تحقق دوري
	ticker := time.NewTicker(bu.interval)
	defer ticker.Stop()

	for {
		select {
		case <-bu.stopChan:
			return
		case <-ticker.C:
			bu.check()
		}
	}
}

// check يقوم بالتحقق
func (bu *BackgroundUpdater) check() {
	info, err := bu.updater.CheckForUpdate()
	if err != nil {
		bu.resultChan <- UpdateResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
		return
	}

	if info != nil {
		bu.resultChan <- UpdateResult{
			Success:    true,
			Message:    "Update available",
			NewVersion: info.Version,
			Timestamp:  time.Now(),
		}
	}
}

// Stop يوقف التحقق الدوري
func (bu *BackgroundUpdater) Stop() {
	if !bu.running {
		return
	}

	close(bu.stopChan)
	bu.running = false
}

// Results يعيد قناة النتائج
func (bu *BackgroundUpdater) Results() <-chan UpdateResult {
	return bu.resultChan
}

// IsRunning يتحقق مما إذا كان المحدث يعمل
func (bu *BackgroundUpdater) IsRunning() bool {
	return bu.running
}
