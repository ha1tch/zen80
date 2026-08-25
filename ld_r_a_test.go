package z80

import "testing"

// Clarify semantics of LD R,A (ED 4F): R should become exactly A (all 8 bits).
// This guards against accidental attempts to preserve R7 here.
func TestED_LD_R_A_CopiesAllBits(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Arrange: set A with a top bit pattern and verify R=A after ED 4F.
	loadProgram(cpu, mem, 0x0000,
		0x3E, 0x81,       // LD A,81h
		0xED, 0x4F,       // LD R,A
	)
	cpu.R = 0x00
	mustStep(t, cpu) // LD A,81
	mustStep(t, cpu) // LD R,A
	if cpu.R != 0x81 {
		t.Errorf("LD R,A: got R=%02X want 81", cpu.R)
	}
}
