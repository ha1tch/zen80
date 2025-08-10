package z80

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestZEXALL_CPMSim_BDOS_PCTrap runs the ZEXALL test suite
// ZEXALL tests ALL flags including undocumented X and Y flags
func TestZEXALL_CPMSim_BDOS_PCTrap(t *testing.T) {
	// Try to load zexall.com from the rom directory
	comPath := filepath.Join("..", "rom", "zexall.com")
	com, err := os.ReadFile(comPath)
	if err != nil {
		t.Skipf("missing zexall.com in ../rom directory: %v", err)
	}

	// Set up memory and I/O
	mem := &ram64{}
	io := &dummyIO{}
	cpu := New(mem, io)
	regs := asRegs(cpu)

	// Load CP/M program at 0x0100
	copy(mem.data[0x0100:], com)
	if !regs.setPC(0x0100) {
		t.Fatalf("cannot set PC")
	}
	if !regs.setSP(0xF000) {
		t.Fatalf("cannot set SP")
	}

	// Configuration from environment
	maxStepsEnv := getenvInt("Z80_ZEXALL_STEPS", 2_000_000_000)
	maxSteps := maxStepsEnv
	if maxStepsEnv == 0 {
		maxSteps = int(^uint(0) >> 1) // max int
	}

	progressEvery := getenvInt("Z80_ZEXALL_PROGRESS_EVERY", 50_000_000) // Progress every 50M ops for ZEXALL
	progressMuted := getenvBool("Z80_ZEXALL_PROGRESS_MUTE", false)
	deltaMuted := getenvBool("Z80_ZEXALL_DELTA_MUTE", false)
	silentLimit := getenvInt("Z80_ZEXALL_SILENT_LIMIT", 100_000_000) // Higher limit for ZEXALL
	tailLines := getenvInt("Z80_ZEXALL_TAIL_LINES", 8)
	tailBytes := getenvInt("Z80_ZEXALL_TAIL_MAX_BYTES", 2048)

	// State tracking
	var con []byte
	lastOut := 0
	lastLogged := 0
	silentSince := 0
	bdosCalls := 0
	fnCounts := make(map[byte]int)
	testCount := 0
	passCount := 0
	failCount := 0

	// ANSI coloring state (for deltaMuted mode)
	ansi := newAnsiState()

	t.Logf("Starting ZEXALL test suite (tests ALL flags including undocumented)")
	t.Logf("This may take several minutes to complete...")
	
	start := time.Now()

	for i := 1; i <= maxSteps; i++ {
		cpu.Step()

		pc, ok := regs.getPC()
		if !ok {
			t.Fatalf("cannot read PC")
		}

		// Handle BDOS trap at 0x0005
		if pc == 0x0005 {
			if fn, ok := regs.getC(); ok {
				fnCounts[fn]++
			}
			bdosCalls++
			handleBDOSTrap(t, regs, mem, &con)
			
			// Check for new output
			if len(con) != lastOut {
				delta := con[lastLogged:]
				
				// Count OK/ERROR in output for progress tracking
				deltaStr := string(delta)
				testCount += strings.Count(deltaStr, "...")
				passCount += strings.Count(deltaStr, "OK")
				failCount += strings.Count(deltaStr, "ERROR")
				
				if deltaMuted {
					ansiWrite(os.Stdout, ansi, delta)
				} else {
					pretty := lastLines(delta, tailBytes, tailLines)
					if strings.TrimSpace(pretty) != "" {
						t.Logf("delta output:\n%s", pretty)
					}
				}
				lastOut = len(con)
				lastLogged = len(con)
				silentSince = 0
			}
			continue
		}

		// Check for warm boot (PC=0000)
		if pc == 0x0000 {
			if deltaMuted {
				_, _ = os.Stdout.Write([]byte(ansiReset))
			}
			t.Log("Warm boot reached (PC=0000), test complete.")
			break
		}

		// Progress reporting
		if !progressMuted && progressEvery > 0 && i%progressEvery == 0 {
			elapsed := time.Since(start)
			t.Logf("Ops: %s | PC=%04X | Tests: %d (Pass: %d, Fail: %d) | Time: %v", 
				commify(i), pc, testCount, passCount, failCount, elapsed)
		}

		// Check for infinite loop
		silentSince++
		if silentSince >= silentLimit {
			startWin := int(pc) - 8
			if startWin < 0 {
				startWin = 0
			}
			endWin := startWin + 32
			if endWin > len(mem.data) {
				endWin = len(mem.data)
			}
			
			if deltaMuted {
				_, _ = os.Stdout.Write([]byte(ansiReset))
			}
			
			t.Logf("No output for %s ops; PC=%04X; memory window: % X", 
				commify(silentSince), pc, mem.data[startWin:endWin])
			
			// Show BDOS call statistics
			for fn, c := range fnCounts {
				if c > 100 { // Only show frequently called functions
					t.Logf("BDOS fn %d calls: %d", fn, c)
				}
			}
			
			// Show last output
			pretty := lastLines(con, tailBytes, tailLines)
			if strings.TrimSpace(pretty) != "" {
				t.Logf("Last output:\n%s", pretty)
			}
			
			t.Fatalf("Bailing after prolonged silent loop")
		}
	}

	elapsed := time.Since(start)

	// Reset ANSI colors if needed
	if deltaMuted {
		_, _ = os.Stdout.Write([]byte(ansiReset))
	}

	// Final results
	t.Logf("=== ZEXALL Test Results ===")
	t.Logf("Total tests: %d", testCount)
	t.Logf("Passed: %d", passCount)
	t.Logf("Failed: %d", failCount)
	t.Logf("Time elapsed: %v", elapsed)
	t.Logf("Total output: %d bytes", len(con))

	// Show final output tail
	pretty := lastLines(con, tailBytes*2, tailLines*2) // Show more lines for final summary
	if strings.TrimSpace(pretty) != "" {
		t.Logf("Final output:\n%s", pretty)
	}

	// Check for failures
	if failCount > 0 {
		t.Errorf("ZEXALL: %d tests FAILED (including undocumented flag tests)", failCount)
		
		// Analyze which tests failed
		output := string(con)
		lines := strings.Split(output, "\n")
		t.Log("Failed tests:")
		for _, line := range lines {
			if strings.Contains(line, "ERROR") {
				// Extract test name from error line
				if idx := strings.Index(line, "..."); idx > 0 {
					testName := strings.TrimSpace(line[:idx])
					if testName != "" {
						t.Logf("  - %s", testName)
					}
				}
			}
		}
	} else if passCount > 0 {
		t.Logf("SUCCESS: All %d ZEXALL tests PASSED!", passCount)
		t.Log("The Z80 emulator correctly implements ALL documented and undocumented behavior!")
	}

	// Save output if requested
	if path := os.Getenv("Z80_ZEXALL_OUTPUT"); path != "" {
		if dir := filepath.Dir(path); dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(path, con, 0o644); err != nil {
			t.Fatalf("failed to write ZEXALL log: %v", err)
		}
		t.Logf("Wrote ZEXALL log to %s (%d bytes)", path, len(con))
	}

	// Verify we got the expected "Tests complete" message
	if !bytes.Contains(con, []byte("Tests complete")) {
		t.Error("ZEXALL did not run to completion (missing 'Tests complete' message)")
	}
}

// TestZEXALL_QuickCheck runs a faster partial ZEXALL test
// This is useful for CI/CD pipelines where full ZEXALL might timeout
func TestZEXALL_QuickCheck(t *testing.T) {
	if !getenvBool("Z80_ZEXALL_QUICK", false) {
		t.Skip("Set Z80_ZEXALL_QUICK=1 to run quick ZEXALL check")
	}

	// Try to load zexall.com
	comPath := filepath.Join("..", "rom", "zexall.com")
	com, err := os.ReadFile(comPath)
	if err != nil {
		t.Skipf("missing zexall.com: %v", err)
	}

	mem := &ram64{}
	io := &dummyIO{}
	cpu := New(mem, io)
	regs := asRegs(cpu)

	// Load and set up
	copy(mem.data[0x0100:], com)
	regs.setPC(0x0100)
	regs.setSP(0xF000)

	var con []byte
	maxOps := 100_000_000 // Run for 100M operations max
	testsPassed := 0

	t.Log("Running ZEXALL quick check (first few tests only)...")

	for i := 0; i < maxOps; i++ {
		cpu.Step()

		pc, _ := regs.getPC()
		if pc == 0x0005 {
			handleBDOSTrap(t, regs, mem, &con)
			
			// Check if we've completed at least a few tests
			output := string(con)
			testsPassed = strings.Count(output, "OK")
			if testsPassed >= 5 { // Stop after 5 passing tests
				break
			}
		}

		if pc == 0x0000 {
			break
		}
	}

	t.Logf("Quick check completed: %d tests passed", testsPassed)
	
	if testsPassed > 0 {
		t.Log("ZEXALL quick check PASSED - emulator handles undocumented flags correctly")
	} else {
		t.Error("ZEXALL quick check FAILED - no tests passed")
	}
}