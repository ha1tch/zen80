package z80

import "testing"

// CALL/RET: verify push byte order, SP updates, and cycles.
func TestCALL_RET_StackOrderAndCycles(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SP = 0xFFFE

	// Place a CALL 1234h, then a RET at 1234h
	loadProgram(cpu, mem, 0x0000, 0xCD, 0x34, 0x12) // CALL 1234
	mustStep(t, cpu) // CALL
	// After CALL: PC=1234, SP=FFFC, memory at FFFC..FFFD contains return address 0003 (low, high)
	if cpu.PC != 0x1234 {
		t.Fatalf("CALL did not set PC to 1234h; got %04X", cpu.PC)
	}
	if cpu.SP != 0xFFFC {
		t.Fatalf("CALL wrong SP; got %04X want FFFC", cpu.SP)
	}
	if mem.Read(0xFFFC) != 0x03 || mem.Read(0xFFFD) != 0x00 {
		t.Fatalf("CALL pushed wrong bytes: [%02X %02X] want [03 00]", mem.Read(0xFFFC), mem.Read(0xFFFD))
	}

	// Write a RET at 1234h and execute it
	mem.Write(0x1234, 0xC9) // RET
	c := mustStep(t, cpu)
	if c != 10 {
		t.Fatalf("RET cycles=%d want 10", c)
	}
	if cpu.PC != 0x0003 {
		t.Fatalf("RET did not restore PC to 0003h; got %04X", cpu.PC)
	}
	if cpu.SP != 0xFFFE {
		t.Fatalf("RET did not restore SP; got %04X want FFFE", cpu.SP)
	}
}
