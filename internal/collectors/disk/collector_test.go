package disk

import "testing"

func TestBuildSummary(t *testing.T) {
	partitions := []Partition{
		{TotalBytes: 1000, UsedBytes: 250, FreeBytes: 750},
		{TotalBytes: 3000, UsedBytes: 1500, FreeBytes: 1500},
	}

	summary := buildSummary(partitions)
	if summary.PartitionCount != 2 {
		t.Fatalf("期望分区数为 2，实际为 %d", summary.PartitionCount)
	}
	if summary.TotalBytes != 4000 {
		t.Fatalf("期望总容量为 4000，实际为 %d", summary.TotalBytes)
	}
	if summary.UsedBytes != 1750 {
		t.Fatalf("期望已用容量为 1750，实际为 %d", summary.UsedBytes)
	}
	if summary.FreeBytes != 2250 {
		t.Fatalf("期望空闲容量为 2250，实际为 %d", summary.FreeBytes)
	}
	if summary.UsagePercent != 43.75 {
		t.Fatalf("期望使用率为 43.75，实际为 %.2f", summary.UsagePercent)
	}
}

func TestUsagePercentWithZeroTotal(t *testing.T) {
	if percent := usagePercent(0, 123); percent != 0 {
		t.Fatalf("总容量为 0 时期望使用率为 0，实际为 %.2f", percent)
	}
}
