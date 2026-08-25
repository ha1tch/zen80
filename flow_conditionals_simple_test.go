package z80

import "testing"

// Basic conditional flow timing: JR cc, RET cc, CALL cc
func TestJRcc_RETcc_CALLcc_Timing_Basics(t *testing.T) {
	cpu, mem, _ := testCPU()

	// Make a small program space
	// OR A (keeps Z=0), then JR Z,+2 (not taken), then XOR A (Z=1), JR NZ,+2 (not taken), JR Z,+2 (taken)
	loadProgram(cpu, mem, 0x0000,
		0xB7,             // OR A (A starts FF; Z=0)
		0x28, 0x02,       // JR Z, +2   -> not taken (7)
		0xAF,             // XOR A -> A=0 Z=1
		0x20, 0x02,       // JR NZ, +2  -> not taken now (7)
		0x28, 0x02,       // JR Z, +2   -> taken (12)
		0x00, 0x00,
	)
	mustStep(t, cpu)
	assertEq(t, mustStep(t, cpu), 7, "JR Z not taken")
	mustStep(t, cpu)
	assertEq(t, mustStep(t, cpu), 7, "JR NZ not taken")
	assertEq(t, mustStep(t, cpu), 12, "JR Z taken")

	// RET cc: make a simple CALL, then set flags so condition is false/true
	cpu, mem, _ = testCPU()
	cpu.SP = 0xFFFE
	// CALL next; place RET C (D8) and RET NC (D0) in two spots and test both timings
	loadProgram(cpu, mem, 0x0000, 0xCD, 0x06, 0x00) // CALL 0006
	mem.Write(0x0006, 0xD8) // RET C
	cpu.setFlag(FlagC, false)
	mustStep(t, cpu) // CALL
	// RET C (not taken): 5 cycles
	assertEq(t, mustStep(t, cpu), 5, "RET C not taken")

	// Put RET NC; set C so taken path triggers
	loadProgram(cpu, mem, cpu.PC, 0xCD, 0x06, 0x00)
	mem.Write(0x0006, 0xD0) // RET NC
	cpu.setFlag(FlagC, false)
	pcBefore := cpu.PC
	mustStep(t, cpu) // CALL
	// RET NC (taken): 11 cycles
	c := mustStep(t, cpu)
	assertEq(t, c, 11, "RET NC taken")
	assertEq(t, cpu.PC, pcBefore+3, "Returned to next instruction after CALL")
}

func TestDJNZ_Taken(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000,
		0x06, 0x02, // LD B,2
		0x10, 0x02, // DJNZ +2 (B->1, taken)
	)
	mustStep(t, cpu) // LD B,2
	assertEq(t, mustStep(t, cpu), 13, "DJNZ taken")
}

func TestDJNZ_NotTaken(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000,
		0x06, 0x01, // LD B,1
		0x10, 0x02, // DJNZ +2 (B->0, not taken)
	)
	mustStep(t, cpu) // LD B,1
	assertEq(t, mustStep(t, cpu), 8, "DJNZ not taken")
}
