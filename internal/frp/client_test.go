package frp

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"minergate/internal/config"
)

func TestValidateConfig(t *testing.T) {
	cfgMgr := config.NewConfigManager("")
	// No FRP server configured
	cfgMgr.SetFRPConfig("", 0, "")
	c := NewClient(cfgMgr)
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected error when config is invalid")
	}
}

func TestGenerateFRPConfigString(t *testing.T) {
	cfg := &config.Config{
		FRPEnabled:    true,
		FRPServerAddr: "127.0.0.1",
		FRPServerPort: 7000,
		FRPToken:      "token123",
		FRPProxyName:  "minergate",
		FRPLocalPort:  7400,
	}
	out := GenerateFRPConfigString(cfg)
	if out == "" {
		t.Fatal("expected non-empty config string")
	}
	if !strings.Contains(out, "serverAddr = \"127.0.0.1\"") {
		t.Fatalf("unexpected config output: %s", out)
	}
	if !strings.Contains(out, "auth.token = \"token123\"") {
		t.Fatalf("unexpected config output: %s", out)
	}
}

// startTestFRPServer starts a simple server that accepts one connection and replies with OK.
func startTestFRPServer(t *testing.T) (addr string, cleanup func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	stop := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Immediately respond so the client can continue.
		_, _ = conn.Write([]byte("OK\n"))

		// Drain any incoming data until closed.
		buf := make([]byte, 1024)
		for {
			select {
			case <-stop:
				return
			default:
				conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
				_, _ = conn.Read(buf)
			}
		}
	}()

	return ln.Addr().String(), func() {
		close(stop)
		_ = ln.Close()
	}
}

func TestClientConnects(t *testing.T) {
	addr, cleanup := startTestFRPServer(t)
	defer cleanup()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("invalid address %q: %v", addr, err)
	}

	cfgMgr := config.NewConfigManager("")
	cfgMgr.SetFRPConfig(host, mustAtoi(port), "token123")

	c := NewClient(cfgMgr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Start(ctx); err != nil {
		t.Fatalf("failed to start client: %v", err)
	}

	if err := c.WaitForConnection(ctx, 3*time.Second); err != nil {
		t.Fatalf("expected client to connect: %v", err)
	}

	if !c.IsConnected() {
		t.Fatal("expected connected state")
	}

	c.Stop()
	// Give some time for state to change
	time.Sleep(100 * time.Millisecond)
	if c.IsConnected() {
		t.Fatal("expected disconnected state after stop")
	}
}

func mustAtoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}
