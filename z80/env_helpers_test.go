package z80

import (
	"fmt"
	"os"
)

// getenvInt reads an integer from an environment variable, falling back
// to def if unset or unparseable. Used by opcov_runtime_rom_test.go for
// its step budget. Previously provided incidentally by zexdoc_test.go
// (moved to tools/zex, which keeps its own independent copy) -- kept
// here as its own small file so this package has no dependency on that
// one, or vice versa.
func getenvInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil {
			return n
		}
	}
	return def
}
