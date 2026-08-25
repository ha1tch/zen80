package z80

import "testing"

// LDI/LDDR/LDIR correctness: registers, memory effects, PV from BC!=0, and timing totals.
func TestLDI_Registers_Flags_Memory(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetHL(0x4000); cpu.SetDE(0x4100); cpu.SetBC(3)
	mem.Write(0x4000, 0x11)
	loadProgram(cpu, mem, 0x0000, 0xED, 0xA0) // LDI
	c := mustStep(t, cpu)
	assertEq(t, c, 16, "LDI cycles")
	assertEq(t, mem.Read(0x4100), uint8(0x11), "LDI moved byte")
	assertEq(t, cpu.HL(), uint16(0x4001), "HL++")
	assertEq(t, cpu.DE(), uint16(0x4101), "DE++")
	assertEq(t, cpu.BC(), uint16(2), "BC--")
	assertFlag(t, cpu, FlagN, false, "N cleared")
	assertFlag(t, cpu, FlagH, false, "H cleared")
	assertFlag(t, cpu, FlagPV, true, "PV mirrors BC!=0")

	// LDIR for 2 bytes: expect 21 + 16 = 37 cycles and final regs.
	cpu, mem, _ = testCPU()
	cpu.SetHL(0x5000); cpu.SetDE(0x6000); cpu.SetBC(2)
	mem.Write(0x5000, 0xAA)
	mem.Write(0x5001, 0xBB)
	loadProgram(cpu, mem, 0x0000, 0xED, 0xB0) // LDIR
	total := 0
	for {
		total += mustStep(t, cpu)
		if cpu.BC() == 0 {
			break
		}
	}
	assertEq(t, total, 37, "LDIR total cycles for 2 bytes")
	assertEq(t, mem.Read(0x6000), uint8(0xAA), "LDIR first byte")
	assertEq(t, mem.Read(0x6001), uint8(0xBB), "LDIR second byte")
	assertEq(t, cpu.HL(), uint16(0x5002), "HL end")
	assertEq(t, cpu.DE(), uint16(0x6002), "DE end")
}
