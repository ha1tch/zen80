package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ha1tch/zen80/system"
)

func main() {
	fmt.Println("=== ZX Spectrum Timing Test ===")
	
	// Create a Spectrum system
	spec := system.NewSpectrum()
	
	// Load a simple test ROM (just NOPs and a jump loop)
	testROM := make([]uint8, 16384)
	// Simple infinite loop at address 0:
	// 0000: NOP (4 cycles)
	// 0001: NOP (4 cycles)  
	// 0002: JP 0000 (10 cycles)
	// Total: 18 cycles per loop
	testROM[0] = 0x00  // NOP
	testROM[1] = 0x00  // NOP
	testROM[2] = 0xC3  // JP nn
	testROM[3] = 0x00  // Low byte of address
	testROM[4] = 0x00  // High byte of address
	
	err := spec.LoadROM(testROM)
	if err != nil {
		log.Fatal(err)
	}
	
	// Reset and run for exactly 1 second
	spec.Reset()
	
	// Run in a goroutine
	done := make(chan bool)
	go func() {
		startTime := time.Now()
		frameCount := 0
		
		for time.Since(startTime) < time.Second {
			spec.RunFrame()
			frameCount++
		}
		
		done <- true
		fmt.Printf("Frames executed in 1 second: %d (target: 50)\n", frameCount)
	}()
	
	// Wait for completion
	<-done
	
	// Get statistics
	stats := spec.GetStatistics()
	fmt.Printf("\n=== Timing Statistics ===\n")
	fmt.Printf("Target CPU frequency: %.0f Hz\n", stats.TargetHz)
	fmt.Printf("Actual CPU frequency: %.0f Hz\n", stats.ActualHz)
	fmt.Printf("Accuracy: %.2f%%\n", (stats.ActualHz/stats.TargetHz)*100)
	fmt.Printf("Frame rate: %.2f FPS (target: 50)\n", stats.FrameRate)
	fmt.Printf("Total cycles: %d\n", stats.TotalCycles)
	
	// Test different speed multipliers
	fmt.Println("\n=== Speed Multiplier Test ===")
	testSpeeds := []float64{0.5, 1.0, 2.0}
	
	for _, speed := range testSpeeds {
		spec.Reset()
		spec.SetSpeed(speed)
		
		startTime := time.Now()
		startCycles := spec.GetStatistics().TotalCycles
		
		// Run for 100 frames
		for i := 0; i < 100; i++ {
			spec.RunFrame()
		}
		
		elapsed := time.Since(startTime)
		endCycles := spec.GetStatistics().TotalCycles
		cyclesExecuted := endCycles - startCycles
		
		expectedTime := (2.0 / speed) // 100 frames at 50 FPS = 2 seconds
		fmt.Printf("\nSpeed %.1fx:\n", speed)
		fmt.Printf("  Expected time: %.2f seconds\n", expectedTime)
		fmt.Printf("  Actual time: %.2f seconds\n", elapsed.Seconds())
		fmt.Printf("  Cycles executed: %d\n", cyclesExecuted)
		fmt.Printf("  Effective frequency: %.0f Hz\n", 
			float64(cyclesExecuted)/elapsed.Seconds())
	}
	
	// Test instruction cycle accuracy
	fmt.Println("\n=== Instruction Cycle Accuracy Test ===")
	testInstructionTiming(spec)
}

func testInstructionTiming(spec *system.Spectrum) {
	// Test patterns with known cycle counts
	testPatterns := []struct {
		name     string
		code     []uint8
		cycles   int
		desc     string
	}{
		{
			"NOP sequence",
			[]uint8{0x00, 0x00, 0x00, 0x00, 0x76}, // 4 NOPs + HALT
			20, // 4*4 + 4 = 20
			"Basic instruction timing",
		},
		{
			"Register loads",
			[]uint8{
				0x06, 0x42,  // LD B,42h (7 cycles)
				0x0E, 0x84,  // LD C,84h (7 cycles)
				0x78,        // LD A,B (4 cycles)
				0x76,        // HALT (4 cycles)
			},
			22,
			"Immediate and register loads",
		},
		{
			"16-bit operations",
			[]uint8{
				0x01, 0x34, 0x12,  // LD BC,1234h (10 cycles)
				0x11, 0x78, 0x56,  // LD DE,5678h (10 cycles)
				0x09,              // ADD HL,BC (11 cycles)
				0x76,              // HALT (4 cycles)
			},
			35,
			"16-bit load and arithmetic",
		},
		{
			"Stack operations",
			[]uint8{
				0x31, 0x00, 0x60,  // LD SP,6000h (10 cycles)
				0x01, 0x34, 0x12,  // LD BC,1234h (10 cycles)
				0xC5,              // PUSH BC (11 cycles)
				0xC1,              // POP BC (10 cycles)
				0x76,              // HALT (4 cycles)
			},
			45,
			"Stack push/pop",
		},
		{
			"Conditional jump taken",
			[]uint8{
				0x3E, 0x00,        // LD A,0 (7 cycles)
				0x28, 0x02,        // JR Z,+2 (12 cycles - taken)
				0x00, 0x00,        // NOPs (skipped)
				0x76,              // HALT (4 cycles)
			},
			23,
			"Conditional relative jump",
		},
	}
	
	for _, test := range testPatterns {
		// Load test code at address 0x8000
		spec.Reset()
		spec.LoadSnapshot(0x8000, test.code)
		spec.cpu.PC = 0x8000
		
		// Execute until HALT
		totalCycles := 0
		for !spec.cpu.Halted && totalCycles < 1000 {
			cycles := spec.cpu.Step()
			totalCycles += cycles
		}
		
		fmt.Printf("\n%s:\n", test.name)
		fmt.Printf("  Description: %s\n", test.desc)
		fmt.Printf("  Expected cycles: %d\n", test.cycles)
		fmt.Printf("  Actual cycles: %d\n", totalCycles)
		if totalCycles == test.cycles {
			fmt.Printf("  Result: ✓ PASS\n")
		} else {
			fmt.Printf("  Result: ✗ FAIL (difference: %+d)\n", 
				totalCycles - test.cycles)
		}
	}
}