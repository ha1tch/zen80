package z80

import "testing"

// ED 80..9F (mostly undefined) act as NOP with 8 cycles per the implementation.
// We probe one byte to cement the contract.
func TestED_UndefinedActsAsNOP8Cycles(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Use ED 80
	loadProgram(cpu, mem, 0x0000, 0xED, 0x80)
	pc := cpu.PC
	c := mustStep(t, cpu)
	assertEq(t, c, 8, "undefined ED opcode should be 8 cycles")
	assertEq(t, cpu.PC, pc+2, "PC advanced over ED xx")
}
