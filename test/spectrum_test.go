package test

import (
	"testing"
	"time"

	"github.com/ha1tch/zen80/system"
)

// TestSpectrumMemoryBanking tests memory layout
func TestSpectrumMemoryBanking(t *testing.T) {
	spec := system.NewSpectrum()
	
	// Load a simple ROM
	rom := make([]uint8, 16384)
	rom[0] = 0x00  // NOP at address 0
	rom[1] = 0x76  // HALT
	spec.LoadROM(rom)
	
	// Test ROM is read-only
	spec.Memory.Write(0x0000, 0xFF)
	if spec.Memory.Read(0x0000) != 0x00 {
		t.Error("ROM should be read-only")
	}
	
	// Test RAM is writable
	spec.Memory.Write(0x4000, 0x42)
	if spec.Memory.Read(0x4000) != 0x42 {
		t.Error("RAM should be writable")
	}
	
	// Test screen memory area
	spec.Memory.Write(0x4000, 0xFF) // Pixel data
	spec.Memory.Write(0x5800, 0x47) // Attribute data
	
	if spec.Memory.Read(0x4000) != 0xFF {
		t.Error("Screen pixel memory should be accessible")
	}
	if spec.Memory.Read(0x5800) != 0x47 {
		t.Error("Screen attribute memory should be accessible")
	}
}

// TestSpectrumIO tests I/O port behavior
func TestSpectrumIO(t *testing.T) {
	spec := system.NewSpectrum()
	
	// Test ULA port (0xFE)
	spec.IO.Out(0xFE, 0x05) // Set border to cyan (5)
	if spec.Border != 0x05 {
		t.Errorf("Border color = %02X, want 05", spec.Border)
	}
	
	// Test keyboard reading
	// Simulate pressing 'A' (CAPS SHIFT + A)
	spec.PressKey(0, 0)  // CAPS SHIFT (row 0, bit 0)
	spec.PressKey(1, 0)  // A (row 1, bit 0)
	
	// Read keyboard port
	val := spec.IO.In(0xFEFE) // Row 0 (CAPS SHIFT row)
	if val&0x01 != 0 {
		t.Error("CAPS SHIFT should be pressed (bit 0 clear)")
	}
	
	val = spec.IO.In(0xFDFE) // Row 1 (A-G row)
	if val&0x01 != 0 {
		t.Error("A key should be pressed (bit 0 clear)")
	}
	
	// Release keys
	spec.ReleaseKey(0, 0)
	spec.ReleaseKey(1, 0)
	
	val = spec.IO.In(0xFEFE)
	if val&0x01 == 0 {
		t.Error("CAPS SHIFT should be released (bit 0 set)")
	}
}

// TestSpectrumInterruptTiming tests 50Hz interrupt generation
func TestSpectrumInterruptTiming(t *testing.T) {
	spec := system.NewSpectrum()
	
	// Load a simple interrupt counter program
	program := []uint8{
		0x31, 0x00, 0x80,  // LD SP,8000h
		0xED, 0x56,        // IM 1
		0xFB,              // EI
		0x3E, 0x00,        // LD A,0
		0x32, 0x00, 0x60,  // LD (6000h),A - interrupt counter
		// Main loop at 0x0009:
		0x18, 0xFE,        // JR -2 (infinite loop)
		
		// Interrupt handler at 0x0038:
	}
	
	// ISR at 0x0038 - increment counter
	isr := []uint8{
		0x3A, 0x00, 0x60,  // LD A,(6000h)
		0x3C,              // INC A
		0x32, 0x00, 0x60,  // LD (6000h),A
		0xFB,              // EI
		0xED, 0x4D,        // RETI
	}
	
	spec.LoadSnapshot(0x0000, program)
	spec.LoadSnapshot(0x0038, isr)
	spec.Reset()
	spec.CPU.PC = 0x0000
	
	// Run for approximately 10 frames (200ms)
	frameCount := 0
	for frameCount < 10 {
		spec.RunFrame()
		frameCount++
	}
	
	// Check interrupt counter
	counter := spec.Memory.Read(0x6000)
	if counter < 8 || counter > 12 {
		t.Errorf("Interrupt counter = %d, expected ~10 (one per frame)", counter)
	}
}

// TestSpectrumTiming tests CPU speed accuracy
func TestSpectrumTiming(t *testing.T) {
	spec := system.NewSpectrum()
	
	// Load a NOP loop
	program := []uint8{
		0x00,        // NOP (4 cycles)
		0xC3, 0x00, 0x00,  // JP 0000 (10 cycles)
		// Total: 14 cycles per loop
	}
	
	spec.LoadSnapshot(0x0000, program)
	spec.Reset()
	
	// Run for exactly 100 frames (2 seconds at 50 FPS)
	startCycles := spec.CPU.Cycles
	for i := 0; i < 100; i++ {
		spec.RunFrame()
	}
	endCycles := spec.CPU.Cycles
	
	cyclesExecuted := endCycles - startCycles
	expectedCycles := uint64(7000000) // 3.5MHz * 2 seconds
	
	// Allow 5% tolerance
	minCycles := uint64(float64(expectedCycles) * 0.95)
	maxCycles := uint64(float64(expectedCycles) * 1.05)
	
	if cyclesExecuted < minCycles || cyclesExecuted > maxCycles {
		t.Errorf("Executed %d cycles, expected ~%d (±5%%)", cyclesExecuted, expectedCycles)
	}
}

// TestSpectrumContention tests memory contention timing
func TestSpectrumContention(t *testing.T) {
	// Note: Our simple emulator doesn't implement contention,
	// but this test shows where it would be tested
	t.Skip("Memory contention not implemented")
	
	spec := system.NewSpectrum()
	
	// Program that accesses contended memory
	program := []uint8{
		0x21, 0x00, 0x40,  // LD HL,4000h (screen memory)
		0x7E,              // LD A,(HL) - contended access
		0x23,              // INC HL
		0x7E,              // LD A,(HL) - contended access
		0x76,              // HALT
	}
	
	spec.LoadSnapshot(0x0000, program)
	spec.Reset()
	
	startCycles := spec.CPU.Cycles
	for !spec.CPU.Halted {
		spec.CPU.Step()
	}
	endCycles := spec.CPU.Cycles
	
	normalCycles := 7 + 7 + 6 + 7 + 4 // Without contention
	actualCycles := int(endCycles - startCycles)
	
	// With contention, this should take more cycles
	if actualCycles <= normalCycles {
		t.Errorf("Expected contention delays, got %d cycles (normal: %d)", actualCycles, normalCycles)
	}
}

// TestSpectrumRealTimeSync tests real-time synchronization
func TestSpectrumRealTimeSync(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-time test in short mode")
	}
	
	spec := system.NewSpectrum()
	
	// Simple infinite loop
	program := []uint8{
		0x18, 0xFE,  // JR -2
	}
	
	spec.LoadSnapshot(0x0000, program)
	spec.Reset()
	
	// Run for exactly 1 second
	startTime := time.Now()
	frameCount := 0
	
	for time.Since(startTime) < time.Second {
		spec.RunFrame()
		frameCount++
	}
	
	elapsed := time.Since(startTime)
	
	// Should be close to 50 frames
	if frameCount < 48 || frameCount > 52 {
		t.Errorf("Frame count = %d in %.2fs, expected ~50", frameCount, elapsed.Seconds())
	}
	
	// Check CPU frequency
	cycles := spec.CPU.Cycles
	effectiveHz := float64(cycles) / elapsed.Seconds()
	
	if effectiveHz < 3325000 || effectiveHz > 3675000 { // ±5%
		t.Errorf("Effective CPU speed = %.0f Hz, expected ~3,500,000 Hz", effectiveHz)
	}
}

// TestSpectrumSpeedControl tests speed multiplier functionality
func TestSpectrumSpeedControl(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping speed control test in short mode")
	}
	
	spec := system.NewSpectrum()
	
	program := []uint8{
		0x00,              // NOP
		0xC3, 0x00, 0x00,  // JP 0000
	}
	
	spec.LoadSnapshot(0x0000, program)
	
	speeds := []float64{0.5, 1.0, 2.0}
	
	for _, speed := range speeds {
		spec.Reset()
		spec.SetSpeed(speed)
		
		startTime := time.Now()
		startCycles := spec.CPU.Cycles
		
		// Run for 50 frames
		for i := 0; i < 50; i++ {
			spec.RunFrame()
		}
		
		elapsed := time.Since(startTime)
		cyclesExecuted := spec.CPU.Cycles - startCycles
		
		expectedTime := 1.0 / speed // 50 frames at 50 FPS = 1 second at 1x speed
		actualTime := elapsed.Seconds()
		
		// Allow 10% tolerance for timing
		minTime := expectedTime * 0.9
		maxTime := expectedTime * 1.1
		
		if actualTime < minTime || actualTime > maxTime {
			t.Errorf("Speed %.1fx: took %.2fs, expected %.2fs (±10%%)", 
				speed, actualTime, expectedTime)
		}
		
		// Cycles should be constant regardless of speed
		expectedCycles := uint64(3500000) // 1 second worth at normal speed
		if cyclesExecuted < expectedCycles*95/100 || cyclesExecuted > expectedCycles*105/100 {
			t.Errorf("Speed %.1fx: executed %d cycles, expected ~%d", 
				speed, cyclesExecuted, expectedCycles)
		}
	}
}

// BenchmarkSpectrumFrame benchmarks frame execution
func BenchmarkSpectrumFrame(b *testing.B) {
	spec := system.NewSpectrum()
	
	// Typical game loop
	program := []uint8{
		0x3E, 0x00,        // LD A,0
		0xD3, 0xFE,        // OUT (FE),A - set border
		0x3C,              // INC A
		0xFE, 0x08,        // CP 8
		0x20, 0x02,        // JR NZ,+2
		0x3E, 0x00,        // LD A,0
		0x18, 0xF4,        // JR -12
	}
	
	spec.LoadSnapshot(0x0000, program)
	spec.Reset()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spec.RunFrame()
	}
	
	framesPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(framesPerSec, "frames/sec")
}

// TestScreenMemoryMapping tests the Spectrum screen layout
func TestScreenMemoryMapping(t *testing.T) {
	spec := system.NewSpectrum()
	
	// Test pixel data area (0x4000-0x57FF)
	// Write to first byte of screen
	spec.Memory.Write(0x4000, 0xFF)
	if spec.Memory.Read(0x4000) != 0xFF {
		t.Error("Screen pixel memory not accessible")
	}
	
	// Test attribute area (0x5800-0x5AFF)
	// Set first character to bright white on black
	spec.Memory.Write(0x5800, 0x47) // Bright, white ink, black paper
	if spec.Memory.Read(0x5800) != 0x47 {
		t.Error("Screen attribute memory not accessible")
	}
	
	// Test that screen memory is in RAM (writable)
	for addr := uint16(0x4000); addr < 0x5B00; addr += 0x100 {
		spec.Memory.Write(addr, uint8(addr>>8))
		if spec.Memory.Read(addr) != uint8(addr>>8) {
			t.Errorf("Screen memory at %04X not writable", addr)
		}
	}
}

// TestKeyboardMatrix tests the Spectrum keyboard
func TestKeyboardMatrix(t *testing.T) {
	spec := system.NewSpectrum()
	
	// Spectrum keyboard matrix
	// Bit 0-4: Keys in that half-row
	// Port high byte selects row
	
	keys := []struct {
		row  uint8
		col  uint8
		port uint16
		key  string
	}{
		{0, 0, 0xFEFE, "CAPS SHIFT"},
		{0, 1, 0xFEFE, "Z"},
		{0, 2, 0xFEFE, "X"},
		{0, 3, 0xFEFE, "C"},
		{0, 4, 0xFEFE, "V"},
		{1, 0, 0xFDFE, "A"},
		{1, 1, 0xFDFE, "S"},
		{1, 2, 0xFDFE, "D"},
		{1, 3, 0xFDFE, "F"},
		{1, 4, 0xFDFE, "G"},
		{7, 0, 0x7FFE, "SPACE"},
		{7, 1, 0x7FFE, "SYMBOL SHIFT"},
		{7, 2, 0x7FFE, "M"},
		{7, 3, 0x7FFE, "N"},
		{7, 4, 0x7FFE, "B"},
	}
	
	for _, k := range keys {
		// Press key
		spec.PressKey(k.row, k.col)
		val := spec.IO.In(k.port)
		if val&(1<<k.col) != 0 {
			t.Errorf("%s key not detected when pressed", k.key)
		}
		
		// Release key
		spec.ReleaseKey(k.row, k.col)
		val = spec.IO.In(k.port)
		if val&(1<<k.col) == 0 {
			t.Errorf("%s key still detected after release", k.key)
		}
	}
}