package z80

import "testing"

// BIT n,(IX+d)/(IY+d): X/Y must come from WZ high byte, cycles 20 for BIT on indexed (no write-back).
func TestBIT_IXIY_Displacement_FlagsAndTiming(t *testing.T) {
	// IX case
	cpu, mem, _ := testCPU()
	cpu.SetIX(0x3000)
	mem.Write(0x3005, 0x80) // bit 7 set
	loadProgram(cpu, mem, 0x0000, 0xDD, 0xCB, 0x05, 0x7E) // BIT 7,(IX+5)
	c := mustStep(t, cpu)
	// Z=0, S=1 for bit7 set
	assertFlag(t, cpu, FlagZ, false, "BIT Z")
	assertFlag(t, cpu, FlagS, true, "BIT S for bit7")
	// X/Y from WZ high byte
	wzhi := uint8(cpu.WZ >> 8)
	assertFlag(t, cpu, FlagX, (wzhi&FlagX)!=0, "X from WZ high (IX)")
	assertFlag(t, cpu, FlagY, (wzhi&FlagY)!=0, "Y from WZ high (IX)")
	assertEq(t, c, 20, "cycles for DDCB BIT (IX+d) should be 20")

	// IY case
	cpu, mem, _ = testCPU()
	cpu.SetIY(0x4000)
	mem.Write(0x4002, 0x01) // bit 0 set
	loadProgram(cpu, mem, 0x0000, 0xFD, 0xCB, 0x02, 0x46) // BIT 0,(IY+2)
	c = mustStep(t, cpu)
	assertFlag(t, cpu, FlagZ, false, "BIT Z (IY)")
	wzhi = uint8(cpu.WZ >> 8)
	assertFlag(t, cpu, FlagX, (wzhi&FlagX)!=0, "X from WZ high (IY)")
	assertFlag(t, cpu, FlagY, (wzhi&FlagY)!=0, "Y from WZ high (IY)")
	assertEq(t, c, 20, "cycles for FDCB BIT (IY+d) should be 20")
}
