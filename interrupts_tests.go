package z80

import "testing"

// EI delay: interrupt must not fire on the instruction immediately following EI.
func TestEI_Delay_BeforeInterruptAccept(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Program: EI; NOP; NOP (we'll be in IM1 so accept RST 38h)
	loadProgram(cpu, mem, 0x0000, 0xFB, 0x00, 0x00) // FB=EI
	cpu.IM = 1
	cpu.INT = true // interrupt line is asserted

	// EI executes; pendingEI set; interrupts checked before enabling => no service yet
	mustStep(t, cpu) // EI
	pc1 := cpu.PC
	mustStep(t, cpu) // NOP immediately after EI - still should not take INT
	if cpu.PC != pc1+1 {
		t.Fatalf("Interrupt fired too soon after EI; PC=%04X expected %04X", cpu.PC, pc1+1)
	}
	// Next step: now IFF1 is enabled and INT should be taken
	mustStep(t, cpu)
	if cpu.PC != 0x0038 {
		t.Fatalf("IM1 should vector to 0038h, PC=%04X", cpu.PC)
	}
}

// HALT exits on interrupt; cycles of the interrupt path are counted.
func TestHALT_Interrupted_ExitsAndVectors(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.IM = 1
	cpu.IFF1, cpu.IFF2 = true, true
	cpu.INT = true
	loadProgram(cpu, mem, 0x0000, 0x76) // HALT
	mustStep(t, cpu) // enter HALT
	if !cpu.Halted {
		t.Fatalf("CPU should be halted")
	}
	// Next step processes the interrupt and clears HALT
	c := mustStep(t, cpu)
	if cpu.Halted {
		t.Fatalf("CPU should exit HALT on interrupt")
	}
	if cpu.PC != 0x0038 {
		t.Fatalf("IM1 vector expected 0038h, got %04X", cpu.PC)
	}
	// Cycle count should match IM1 path (13)
	if c != 13 {
		t.Fatalf("Interrupt from HALT cycles got %d want 13", c)
	}
}
