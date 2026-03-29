package startup

import "testing"

func TestNewCommandIncludesExpectedChildren(t *testing.T) {
	cmd := NewCommand()
	children := map[string]bool{}
	for _, child := range cmd.Commands() {
		children[child.Name()] = true
	}

	for _, name := range []string{"list", "enable", "disable"} {
		if !children[name] {
			t.Fatalf("missing child command: %s", name)
		}
	}
}
