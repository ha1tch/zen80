package z80

import "testing"

func TestJR_taken_vs_not_taken_cycles(t *testing.T) {
	// JR Z,d : when Z set -> taken 12 cycles; when not -> 7 cycles
	// We'll do: OR A (so Z=0) then JR Z,+2 (not taken); then XOR A (Z=1) then JR Z,+2 (taken)
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000,
		0xB7,              // OR A -> Z depends on A; initial A=FF (from New), so OR A keeps Z=0
		0x28, 0x02,        // JR Z,+2 (not taken)
		0xAF,              // XOR A -> A=0, Z=1
		0x28, 0x02,        // JR Z,+2 (taken)
		0x00, 0x00,        // padding
	)
	// OR A
	mustStep(t, cpu)
	// JR Z (not taken)
	c1 := mustStep(t, cpu)
	assertEq(t, c1, 7, "JR Z not taken cycles")
	// XOR A
	mustStep(t, cpu)
	// JR Z (taken)
	c2 := mustStep(t, cpu)
	assertEq(t, c2, 12, "JR Z taken cycles")
}

func TestHALTBehavior(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000, 0x76) // HALT
	c := mustStep(t, cpu)
	assertEq(t, c, 4, "HALT cycles first step")
	assertEq(t, cpu.Halted, true, "CPU halted")
	// Subsequent step should still be 4 cycles and not change PC
	pc := cpu.PC
	c2 := mustStep(t, cpu)
	assertEq(t, c2, 4, "HALT cycles subsequent step")
	assertEq(t, cpu.PC, pc, "PC stable while halted")
}

// WZ correctness for a few representative instructions.
func TestWZUpdates(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetBC(0x1234)
	cpu.A = 0x9A
	loadProgram(cpu, mem, 0x0000,
		0x02,       // LD (BC),A  [WZ=(A<<8)|((BC+1)&0xFF)]
		0x0A,       // LD A,(BC)  [WZ=BC+1]
	)

	mustStep(t, cpu)
	expected := (uint16(cpu.A) << 8) | ((cpu.BC() + 1) & 0x00FF)
	assertEq(t, cpu.WZ, expected, "WZ after LD (BC),A")

	mustStep(t, cpu)
	assertEq(t, cpu.WZ, cpu.BC()+1, "WZ after LD A,(BC)")
}
