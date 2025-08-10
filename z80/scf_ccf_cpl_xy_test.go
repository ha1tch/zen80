package z80

import "testing"

// Ensure SCF, CCF, CPL set X/Y from A (regression locks).
func TestSCF_CCF_CPL_XY_FromA(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Make A have both X/Y set -> e.g., 0x28
	loadProgram(cpu, mem, 0x0000, 0x3E, 0x28, 0x37, 0x3F, 0x2F) // LD A,28 ; SCF ; CCF ; CPL
	mustStep(t, cpu) // LD
	mustStep(t, cpu) // SCF
	assertFlag(t, cpu, FlagX, true, "SCF X from A")
	assertFlag(t, cpu, FlagY, true, "SCF Y from A")
	mustStep(t, cpu) // CCF
	assertFlag(t, cpu, FlagX, true, "CCF X from A")
	assertFlag(t, cpu, FlagY, true, "CCF Y from A")
	mustStep(t, cpu) // CPL (A becomes ^A)
	assertFlag(t, cpu, FlagX, (cpu.A&FlagX)!=0, "CPL X from new A")
	assertFlag(t, cpu, FlagY, (cpu.A&FlagY)!=0, "CPL Y from new A")
}
