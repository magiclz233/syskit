package cli

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	gopsprocess "github.com/shirou/gopsutil/v4/process"
)

func TestDoctorAllJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	result := runCLI(t, "--config", configPath, "doctor", "all", "--fail-on", "never", "--format", "json")
	if result.ExitCode != 0 && result.ExitCode != 1 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{"scope", "modules", "health_score", "health_level", "coverage", "fail_on", "fail_on_matched", "issues"} {
		if _, ok := data[field]; !ok {
			t.Fatalf("doctor all data missing field %q: %#v", field, data)
		}
	}
}

func TestDoctorNetworkJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	result := runCLI(t, "--config", configPath, "doctor", "network", "--target", "localhost", "--fail-on", "never", "--format", "json")
	if result.ExitCode != 0 && result.ExitCode != 1 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{"scope", "module", "health_score", "health_level", "coverage", "fail_on", "fail_on_matched", "issues"} {
		if _, ok := data[field]; !ok {
			t.Fatalf("doctor network data missing field %q: %#v", field, data)
		}
	}
	if got := fmt.Sprint(data["module"]); got != "network" {
		t.Fatalf("doctor network module = %s, want network", got)
	}
}

func TestDoctorDiskFullJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)
	scanDir := filepath.Join(root, "scan-disk-full")
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(scanDir) error = %v", err)
	}
	writeSizedFile(t, filepath.Join(scanDir, "big.log"), 4096, 24*time.Hour)
	writeSizedFile(t, filepath.Join(scanDir, "nested", "app.log"), 2048, 24*time.Hour)

	result := runCLI(t, "--config", configPath, "doctor", "disk-full", "--path", scanDir, "--top", "5", "--fail-on", "never", "--format", "json")
	if result.ExitCode != 0 && result.ExitCode != 1 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{"scope", "module", "health_score", "health_level", "coverage", "fail_on", "fail_on_matched", "issues"} {
		if _, ok := data[field]; !ok {
			t.Fatalf("doctor disk-full data missing field %q: %#v", field, data)
		}
	}
	if got := fmt.Sprint(data["module"]); got != "disk-full" {
		t.Fatalf("doctor disk-full module = %s, want disk-full", got)
	}
}

func TestDoctorSlownessJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	result := runCLI(t, "--config", configPath, "doctor", "slowness", "--mode", "quick", "--fail-on", "never", "--format", "json")
	if result.ExitCode != 0 && result.ExitCode != 1 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{"scope", "module", "health_score", "health_level", "coverage", "fail_on", "fail_on_matched", "issues"} {
		if _, ok := data[field]; !ok {
			t.Fatalf("doctor slowness data missing field %q: %#v", field, data)
		}
	}
	if got := fmt.Sprint(data["module"]); got != "slowness" {
		t.Fatalf("doctor slowness module = %s, want slowness", got)
	}
}

func TestCPUBurstJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	result := runCLI(
		t,
		"--config", configPath,
		"cpu", "burst",
		"--interval", "200ms",
		"--duration", "1s",
		"--threshold", "10000",
		"--format", "json",
	)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{
		"interval_ms",
		"duration_ms",
		"continuous",
		"threshold_percent",
		"cpu_cores",
		"sample_count",
		"started_at",
		"ended_at",
		"processes",
	} {
		if _, ok := data[field]; !ok {
			t.Fatalf("cpu burst data missing field %q: %#v", field, data)
		}
	}

	processes := mustSlice(t, data["processes"], "processes")
	if len(processes) != 0 {
		t.Fatalf("cpu burst with threshold=10000 should not hit processes: %#v", processes)
	}
}

func TestCPUWatchJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	result := runCLI(
		t,
		"--config", configPath,
		"cpu", "watch",
		"--interval", "200ms",
		"--top", "5",
		"--threshold-cpu", "10000",
		"--threshold-load", "10000",
		"--timeout", "2s",
		"--format", "json",
	)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{
		"top_n",
		"interval_ms",
		"threshold_cpu",
		"threshold_load",
		"sample_count",
		"stopped_reason",
		"alerts",
		"last_top_processes",
	} {
		if _, ok := data[field]; !ok {
			t.Fatalf("cpu watch data missing field %q: %#v", field, data)
		}
	}
	if got := fmt.Sprint(data["stopped_reason"]); got != "timeout" {
		t.Fatalf("cpu watch stopped_reason = %s, want timeout", got)
	}
}

func TestMemLeakJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	procCmd := startHelperProcess(t, "sleep")
	pid := procCmd.Process.Pid

	result := runCLI(
		t,
		"--config", configPath,
		"mem", "leak", strconv.Itoa(pid),
		"--duration", "1s",
		"--interval", "200ms",
		"--format", "json",
	)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{
		"pid",
		"duration_ms",
		"interval_ms",
		"sample_count",
		"stopped_reason",
		"rss_growth_bytes",
		"rss_growth_rate_mb_min",
		"leak_risk",
		"leak_reason",
		"samples",
	} {
		if _, ok := data[field]; !ok {
			t.Fatalf("mem leak data missing field %q: %#v", field, data)
		}
	}
	if got := int(data["sample_count"].(float64)); got <= 0 {
		t.Fatalf("mem leak sample_count = %d, want > 0", got)
	}
}

func TestMemWatchJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	result := runCLI(
		t,
		"--config", configPath,
		"mem", "watch",
		"--interval", "200ms",
		"--top", "5",
		"--threshold-mem", "10000",
		"--threshold-swap", "10000",
		"--timeout", "2s",
		"--format", "json",
	)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{
		"top_n",
		"interval_ms",
		"threshold_mem",
		"threshold_swap",
		"sample_count",
		"stopped_reason",
		"alerts",
		"last_top_processes",
	} {
		if _, ok := data[field]; !ok {
			t.Fatalf("mem watch data missing field %q: %#v", field, data)
		}
	}
	if got := fmt.Sprint(data["stopped_reason"]); got != "timeout" {
		t.Fatalf("mem watch stopped_reason = %s, want timeout", got)
	}
}

func TestMonitorAllJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, dataDir, _ := writeRuntimeConfig(t, root)

	result := runCLI(
		t,
		"--config", configPath,
		"monitor", "all",
		"--interval", "200ms",
		"--max-samples", "3",
		"--format", "json",
	)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	for _, field := range []string{
		"interval_ms",
		"max_samples",
		"alert_threshold",
		"sample_count",
		"stopped_reason",
		"monitor_file",
		"peaks",
		"alerts",
	} {
		if _, ok := data[field]; !ok {
			t.Fatalf("monitor all data missing field %q: %#v", field, data)
		}
	}

	if got := int(data["sample_count"].(float64)); got != 3 {
		t.Fatalf("monitor all sample_count = %d, want 3", got)
	}
	monitorFile := fmt.Sprint(data["monitor_file"])
	if !strings.HasPrefix(monitorFile, filepath.Join(dataDir, "monitor")) {
		t.Fatalf("monitor_file = %s, want under %s", monitorFile, filepath.Join(dataDir, "monitor"))
	}
	raw, err := os.ReadFile(monitorFile)
	if err != nil {
		t.Fatalf("ReadFile(monitor_file) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("monitor sample lines = %d, want 3", len(lines))
	}
}

func TestMonitorAllInspectionJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)

	result := runCLI(
		t,
		"--config", configPath,
		"monitor", "all",
		"--interval", "200ms",
		"--max-samples", "2",
		"--inspection-interval", "200ms",
		"--inspection-mode", "quick",
		"--inspection-fail-on", "never",
		"--format", "json",
	)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	inspections := mustSlice(t, data["inspections"], "inspections")
	if len(inspections) == 0 {
		t.Fatalf("monitor all inspections is empty: %#v", data)
	}
	first := mustMap(t, inspections[0], "inspections[0]")
	for _, field := range []string{"timestamp", "mode", "fail_on", "exit_code"} {
		if _, ok := first[field]; !ok {
			t.Fatalf("monitor inspection missing field %q: %#v", field, first)
		}
	}
}

func TestDiskScanJSONIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)
	scanDir := filepath.Join(root, "scan")
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(scanDir) error = %v", err)
	}
	writeSizedFile(t, filepath.Join(scanDir, "big.log"), 4096, 48*time.Hour)
	writeSizedFile(t, filepath.Join(scanDir, "small.log"), 16, 48*time.Hour)
	subDir := filepath.Join(scanDir, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(subDir) error = %v", err)
	}
	writeSizedFile(t, filepath.Join(subDir, "nested.bin"), 2048, 48*time.Hour)

	result := runCLI(t, "--config", configPath, "disk", "scan", scanDir, "--limit", "5", "--min-size", "1B", "--format", "json")
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr=%s, err=%v", result.ExitCode, result.Stderr, result.Err)
	}

	payload := parseJSONResult(t, result.Stdout)
	data := mustMap(t, payload["data"], "data")
	if _, ok := data["top_files"]; !ok {
		t.Fatalf("disk scan missing top_files: %#v", data)
	}
	if _, ok := data["top_dirs"]; !ok {
		t.Fatalf("disk scan missing top_dirs: %#v", data)
	}
	files := mustSlice(t, data["top_files"], "top_files")
	if len(files) == 0 {
		t.Fatalf("disk scan top_files is empty: %#v", data)
	}
	first := mustMap(t, files[0], "top_files[0]")
	if !strings.Contains(fmt.Sprint(first["path"]), "big.log") {
		t.Fatalf("unexpected first top file: %#v", first)
	}
}

func TestFixCleanupDryRunAndApply(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	t.Setenv("TMPDIR", root)
	configPath, dataDir, logFile := writeRuntimeConfig(t, root)

	tempFile := writeSizedFile(t, filepath.Join(root, "temp", "old.tmp"), 256, 10*24*time.Hour)
	logTarget := writeSizedFile(t, filepath.Join(filepath.Dir(logFile), "old.log"), 512, 10*24*time.Hour)
	cacheFile := writeSizedFile(t, filepath.Join(dataDir, "cache", "old.cache"), 128, 10*24*time.Hour)
	newFile := writeSizedFile(t, filepath.Join(root, "temp", "new.tmp"), 64, time.Hour)

	dryRun := runCLI(t, "--config", configPath, "fix", "cleanup", "--older-than", "7d", "--format", "json")
	if dryRun.ExitCode != 0 {
		t.Fatalf("dry-run exit code = %d, stderr=%s, err=%v", dryRun.ExitCode, dryRun.Stderr, dryRun.Err)
	}
	dryRunData := mustMap(t, parseJSONResult(t, dryRun.Stdout)["data"], "data")
	if got := fmt.Sprint(dryRunData["mode"]); got != "dry-run" {
		t.Fatalf("dry-run mode = %s, want dry-run", got)
	}
	if !fileExists(tempFile) || !fileExists(logTarget) || !fileExists(cacheFile) {
		t.Fatal("dry-run unexpectedly deleted candidate files")
	}

	apply := runCLI(t, "--config", configPath, "fix", "cleanup", "--older-than", "7d", "--apply", "--format", "json")
	if apply.ExitCode != 0 {
		t.Fatalf("apply exit code = %d, stderr=%s, err=%v", apply.ExitCode, apply.Stderr, apply.Err)
	}
	applyData := mustMap(t, parseJSONResult(t, apply.Stdout)["data"], "data")
	if got := fmt.Sprint(applyData["mode"]); got != "apply" {
		t.Fatalf("apply mode = %s, want apply", got)
	}
	if fileExists(tempFile) || fileExists(logTarget) || fileExists(cacheFile) {
		t.Fatal("apply did not delete old candidate files")
	}
	if !fileExists(newFile) {
		t.Fatal("apply deleted a file newer than threshold")
	}
	assertAuditFileExists(t, dataDir)
}

func TestSnapshotLifecycleIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, dataDir, _ := writeRuntimeConfig(t, root)

	createA := runCLI(t, "--config", configPath, "snapshot", "create", "--name", "snap-a", "--module", "cpu", "--format", "json")
	if createA.ExitCode != 0 {
		t.Fatalf("createA exit code = %d, stderr=%s, err=%v", createA.ExitCode, createA.Stderr, createA.Err)
	}
	snapA := mustMap(t, mustMap(t, parseJSONResult(t, createA.Stdout)["data"], "data")["snapshot"], "snapshot")
	idA := fmt.Sprint(snapA["id"])

	createB := runCLI(t, "--config", configPath, "snapshot", "create", "--name", "snap-b", "--module", "cpu,mem", "--format", "json")
	if createB.ExitCode != 0 {
		t.Fatalf("createB exit code = %d, stderr=%s, err=%v", createB.ExitCode, createB.Stderr, createB.Err)
	}
	snapB := mustMap(t, mustMap(t, parseJSONResult(t, createB.Stdout)["data"], "data")["snapshot"], "snapshot")
	idB := fmt.Sprint(snapB["id"])

	listResult := runCLI(t, "--config", configPath, "snapshot", "list", "--limit", "10", "--format", "json")
	if listResult.ExitCode != 0 {
		t.Fatalf("list exit code = %d, stderr=%s, err=%v", listResult.ExitCode, listResult.Stderr, listResult.Err)
	}
	listData := mustMap(t, parseJSONResult(t, listResult.Stdout)["data"], "data")
	if count := int(listData["count"].(float64)); count < 2 {
		t.Fatalf("snapshot count = %d, want >= 2", count)
	}

	showResult := runCLI(t, "--config", configPath, "snapshot", "show", idA, "--module", "cpu", "--format", "json")
	if showResult.ExitCode != 0 {
		t.Fatalf("show exit code = %d, stderr=%s, err=%v", showResult.ExitCode, showResult.Stderr, showResult.Err)
	}
	showData := mustMap(t, parseJSONResult(t, showResult.Stdout)["data"], "data")
	shown := mustMap(t, showData["snapshot"], "snapshot")
	if fmt.Sprint(shown["id"]) != idA {
		t.Fatalf("snapshot show id = %v, want %s", shown["id"], idA)
	}

	diffResult := runCLI(t, "--config", configPath, "snapshot", "diff", idA, idB, "--format", "json")
	if diffResult.ExitCode != 0 {
		t.Fatalf("diff exit code = %d, stderr=%s, err=%v", diffResult.ExitCode, diffResult.Stderr, diffResult.Err)
	}
	diffData := mustMap(t, parseJSONResult(t, diffResult.Stdout)["data"], "data")
	if fmt.Sprint(diffData["base_id"]) != idA || fmt.Sprint(diffData["target_id"]) != idB {
		t.Fatalf("unexpected diff ids: %#v", diffData)
	}

	deleteDryRun := runCLI(t, "--config", configPath, "snapshot", "delete", idA, "--format", "json")
	if deleteDryRun.ExitCode != 0 {
		t.Fatalf("delete dry-run exit code = %d, stderr=%s, err=%v", deleteDryRun.ExitCode, deleteDryRun.Stderr, deleteDryRun.Err)
	}
	deleteDryRunData := mustMap(t, parseJSONResult(t, deleteDryRun.Stdout)["data"], "data")
	if got := fmt.Sprint(deleteDryRunData["mode"]); got != "dry-run" {
		t.Fatalf("delete dry-run mode = %s, want dry-run", got)
	}

	deleteApply := runCLI(t, "--config", configPath, "snapshot", "delete", idA, "--apply", "--yes", "--format", "json")
	if deleteApply.ExitCode != 0 {
		t.Fatalf("delete apply exit code = %d, stderr=%s, err=%v", deleteApply.ExitCode, deleteApply.Stderr, deleteApply.Err)
	}

	postDeleteList := runCLI(t, "--config", configPath, "snapshot", "list", "--limit", "10", "--format", "json")
	if postDeleteList.ExitCode != 0 {
		t.Fatalf("post-delete list exit code = %d, stderr=%s, err=%v", postDeleteList.ExitCode, postDeleteList.Stderr, postDeleteList.Err)
	}
	items := mustSlice(t, mustMap(t, parseJSONResult(t, postDeleteList.Stdout)["data"], "data")["snapshots"], "snapshots")
	for _, item := range items {
		snapshot := mustMap(t, item, "snapshot")
		if fmt.Sprint(snapshot["id"]) == idA {
			t.Fatalf("snapshot %s still exists after delete", idA)
		}
	}
	assertAuditFileExists(t, dataDir)
}

func TestPolicyValidateIntegration(t *testing.T) {
	root := t.TempDir()
	configPath, _, _ := writeRuntimeConfig(t, root)
	policyPath := writePolicyFile(t, root)

	configResult := runCLI(t, "policy", "validate", configPath, "--type", "config", "--format", "json")
	if configResult.ExitCode != 0 {
		t.Fatalf("config validate exit code = %d, stderr=%s, err=%v", configResult.ExitCode, configResult.Stderr, configResult.Err)
	}
	configData := mustMap(t, parseJSONResult(t, configResult.Stdout)["data"], "data")
	if !configData["valid"].(bool) {
		t.Fatalf("config validate should be valid: %#v", configData)
	}

	policyResult := runCLI(t, "policy", "validate", policyPath, "--type", "policy", "--format", "json")
	if policyResult.ExitCode != 0 {
		t.Fatalf("policy validate exit code = %d, stderr=%s, err=%v", policyResult.ExitCode, policyResult.Stderr, policyResult.Err)
	}
	policyData := mustMap(t, parseJSONResult(t, policyResult.Stdout)["data"], "data")
	if !policyData["valid"].(bool) {
		t.Fatalf("policy validate should be valid: %#v", policyData)
	}
}

func TestProcKillDryRunAndApply(t *testing.T) {
	root := t.TempDir()
	configPath, dataDir, _ := writeRuntimeConfig(t, root)

	procCmd := startHelperProcess(t, "sleep")
	pid := procCmd.Process.Pid

	dryRun := runCLI(t, "--config", configPath, "proc", "kill", strconv.Itoa(pid), "--format", "json")
	if dryRun.ExitCode != 0 {
		t.Fatalf("dry-run exit code = %d, stderr=%s, err=%v", dryRun.ExitCode, dryRun.Stderr, dryRun.Err)
	}
	if !waitForPIDState(t, int32(pid), true, 3*time.Second) {
		t.Fatalf("pid %d should still be alive after dry-run", pid)
	}

	apply := runCLI(t, "--config", configPath, "proc", "kill", strconv.Itoa(pid), "--apply", "--yes", "--format", "json")
	if apply.ExitCode != 0 {
		t.Fatalf("apply exit code = %d, stderr=%s, err=%v", apply.ExitCode, apply.Stderr, apply.Err)
	}
	if !waitForPIDState(t, int32(pid), false, 5*time.Second) {
		t.Fatalf("pid %d still alive after apply", pid)
	}
	_ = procCmd.Wait()
	assertAuditFileExists(t, dataDir)
}

func TestPortKillDryRunAndApply(t *testing.T) {
	root := t.TempDir()
	configPath, dataDir, _ := writeRuntimeConfig(t, root)

	listenerCmd, port := startListenerHelper(t)

	dryRun := runCLI(t, "--config", configPath, "port", "kill", strconv.Itoa(port), "--format", "json")
	if dryRun.ExitCode != 0 {
		t.Fatalf("dry-run exit code = %d, stderr=%s, err=%v", dryRun.ExitCode, dryRun.Stderr, dryRun.Err)
	}
	if !waitForPIDState(t, int32(listenerCmd.Process.Pid), true, 3*time.Second) {
		t.Fatalf("listener pid %d should still be alive after dry-run", listenerCmd.Process.Pid)
	}

	apply := runCLI(t, "--config", configPath, "port", "kill", strconv.Itoa(port), "--apply", "--yes", "--format", "json")
	if apply.ExitCode != 0 {
		t.Fatalf("apply exit code = %d, stderr=%s, err=%v", apply.ExitCode, apply.Stderr, apply.Err)
	}
	if !waitForPIDState(t, int32(listenerCmd.Process.Pid), false, 5*time.Second) {
		t.Fatalf("listener pid %d still alive after apply", listenerCmd.Process.Pid)
	}
	_ = listenerCmd.Wait()
	assertAuditFileExists(t, dataDir)
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("SYSKIT_TEST_HELPER") != "1" {
		return
	}

	args := helperCommandArgs()
	if len(args) == 0 {
		os.Exit(2)
	}

	switch args[0] {
	case "sleep":
		for {
			time.Sleep(time.Hour)
		}
	case "listen":
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer ln.Close()

		port := ln.Addr().(*net.TCPAddr).Port
		fmt.Fprintln(os.Stdout, port)
		for {
			conn, err := ln.Accept()
			if err != nil {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			_ = conn.Close()
		}
	default:
		os.Exit(2)
	}
}

func helperCommandArgs() []string {
	for idx, arg := range os.Args {
		if arg == "--" && idx+1 < len(os.Args) {
			return os.Args[idx+1:]
		}
	}
	return nil
}

func startHelperProcess(t *testing.T, mode string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestCLIHelperProcess", "--", mode)
	cmd.Env = append(os.Environ(), "SYSKIT_TEST_HELPER=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start(%s) error = %v", mode, err)
	}

	t.Cleanup(func() {
		if waitForPIDState(t, int32(cmd.Process.Pid), true, 200*time.Millisecond) {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})
	return cmd
}

func startListenerHelper(t *testing.T) (*exec.Cmd, int) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestCLIHelperProcess", "--", "listen")
	cmd.Env = append(os.Environ(), "SYSKIT_TEST_HELPER=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start(listener helper) error = %v", err)
	}

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ReadString(port) error = %v", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("Atoi(port) error = %v, raw=%q", err, line)
	}
	if !waitForTCPReady(port, 3*time.Second) {
		t.Fatalf("port %d did not become ready", port)
	}

	t.Cleanup(func() {
		if waitForPIDState(t, int32(cmd.Process.Pid), true, 200*time.Millisecond) {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})
	return cmd, port
}

func waitForTCPReady(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func waitForPIDState(t *testing.T, pid int32, wantAlive bool, timeout time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		alive, err := gopsprocess.PidExistsWithContext(ctx, pid)
		if err == nil && alive == wantAlive {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func writeSizedFile(t *testing.T, path string, size int, age time.Duration) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	payload := []byte(strings.Repeat("x", size))
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
	modTime := time.Now().Add(-age)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%s) error = %v", path, err)
	}
	return path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func assertAuditFileExists(t *testing.T, dataDir string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(dataDir, "audit"))
	if err != nil {
		t.Fatalf("ReadDir(audit) error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("audit dir is empty under %s", dataDir)
	}
}
