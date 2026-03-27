package net

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestParseSpeedMode(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    SpeedMode
		wantErr bool
	}{
		{name: "default", raw: "", want: SpeedModeFull},
		{name: "full", raw: "full", want: SpeedModeFull},
		{name: "download", raw: "download", want: SpeedModeDownload},
		{name: "upload", raw: "UPLOAD", want: SpeedModeUpload},
		{name: "invalid", raw: "mixed", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSpeedMode(tt.raw)
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

func TestCollectSpeedFull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cdn-cgi/trace":
			_, _ = io.WriteString(w, "ip=203.0.113.10\n")
		case "/__down":
			size := int(defaultSpeedDownloadBytes)
			if raw := r.URL.Query().Get("bytes"); raw != "" {
				if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
					size = parsed
				}
			}
			_, _ = w.Write(bytes.Repeat([]byte("a"), size))
		case "/__up":
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := CollectSpeed(context.Background(), SpeedOptions{
		Server:  server.URL,
		Mode:    SpeedModeFull,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("CollectSpeed() error = %v", err)
	}
	if result.Ping == nil || result.Download == nil || result.Upload == nil {
		t.Fatalf("full mode result incomplete: %#v", result)
	}
	if result.PublicIP != "203.0.113.10" {
		t.Fatalf("public ip = %s, want 203.0.113.10", result.PublicIP)
	}
	if result.Download.Mbps <= 0 || result.Upload.Mbps <= 0 {
		t.Fatalf("invalid throughput result: %#v", result)
	}
}

func TestCollectSpeedDownloadOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/__down" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1024))
	}))
	defer server.Close()

	result, err := CollectSpeed(context.Background(), SpeedOptions{
		Server:  server.URL,
		Mode:    SpeedModeDownload,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("CollectSpeed() error = %v", err)
	}
	if result.Ping != nil || result.Upload != nil {
		t.Fatalf("download mode should not include ping/upload: %#v", result)
	}
	if result.Download == nil || result.Download.Bytes == 0 {
		t.Fatalf("download result missing: %#v", result)
	}
}

func TestParseStateFilter(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "empty", raw: "", want: nil},
		{name: "single", raw: "listen", want: []string{"listen"}},
		{name: "multi dedup", raw: "listen,ESTABLISHED,listen", want: []string{"established", "listen"}},
		{name: "invalid", raw: "running", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStateFilter(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("want %v, got %v", tt.want, got)
			}
		})
	}
}

func TestIsListenEntry(t *testing.T) {
	tests := []struct {
		name string
		item ConnectionEntry
		want bool
	}{
		{name: "tcp listen", item: ConnectionEntry{Protocol: "tcp", State: "listen", LocalAddr: "127.0.0.1:80"}, want: true},
		{name: "tcp established", item: ConnectionEntry{Protocol: "tcp", State: "established", LocalAddr: "127.0.0.1:80"}, want: false},
		{name: "udp local", item: ConnectionEntry{Protocol: "udp", State: "none", LocalAddr: "127.0.0.1:53"}, want: true},
		{name: "unknown", item: ConnectionEntry{Protocol: "icmp", State: "none", LocalAddr: "127.0.0.1:0"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isListenEntry(tt.item); got != tt.want {
				t.Fatalf("isListenEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}
