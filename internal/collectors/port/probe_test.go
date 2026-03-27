package port

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"
)

func TestPingPort(t *testing.T) {
	target, openPort, closeListener := startTestTCPListener(t)
	defer closeListener()

	result, err := PingPort(context.Background(), PingOptions{
		Target:   target,
		Port:     openPort,
		Count:    3,
		Timeout:  300 * time.Millisecond,
		Interval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PingPort() error = %v", err)
	}
	if result.SuccessCount != 3 {
		t.Fatalf("SuccessCount = %d, want 3", result.SuccessCount)
	}
	if result.FailureCount != 0 {
		t.Fatalf("FailureCount = %d, want 0", result.FailureCount)
	}
	if len(result.Attempts) != 3 {
		t.Fatalf("attempts len = %d, want 3", len(result.Attempts))
	}
}

func TestPingPortFailure(t *testing.T) {
	target, closedPort := reserveClosedPort(t)

	result, err := PingPort(context.Background(), PingOptions{
		Target:   target,
		Port:     closedPort,
		Count:    2,
		Timeout:  100 * time.Millisecond,
		Interval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PingPort() error = %v", err)
	}
	if result.SuccessCount != 0 {
		t.Fatalf("SuccessCount = %d, want 0", result.SuccessCount)
	}
	if result.FailureCount != 2 {
		t.Fatalf("FailureCount = %d, want 2", result.FailureCount)
	}
}

func TestScanPorts(t *testing.T) {
	target, openPort, closeListener := startTestTCPListener(t)
	defer closeListener()
	_, closedPort := reserveClosedPort(t)

	result, err := ScanPorts(context.Background(), ScanOptions{
		Target:  target,
		Mode:    ScanModeQuick,
		Ports:   []int{openPort, closedPort},
		Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("ScanPorts() error = %v", err)
	}
	if result.OpenCount != 1 {
		t.Fatalf("OpenCount = %d, want 1", result.OpenCount)
	}
	if !slices.Contains(result.OpenPorts, openPort) {
		t.Fatalf("open ports = %v, want contains %d", result.OpenPorts, openPort)
	}
	if len(result.Results) != 2 {
		t.Fatalf("results len = %d, want 2", len(result.Results))
	}
}

func startTestTCPListener(t *testing.T) (string, int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		t.Fatalf("addr type = %T, want *net.TCPAddr", ln.Addr())
	}

	stopCh := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stopCh:
					return
				default:
					return
				}
			}
			_ = conn.Close()
		}
	}()

	closeFn := func() {
		close(stopCh)
		_ = ln.Close()
	}
	return "127.0.0.1", tcpAddr.Port, closeFn
}

func reserveClosedPort(t *testing.T) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		t.Fatalf("addr type = %T, want *net.TCPAddr", ln.Addr())
	}
	port := tcpAddr.Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener error = %v", err)
	}
	return "127.0.0.1", port
}
