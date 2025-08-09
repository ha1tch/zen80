package z80

import "testing"

// Test LD r,(IX+d) and LD (IX+d),r timing and behavior.
func TestIndexedLoadsIX(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetIX(0x3000)
	mem.Write(0x3005, 0xAB)

	// DD 46 05 = LD B,(IX+5)
	loadProgram(cpu, mem, 0x0000, 0xDD, 0x46, 0x05)
	c := mustStep(t, cpu)
	assertEq(t, cpu.B, uint8(0xAB), "LD B,(IX+5)")
	assertEq(t, c, 19, "cycles for LD r,(IX+d)")

	// DD 70 05 = LD (IX+5),B
	loadProgram(cpu, mem, cpu.PC, 0xDD, 0x70, 0x05)
	c = mustStep(t, cpu)
	assertEq(t, mem.Read(0x3005), cpu.B, "LD (IX+5),B")
	assertEq(t, c, 19, "cycles for LD (IX+d),r")
}

// Undocumented IXH/IXL access: LD IXH,n; LD IXL,n; ADD A,IXH; XOR IXL
func TestIXHIXL_Undocumented(t *testing.T) {
	cpu, mem, _ := testCPU()
	// DD 26 12 = LD IXH,12; DD 2E 34 = LD IXL,34
	// DD 84 = ADD A,IXH (ADD A,H with DD prefix)
	// DD AE = XOR (HL)?? No, for XOR r it's 0xAE for (HL); use DD A5 = AND L ?
	// We'll do: LD A,0x01; DD 84 (ADD A,IXH) -> 0x13; DD B5 isn't valid; use DD A5 for AND IXL via "AND L" with DD -> IXL.
	loadProgram(cpu, mem, 0x0000,
		0xDD, 0x26, 0x12,
		0xDD, 0x2E, 0x34,
		0x3E, 0x01,
		0xDD, 0x84,       // ADD A,IXH
		0xDD, 0xA5,       // AND IXL (AND L with DD prefix)
	)
	mustStep(t, cpu) // LD IXH,12
	mustStep(t, cpu) // LD IXL,34
	mustStep(t, cpu) // LD A,01
	mustStep(t, cpu) // ADD A,IXH -> 0x13
	assertEq(t, cpu.A, uint8(0x13), "ADD A,IXH")
	mustStep(t, cpu) // AND IXL (0x34) -> 0x10
	assertEq(t, cpu.A, uint8(0x10), "AND IXL")
	// XY flags come from A after logical ops per implementation
	assertFlag(t, cpu, FlagX, (cpu.A&FlagX)!=0, "X from A after AND")
	assertFlag(t, cpu, FlagY, (cpu.A&FlagY)!=0, "Y from A after AND")
}

// IY mirrors IX tests.
func TestIndexedLoadsIY(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetIY(0x4000)
	mem.Write(0x4002, 0x55)

	// FD 4E 02 = LD C,(IY+2)
	loadProgram(cpu, mem, 0x0000, 0xFD, 0x4E, 0x02)
	c := mustStep(t, cpu)
	assertEq(t, cpu.C, uint8(0x55), "LD C,(IY+2)")
	assertEq(t, c, 19, "cycles for LD r,(IY+d)")
}
