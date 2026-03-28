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
			w.Header().Set("X-Syskit-Operator", "ExampleNet")
			_, _ = io.WriteString(w, "ip=203.0.113.10\n")
			_, _ = io.WriteString(w, "loc=HK\n")
			_, _ = io.WriteString(w, "colo=HKG\n")
			_, _ = io.WriteString(w, "visit_scheme=https\n")
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
			time.Sleep(2 * time.Millisecond)
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
	if result.Trace == nil {
		t.Fatalf("trace info should not be nil")
	}
	if result.Trace.Operator != "ExampleNet" {
		t.Fatalf("operator = %s, want ExampleNet", result.Trace.Operator)
	}
	if result.Trace.Location != "HK" || result.Trace.Colo != "HKG" {
		t.Fatalf("unexpected trace location: %#v", result.Trace)
	}
	if result.Download.Mbps <= 0 || result.Upload.Mbps <= 0 {
		t.Fatalf("invalid throughput result: %#v", result)
	}
	if len(result.Phases) != 3 {
		t.Fatalf("phases len = %d, want 3", len(result.Phases))
	}
	if result.Assessment == nil || result.Assessment.Summary == "" {
		t.Fatalf("assessment should not be empty: %#v", result.Assessment)
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

func TestCollectSpeedReportsProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cdn-cgi/trace":
			_, _ = io.WriteString(w, "ip=203.0.113.11\n")
		case "/__down":
			_, _ = w.Write(bytes.Repeat([]byte("x"), 1024))
		case "/__up":
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	events := make([]string, 0, 6)
	_, err := CollectSpeed(context.Background(), SpeedOptions{
		Server:  server.URL,
		Mode:    SpeedModeFull,
		Timeout: 5 * time.Second,
		Progress: func(event SpeedProgressEvent) {
			events = append(events, event.Stage+":"+event.Message)
		},
	})
	if err != nil {
		t.Fatalf("CollectSpeed() error = %v", err)
	}

	wantStages := []string{
		"ping:开始延迟与公网出口探测",
		"ping:延迟与出口探测完成",
		"download:开始下载测速",
		"download:下载测速完成",
		"upload:开始上传测速",
		"upload:上传测速完成",
	}
	if !reflect.DeepEqual(events, wantStages) {
		t.Fatalf("progress events mismatch\nwant: %v\ngot:  %v", wantStages, events)
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
