package networkprobe

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"syskit/internal/errs"
	"testing"
	"time"
)

func TestParseTraceProtocol(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    TraceProtocol
		wantErr bool
	}{
		{name: "default", raw: "", want: TraceProtocolICMP},
		{name: "icmp", raw: "icmp", want: TraceProtocolICMP},
		{name: "tcp", raw: "TCP", want: TraceProtocolTCP},
		{name: "invalid", raw: "udp", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTraceProtocol(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %s, got %s", tt.want, got)
			}
		})
	}
}

func TestPingCollectsStats(t *testing.T) {
	call := 0
	withTestRunner(t, "linux", func(ctx context.Context, name string, args ...string) ([]byte, error) {
		t.Helper()
		if name != "ping" {
			t.Fatalf("command = %s, want ping", name)
		}
		if !containsArg(args, "-c") {
			t.Fatalf("linux ping should contain -c flag, args=%v", args)
		}

		call++
		switch call {
		case 1:
			return []byte("64 bytes from 127.0.0.1: icmp_seq=1 ttl=64 time=0.12 ms"), nil
		case 2:
			return []byte("Request timed out."), errors.New("exit status 1")
		default:
			return []byte("来自 127.0.0.1 的回复: 字节=32 时间=5ms TTL=128"), nil
		}
	})

	result, err := Ping(context.Background(), PingOptions{
		Target:   "127.0.0.1",
		Count:    3,
		Interval: 0,
		Timeout:  time.Second,
		Size:     32,
	})
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if result.SuccessCount != 2 || result.FailureCount != 1 {
		t.Fatalf("success/failure = %d/%d, want 2/1", result.SuccessCount, result.FailureCount)
	}
	if len(result.Attempts) != 3 {
		t.Fatalf("attempts len = %d, want 3", len(result.Attempts))
	}
	if result.Attempts[1].Success {
		t.Fatalf("second attempt should fail: %#v", result.Attempts[1])
	}
	if result.Attempts[1].Error != "timeout" {
		t.Fatalf("second attempt error = %s, want timeout", result.Attempts[1].Error)
	}
	if result.Attempts[0].TTL == 0 {
		t.Fatalf("first attempt ttl should be parsed: %#v", result.Attempts[0])
	}
	if result.Attempts[2].TTL == 0 {
		t.Fatalf("third attempt ttl should be parsed: %#v", result.Attempts[2])
	}
}

func TestPingCommandNotFound(t *testing.T) {
	withTestRunner(t, "linux", func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	})

	_, err := Ping(context.Background(), PingOptions{Target: "localhost", Count: 1, Timeout: time.Second})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if errs.ErrorCode(err) != errs.CodeDependencyMissing {
		t.Fatalf("error code = %s, want %s", errs.ErrorCode(err), errs.CodeDependencyMissing)
	}
}

func TestParseTracerouteHops(t *testing.T) {
	output := []byte("traceroute to 8.8.8.8 (8.8.8.8), 30 hops max\n 1  192.168.1.1  1.00 ms  1.20 ms  1.10 ms\n 2  * * *\n 3  8.8.8.8  10.10 ms  11.20 ms  12.30 ms\n")
	hops := parseTracerouteHops(output)
	if len(hops) != 3 {
		t.Fatalf("hops len = %d, want 3", len(hops))
	}
	if !hops[1].Timeout {
		t.Fatalf("hop 2 should be timeout: %#v", hops[1])
	}
	if hops[2].IP != "8.8.8.8" {
		t.Fatalf("hop 3 ip = %s, want 8.8.8.8", hops[2].IP)
	}
	if len(hops[2].RTTsMs) != 3 {
		t.Fatalf("hop 3 rtts len = %d, want 3", len(hops[2].RTTsMs))
	}
}

func TestTracerouteWindowsTCPFallback(t *testing.T) {
	withTestRunner(t, "windows", func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "tracert" {
			t.Fatalf("command = %s, want tracert", name)
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "-h 5") {
			t.Fatalf("args missing -h 5: %v", args)
		}
		return []byte("Tracing route to 8.8.8.8\n 1    <1 ms    <1 ms    <1 ms  192.168.1.1\n 2    10 ms    11 ms    12 ms  8.8.8.8\n"), nil
	})

	result, err := Traceroute(context.Background(), TracerouteOptions{
		Target:   "8.8.8.8",
		MaxHops:  5,
		Timeout:  2 * time.Second,
		Protocol: TraceProtocolTCP,
	})
	if err != nil {
		t.Fatalf("Traceroute() error = %v", err)
	}
	if !result.Reached {
		t.Fatalf("expected reached=true, got false: %#v", result)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected fallback warning, got none")
	}
	if !strings.Contains(result.Warnings[0], "降级") {
		t.Fatalf("unexpected warning: %v", result.Warnings)
	}
}

func TestTracerouteCommandNotFound(t *testing.T) {
	withTestRunner(t, "linux", func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	})

	_, err := Traceroute(context.Background(), TracerouteOptions{Target: "localhost"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if errs.ErrorCode(err) != errs.CodeDependencyMissing {
		t.Fatalf("error code = %s, want %s", errs.ErrorCode(err), errs.CodeDependencyMissing)
	}
}

func withTestRunner(t *testing.T, goos string, runner commandExecFunc) {
	t.Helper()
	oldRunner := commandRunner
	oldRuntime := runtimeName
	commandRunner = runner
	runtimeName = goos
	t.Cleanup(func() {
		commandRunner = oldRunner
		runtimeName = oldRuntime
	})
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
