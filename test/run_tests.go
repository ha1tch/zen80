// +build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Test categories and their descriptions
var testSuites = []struct {
	name        string
	pkg         string
	description string
	tests       []string
}{
	{
		"Z80 Core",
		"github.com/ha1tch/zen80/test",
		"Core Z80 instruction tests",
		[]string{
			"TestUndocumentedFlags",
			"TestBlockInstructions",
			"TestInterrupts",
			"TestIndexedInstructions",
			"TestDAA",
			"TestStackOperations",
			"TestEdgeCases",
		},
	},
	{
		"Spectrum System",
		"github.com/ha1tch/zen80/test",
		"ZX Spectrum system emulation",
		[]string{
			"TestSpectrumMemoryBanking",
			"TestSpectrumIO",
			"TestSpectrumInterruptTiming",
			"TestSpectrumTiming",
			"TestSpectrumRealTimeSync",
			"TestSpectrumSpeedControl",
			"TestScreenMemoryMapping",
			"TestKeyboardMatrix",
		},
	},
	{
		"Integration",
		"github.com/ha1tch/zen80/test",
		"Complex program execution",
		[]string{
			"TestComplexProgram",
			"TestSelfModifyingCode",
			"TestRecursion",
			"TestIOEcho",
			"TestConditionalExecution",
			"TestStringOperations",
		},
	},
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                  ZEN80 COMPREHENSIVE TEST SUITE           ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	totalTests := 0
	totalPassed := 0
	totalFailed := 0
	startTime := time.Now()

	for _, suite := range testSuites {
		fmt.Printf("▶ Running %s Tests\n", suite.name)
		fmt.Printf("  %s\n\n", suite.description)

		for _, test := range suite.tests {
			totalTests++
			if runTest(suite.pkg, test) {
				totalPassed++
			} else {
				totalFailed++
			}
		}
		fmt.Println()
	}

	// Run benchmarks
	fmt.Println("▶ Running Benchmarks")
	runBenchmarks()

	// Summary
	elapsed := time.Since(startTime)
	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("Total Tests:  %d\n", totalTests)
	fmt.Printf("Passed:       %d (%.1f%%)\n", totalPassed, float64(totalPassed)*100/float64(totalTests))
	fmt.Printf("Failed:       %d\n", totalFailed)
	fmt.Printf("Time:         %.2fs\n", elapsed.Seconds())
	
	if totalFailed == 0 {
		fmt.Println("\n✅ ALL TESTS PASSED!")
	} else {
		fmt.Printf("\n❌ %d TESTS FAILED\n", totalFailed)
		os.Exit(1)
	}
}

func runTest(pkg, testName string) bool {
	cmd := exec.Command("go", "test", "-v", "-run", "^"+testName+"$", pkg)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		fmt.Printf("  ❌ %s - FAILED\n", testName)
		if len(output) > 0 {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Error:") || strings.Contains(line, "FAIL") {
					fmt.Printf("     %s\n", line)
				}
			}
		}
		return false
	}
	
	fmt.Printf("  ✅ %s - PASSED\n", testName)
	return true
}

func runBenchmarks() {
	benchmarks := []string{
		"BenchmarkInstructionExecution",
		"BenchmarkSpectrumFrame",
		"BenchmarkComplexProgram",
	}
	
	for _, bench := range benchmarks {
		cmd := exec.Command("go", "test", "-bench", bench, "-benchtime", "1s", "./test")
		output, err := cmd.CombinedOutput()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Benchmark") {
					fmt.Printf("  %s\n", line)
				}
			}
		}
	}
}