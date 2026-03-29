package rules

import (
	"context"
	"testing"
)

func TestNewP1RulesContainsRequiredIDs(t *testing.T) {
	rules := NewP1Rules()
	want := []string{"NET-001", "SVC-001", "STARTUP-001", "LOG-001"}
	if len(rules) != len(want) {
		t.Fatalf("rule count want=%d got=%d", len(want), len(rules))
	}
	for idx, id := range want {
		if rules[idx].ID() != id {
			t.Fatalf("rule[%d] want=%s got=%s", idx, id, rules[idx].ID())
		}
	}
}

func TestEachP1RuleCanTrigger(t *testing.T) {
	in := DiagnoseInput{
		Options: DiagnoseOptions{
			Thresholds: RuleThresholds{
				ConnectionCount: 100,
			},
			Policy: RulePolicy{
				RequiredServices:     []string{"docker"},
				RequiredStartupItems: []string{"safe-item"},
			},
		},
		ModuleData: map[string]any{
			"net_baseline_count": 20,
		},
		Snapshots: DiagnoseSnapshots{
			Connections: []ConnectionObservation{
				{RemoteAddr: "10.0.0.1:443"},
				{RemoteAddr: "10.0.0.1:443"},
				{RemoteAddr: "10.0.0.2:443"},
			},
			Services: []ServiceObservation{
				{Name: "docker", State: "stopped"},
			},
			StartupItems: []StartupObservation{
				{ID: "stp-1", Name: "unknown", Risk: true, RiskReason: "temp path"},
			},
			Log: &LogObservation{
				ErrorRate:       55,
				GrowthMBPerHour: 210,
				ErrorLines:      55,
				TotalLines:      100,
			},
		},
	}

	for _, rule := range NewP1Rules() {
		local := in
		switch rule.ID() {
		case "NET-001":
			local.Snapshots.Connections = make([]ConnectionObservation, 0, 120)
			for i := 0; i < 120; i++ {
				local.Snapshots.Connections = append(local.Snapshots.Connections, ConnectionObservation{
					RemoteAddr: "10.0.0.1:443",
				})
			}
		case "SVC-001":
			local.Options.Policy.RequiredServices = []string{"docker"}
			local.Snapshots.Services = []ServiceObservation{{Name: "docker", State: "stopped"}}
		case "STARTUP-001":
			local.Snapshots.StartupItems = []StartupObservation{{ID: "stp-risk", Name: "evil", Risk: true}}
		case "LOG-001":
			local.Snapshots.Log = &LogObservation{ErrorRate: 60, GrowthMBPerHour: 230, ErrorLines: 60, TotalLines: 100}
		}

		issue, err := rule.Check(context.Background(), local)
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

func TestP1RuleNotTriggeredWhenHealthy(t *testing.T) {
	in := DiagnoseInput{
		Options: DiagnoseOptions{
			Thresholds: RuleThresholds{
				ConnectionCount: 1000,
			},
			Policy: RulePolicy{
				RequiredServices: []string{"docker"},
			},
		},
		Snapshots: DiagnoseSnapshots{
			Connections: []ConnectionObservation{{RemoteAddr: "10.0.0.1:443"}},
			Services:    []ServiceObservation{{Name: "docker", State: "running"}},
			StartupItems: []StartupObservation{
				{ID: "safe", Name: "safe", Risk: false},
			},
			Log: &LogObservation{
				ErrorRate:       1,
				GrowthMBPerHour: 5,
				ErrorLines:      1,
				TotalLines:      100,
			},
		},
	}
	for _, rule := range NewP1Rules() {
		issue, err := rule.Check(context.Background(), in)
		if err != nil {
			t.Fatalf("rule %s check failed: %v", rule.ID(), err)
		}
		if issue != nil {
			t.Fatalf("rule %s should not trigger, issue=%+v", rule.ID(), issue)
		}
	}
}
