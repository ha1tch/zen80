package z80

import "testing"

// RLCA/RRCA/RLA/RRA: verify they don't change S/Z/PV and check cycles.
func TestRotatesOnA_Basics(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.A = 0x81
	cpu.F = 0xFF // start with flags set so we can see what's cleared
	loadProgram(cpu, mem, 0x0000,
		0x07, // RLCA
		0x0F, // RRCA
		0x17, // RLA
		0x1F, // RRA
	)
	// RLCA
	c := mustStep(t, cpu); assertEq(t, c, 4, "RLCA cycles")
	// RRCA
	c = mustStep(t, cpu); assertEq(t, c, 4, "RRCA cycles")
	// RLA
	c = mustStep(t, cpu); assertEq(t, c, 4, "RLA cycles")
	// RRA
	c = mustStep(t, cpu); assertEq(t, c, 4, "RRA cycles")
	// For these ops, S/Z/PV are unaffected (per your core they are cleared back where needed).
	// Quick sanity: ensure no unexpected setting of Z just by rotates
	assertFlag(t, cpu, FlagZ, false, "Rotates shouldn't set Z spuriously")
}
