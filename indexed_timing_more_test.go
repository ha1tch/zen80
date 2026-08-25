package z80

import "testing"

// Extra coverage for IX/IY (HL-replacement) timings:
// - RES/SET on (IX+d)/(IY+d) should take 23 cycles via DDCB/FDCB.
func TestDDCB_SET_RES_TimingAndWriteback(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetIX(0x3000)
	mem.Write(0x3005, 0x00)

	// DDCB 05 C6 = SET 0,(IX+5)  (opcode C6 => SET 0,(HL) in CB space)
	loadProgram(cpu, mem, 0x0000, 0xDD, 0xCB, 0x05, 0xC6)
	c1 := mustStep(t, cpu)
	assertEq(t, mem.Read(0x3005)&0x01, uint8(1), "SET 0,(IX+5)")
	assertEq(t, c1, 23, "cycles for DDCB SET on (IX+d)")

	// DDCB 05 86 = RES 0,(IX+5)
	loadProgram(cpu, mem, cpu.PC, 0xDD, 0xCB, 0x05, 0x86)
	c2 := mustStep(t, cpu)
	assertEq(t, mem.Read(0x3005)&0x01, uint8(0), "RES 0,(IX+5)")
	assertEq(t, c2, 23, "cycles for DDCB RES on (IX+d)")
}
