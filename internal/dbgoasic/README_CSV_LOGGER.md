# مسجل بيانات أجهزة التعدين إلى CSV

## نظرة عامة
هذا الملف يوفر وظيفة تسجيل قراءات أجهزة التعدين في ملفات CSV داخل مجلد `device_log`.

## الميزات الرئيسية

### 1. تسجيل تلقائي للبيانات
- يتم تسجيل البيانات تلقائياً عند جمعها من أجهزة التعدين
- كل جهاز له ملف CSV منفصل باسم عنوان IP الخاص به
- مثال: جهاز بعنوان `10.0.0.1` سيكون ملفه `10_0_0_1.csv`

### 2. القراءات المسجلة
يتم تسجيل القراءات التالية لكل جهاز:

| الحقل | الوصف | الوحدة |
|-------|-------|--------|
| Timestamp | وقت القراءة | YYYY-MM-DD HH:MM:SS |
| IP | عنوان الجهاز | - |
| Model | موديل الجهاز | - |
| Hashrate_TH_s | معدل التجزئة | TH/s |
| Power_W | استهلاك الطاقة | Watts |
| Temperature_C | درجة الحرارة | Celsius |
| Uptime_S | وقت التشغيل | Seconds |
| FanSpeeds_RPM | سرعات المراوح | RPM (مفصولة بـ ;) |
| Efficiency_J_TH | الكفاءة | J/TH |
| IsMining | حالة التعدين | true/false |
| Status | الحالة العامة | online/offline/error |

### 3. إدارة الملفات
- **إنشاء تلقائي للمجلد**: يتم إنشاء مجلد `device_log` تلقائياً إذا لم يكن موجوداً
- **تدوير الملفات**: عند الوصول إلى 10,000 صف، يتم تدوير الملف القديم وإضافة طابع زمني
- **إغلاق آمن**: يتم إغلاق جميع الملفات بشكل صحيح عند إيقاف التطبيق

## الاستخدام

### إنشاء مسجل جديد
```go
logger, err := goasic.NewCSVLogger("device_log")
if err != nil {
    log.Fatal(err)
}
defer logger.Close()
```

### تسجيل بيانات جهاز
```go
record := goasic.DeviceLogRecord{
    Timestamp:   time.Now(),
    IP:          "10.0.0.1",
    Model:       "Antminer S19",
    Hashrate:    120.5,
    Power:       3200,
    Temperature: 75.3,
    Uptime:      86400,
    FanSpeeds:   []int{1200, 1300},
    Efficiency:  9.5,
    IsMining:    true,
    Status:      "online",
}

err := logger.Log(record)
if err != nil {
    log.Printf("Failed to log: %v", err)
}
```

### استخدام LogFromDeviceInfo
```go
err := logger.LogFromDeviceInfo(
    "10.0.0.1",           // IP
    "Antminer S19",       // Model
    "online",             // Status
    &hashrate,            // *float64
    &power,               // *int
    temperature,          // []float64
    &uptime,              // *uint64
    fanSpeeds,            // []int
    &efficiency,          // *float64
    true,                 // IsMining
)
```

## التكامل مع Manager

يتم دمج مسجل CSV تلقائياً مع `Manager` في `internal/dbgoasic/manager.go`:

```go
func NewManager(cfg *config.Config) *Manager {
    csvLogger, err := NewCSVLogger(DefaultLogDir)
    if err != nil {
        fmt.Printf("⚠️ WARNING: Failed to create CSV logger: %v\n", err)
    }
    
    return &Manager{
        cfg:       cfg,
        devices:   make(map[string]*DiscoveredDevice),
        csvLogger: csvLogger,
        // ...
    }
}
```

عند تحديث بيانات الجهاز، يتم تسجيلها تلقائياً:

```go
func (m *Manager) collectDeviceData(ctx context.Context, device *DiscoveredDevice) {
    // ... جمع البيانات ...
    
    if m.csvLogger != nil && deviceInfo.Hashrate != nil {
        m.csvLogger.LogFromDeviceInfo(...)
    }
}
```

## مثال على ملف CSV

```csv
Timestamp,IP,Model,Hashrate_TH_s,Power_W,Temperature_C,Uptime_S,FanSpeeds_RPM,Efficiency_J_TH,IsMining,Status
2026-03-21 15:30:00,10.0.0.1,Antminer S19,120.50,3200,75.3,86400,1200;1300,9.50,true,online
2026-03-21 15:30:30,10.0.0.1,Antminer S19,121.20,3250,76.1,86430,1250;1350,9.45,true,online
2026-03-21 15:31:00,10.0.0.1,Antminer S19,119.80,3180,74.8,86460,1180;1280,9.55,true,online
```

## الاختبارات

يحتوي الملف على اختبارات وحدة شاملة:

```bash
cd /workspace
go test ./internal/dbgoasic/... -v
```

### تغطية الاختبارات:
- ✅ إنشاء المسجل وإنشاء المجلد
- ✅ تسجيل سجل واحد
- ✅ تسجيل من DeviceInfo
- ✅ التعامل مع القيم nil
- ✅ تسجيل متعدد لنفس الجهاز
- ✅ تسجيل لعدة أجهزة
- ✅ الحصول على مسار السجل
- ✅ إغلاق المسجل
- ✅ تنسيق سرعات المراوح
- ✅ تحويل IP إلى اسم ملف
- ✅ الكتابة المتزامنة (Concurrent)

## أفضل الممارسات

1. **إغلاق المسجل**: تأكد من استدعاء `Close()` عند إيقاف التطبيق
2. **التعامل مع الأخطاء**: تحقق من أخطاء التسجيل وسجلها
3. **الأداء**: استخدم المسجل بشكل غير متزامن لتجنب إبطاء جمع البيانات
4. **الصيانة**: نظف الملفات القديمة دورياً باستخدام `CleanupOldLogs()`

## تنظيف الملفات القديمة

```go
// حذف الملفات الأقدم من 7 أيام
err := logger.CleanupOldLogs(7 * 24 * time.Hour)
if err != nil {
    log.Printf("Failed to cleanup old logs: %v", err)
}
```

## الثوابت القابلة للتكوين

```go
const (
    DefaultLogDir    = "device_log"  // المجلد الافتراضي
    MaxLogRows       = 10000         // الحد الأقصى للصفوف قبل التدوير
    LogFileExtension = ".csv"        // امتداد الملفات
)
```

## التوافق

- Go 1.19+
- يعمل على Linux, macOS, Windows
- متوافق مع نظام الملفات POSIX
