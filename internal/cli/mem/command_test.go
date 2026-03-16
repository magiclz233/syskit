package mem

import "testing"

func TestTopOptionsDefault(t *testing.T) {
	cmd := newTopCommand()
	flags := cmd.Flags()

	by, err := flags.GetString("by")
	if err != nil {
		t.Fatalf("read --by failed: %v", err)
	}
	if by != "rss" {
		t.Fatalf("default --by should be rss, got %s", by)
	}

	topN, err := flags.GetInt("top")
	if err != nil {
		t.Fatalf("read --top failed: %v", err)
	}
	if topN != 20 {
		t.Fatalf("default --top should be 20, got %d", topN)
	}
}
