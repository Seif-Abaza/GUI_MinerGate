// =============================================================================
// Package frp - نماذج البيانات
// =============================================================================
package frp

import (
	"time"
)

// FRPConfig تكوين FRP
type FRPConfig struct {
	// إعدادات الخادم
	ServerAddr string `json:"server_addr" toml:"serverAddr"`
	ServerPort int    `json:"server_port" toml:"serverPort"`

	// المصادقة
	Token string `json:"token" toml:"auth.token"`

	// إعدادات الوكيل
	ProxyName string `json:"proxy_name" toml:"-"`
	LocalIP   string `json:"local_ip" toml:"localIP"`
	LocalPort int    `json:"local_port" toml:"localPort"`
	Type      string `json:"type" toml:"type"`

	// إعدادات متقدمة
	HeartbeatInterval int `json:"heartbeat_interval" toml:"transport.heartbeatInterval"`
	HeartbeatTimeout  int `json:"heartbeat_timeout" toml:"transport.heartbeatTimeout"`
}

// ProxyStatus حالة الوكيل
type ProxyStatus struct {
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	LocalAddr       string    `json:"local_addr"`
	RemoteAddr      string    `json:"remote_addr"`
	TrafficIn       int64     `json:"traffic_in"`
	TrafficOut      int64     `json:"traffic_out"`
	LastStart       time.Time `json:"last_start"`
	LastClose       time.Time `json:"last_close"`
	CurrentConns    int       `json:"current_conns"`
}

// ConnectionStats إحصائيات الاتصال
type ConnectionStats struct {
	TotalConnections  int64     `json:"total_connections"`
	ActiveConnections int       `json:"active_connections"`
	BytesSent         int64     `json:"bytes_sent"`
	BytesReceived     int64     `json:"bytes_received"`
	ConnectionTime    time.Time `json:"connection_time"`
	Uptime            time.Duration `json:"uptime"`
}

// ProxyConfig تكوين الوكيل
type ProxyConfig struct {
	Name      string `json:"name" toml:"name"`
	Type      string `json:"type" toml:"type"`
	LocalIP   string `json:"local_ip" toml:"localIP"`
	LocalPort int    `json:"local_port" toml:"localPort"`
	RemotePort int   `json:"remote_port,omitempty" toml:"remotePort,omitempty"`
	CustomDomains []string `json:"custom_domains,omitempty" toml:"customDomains,omitempty"`
	SubDomain string `json:"subdomain,omitempty" toml:"subdomain,omitempty"`
}
