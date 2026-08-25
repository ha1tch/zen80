package z80

import "testing"

// EX/EXX/LD SP,HL and EX (SP),HL basics.
func TestEX_EXX_SP_HL_Basics(t *testing.T) {
	cpu, mem, _ := testCPU()

	// EX AF,AF'
	cpu.A, cpu.F = 0x12, 0x34
	cpu.A_, cpu.F_ = 0xAB, 0xCD
	loadProgram(cpu, mem, 0x0000, 0x08) // EX AF,AF'
	mustStep(t, cpu)
	if cpu.A != 0xAB || cpu.F != 0xCD || cpu.A_ != 0x12 || cpu.F_ != 0x34 {
		t.Fatalf("EX AF,AF' failed")
	}

	// EXX
	cpu.SetBC(0x1111); cpu.SetDE(0x2222); cpu.SetHL(0x3333)
	cpu.SetAF(0x0000) // ensure flags not impacted by EXX
	cpu.B_, cpu.C_, cpu.D_, cpu.E_, cpu.H_, cpu.L_ = 0xAA,0xBB,0xCC,0xDD,0xEE,0xFF
	loadProgram(cpu, mem, cpu.PC, 0xD9) // EXX
	mustStep(t, cpu)
	if cpu.BC() != 0xAABB || cpu.DE() != 0xCCDD || cpu.HL() != 0xEEFF {
		t.Fatalf("EXX failed to swap alt sets")
	}

	// LD SP,HL
	cpu.SetHL(0x4321)
	loadProgram(cpu, mem, cpu.PC, 0xF9) // LD SP,HL
	c := mustStep(t, cpu)
	assertEq(t, c, 6, "LD SP,HL cycles")
	assertEq(t, cpu.SP, uint16(0x4321), "LD SP,HL moved value")

	// EX (SP),HL
	cpu.SP = 0x8000
	mem.Write(0x8000, 0x78) // low
	mem.Write(0x8001, 0x56) // high -> word 0x5678
	cpu.SetHL(0x9ABC)
	loadProgram(cpu, mem, cpu.PC, 0xE3) // EX (SP),HL
	c = mustStep(t, cpu)
	assertEq(t, c, 19, "EX (SP),HL cycles")
	// HL should now be 0x5678; memory should now hold 0x9ABC
	if cpu.HL() != 0x5678 || mem.Read(0x8000) != 0xBC || mem.Read(0x8001) != 0x9A {
		t.Fatalf("EX (SP),HL failed: HL=%04X mem=[%02X %02X]", cpu.HL(), mem.Read(0x8000), mem.Read(0x8001))
	}
}
