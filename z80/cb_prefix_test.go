package z80

import "testing"

// BIT n,(HL) must set X/Y from WZ high byte (implementation note in prefix_cb.go).
func TestBIT_HL_XYFromWZHigh(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Put value at 0x4000, set HL=0x4000, then CB 7E = BIT 7,(HL)
	mem.Write(0x4000, 0x80) // bit 7 set
	cpu.SetHL(0x4000)
	loadProgram(cpu, mem, 0x0000, 0xCB, 0x7E)
	c := mustStep(t, cpu)

	// For BIT 7,(HL), Z=0, S=1, PV mirrors Z, H=1, N=0.
	assertFlag(t, cpu, FlagZ, false, "BIT Z")
	// In our implementation, executeCB overrides X/Y with WZ high.
	wzhi := uint8(cpu.WZ >> 8)
	assertFlag(t, cpu, FlagX, (wzhi&FlagX)!=0, "X from WZ high")
	assertFlag(t, cpu, FlagY, (wzhi&FlagY)!=0, "Y from WZ high")

	assertEq(t, c, 12, "cycles for BIT n,(HL)")
}

// RLC (HL): verify write-back and timing 15 cycles.
func TestRLC_HL_WriteBackAndTiming(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetHL(0x2000)
	mem.Write(0x2000, 0x81) // 1000 0001 -> RLC -> 0000 0011, C=1
	loadProgram(cpu, mem, 0x0000, 0xCB, 0x06) // RLC (HL)
	c := mustStep(t, cpu)
	assertEq(t, mem.Read(0x2000), uint8(0x03), "RLC(HL) write-back")
	assertFlag(t, cpu, FlagC, true, "C set after RLC")
	assertEq(t, c, 15, "cycles for RLC (HL)")
}
