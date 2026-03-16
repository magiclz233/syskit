package rules

import (
	"context"
	"testing"
)

func TestNewP0RulesContainsRequiredIDs(t *testing.T) {
	rules := NewP0Rules()
	want := []string{"PORT-001", "PORT-002", "PROC-001", "PROC-002", "CPU-001", "MEM-001", "DISK-001", "DISK-002", "FILE-001", "ENV-001"}
	if len(rules) != len(want) {
		t.Fatalf("rule count want=%d got=%d", len(want), len(rules))
	}

	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID())
	}
	for idx, id := range want {
		if ids[idx] != id {
			t.Fatalf("rule[%d] want=%s got=%s", idx, id, ids[idx])
		}
	}
}

func TestEachP0RuleCanTrigger(t *testing.T) {
	in := DiagnoseInput{
		Options: DiagnoseOptions{
			Thresholds: RuleThresholds{
				CPUPercent:          80,
				MemPercent:          70,
				DiskPercent:         85,
				FileSizeGB:          1,
				SwapPercent:         40,
				FileGrowthMBPerHour: 300,
			},
		},
		Snapshots: DiagnoseSnapshots{
			Ports: []PortSnapshot{
				{Port: 8080, PID: 101, ProcessName: "node", Command: "node app.js", LocalAddr: "127.0.0.1:8080", ParentPID: 1},
				{Port: 3000, PID: 102, ProcessName: "python", Command: "python app.py", LocalAddr: "0.0.0.0:3000", ParentPID: 1},
			},
			Processes: []ProcessSnapshot{
				{PID: 201, Name: "cpu-burn", CPUPercent: 95.5, Command: "stress --cpu 4"},
			},
			CPU: &CPUOverviewSnapshot{
				CPUCores:     4,
				UsagePercent: 93,
				Load1:        9.2,
				Load5:        8.1,
				Load15:       6.5,
				TopProcesses: []ProcessSnapshot{{PID: 201, Name: "cpu-burn", CPUPercent: 95.5}},
			},
			Memory: &MemoryOverview{
				TotalBytes:       100 * 1024 * 1024,
				AvailableBytes:   10 * 1024 * 1024,
				UsagePercent:     90,
				SwapUsagePercent: 50,
			},
			MemoryTop: []MemoryProcess{
				{PID: 301, Name: "java", MemPercent: 88, RSSBytes: 6 * 1024 * 1024 * 1024, SwapBytes: 256 * 1024 * 1024},
			},
			Disk: []DiskPartition{
				{MountPoint: "/", UsagePercent: 92, FreeBytes: 2 * 1024 * 1024 * 1024},
			},
			DiskGrowth: []DiskGrowthSample{
				{MountPoint: "/", GrowthRateGBPerDay: 20, BaselineGBPerDay: 5, WindowDays: 7},
			},
			Files: []FileObservation{
				{Path: "/var/log/app.log", SizeBytes: 2 * 1024 * 1024 * 1024, GrowthMBPerHour: 500},
			},
			PathEntries: []string{"/usr/bin", "/usr/local/bin", "/usr/bin"},
		},
	}

	for _, rule := range NewP0Rules() {
		issue, err := rule.Check(context.Background(), in)
		if err != nil {
			t.Fatalf("rule %s check failed: %v", rule.ID(), err)
		}
		if issue == nil {
			t.Fatalf("rule %s should trigger", rule.ID())
		}
		if issue.RuleID != rule.ID() {
			t.Fatalf("rule %s issue id mismatch: %s", rule.ID(), issue.RuleID)
		}
	}
}

func TestPort002RespectsAllowPublicListen(t *testing.T) {
	rules := NewP0Rules()
	var target Rule
	for _, rule := range rules {
		if rule.ID() == "PORT-002" {
			target = rule
			break
		}
	}
	if target == nil {
		t.Fatalf("PORT-002 not found")
	}

	in := DiagnoseInput{
		Options: DiagnoseOptions{
			Policy: RulePolicy{AllowPublicListen: []string{"python"}},
		},
		Snapshots: DiagnoseSnapshots{
			Ports: []PortSnapshot{{Port: 3000, PID: 1, ProcessName: "python", LocalAddr: "0.0.0.0:3000"}},
		},
	}

	issue, err := target.Check(context.Background(), in)
	if err != nil {
		t.Fatalf("PORT-002 check failed: %v", err)
	}
	if issue != nil {
		t.Fatalf("PORT-002 should be skipped by allow_public_listen")
	}
}
