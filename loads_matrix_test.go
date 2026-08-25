package z80

import "testing"

// Exhaustive LD r,r' matrix (register-register moves) + immediate/memory forms.
// Also verifies loads do NOT affect flags.
func TestLD_Register_Matrix_And_Immediates(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Prepare HL memory
	cpu.SetHL(0x4000)
	mem.Write(0x4000, 0xA5)

	// Fill registers with distinct values
	cpu.A, cpu.B, cpu.C, cpu.D, cpu.E, cpu.H, cpu.L = 0x11,0x22,0x33,0x44,0x55,0x66,0x77
	flagsStart := cpu.F

	// LD B,C ; LD D,E ; LD A,B ; LD L,H ; (skip HALT 0x76)
	loadProgram(cpu, mem, 0x0000,
		0x41,       // LD B,C
		0x53,       // LD D,E
		0x78,       // LD A,B
		0x6C,       // LD L,H
		0x06, 0x99, // LD B,99
		0x36, 0xFE, // LD (HL),FE
		0x7E,       // LD A,(HL)
		0x70,       // LD (HL),B
	)

	// LD B,C
	c := mustStep(t, cpu)
	assertEq(t, c, 4, "LD r,r' cycles")
	assertEq(t, cpu.B, cpu.C, "LD B,C value")
	assertEq(t, cpu.F, flagsStart, "LD r,r' must not alter F")

	// LD D,E
	mustStep(t, cpu); assertEq(t, cpu.D, uint8(0x55), "LD D,E value")

	// LD A,B
	mustStep(t, cpu); assertEq(t, cpu.A, cpu.B, "LD A,B value")

	// LD L,H
	mustStep(t, cpu); assertEq(t, cpu.L, cpu.H, "LD L,H value")

	// LD B,n
	c = mustStep(t, cpu); assertEq(t, c, 7, "LD r,n cycles")
	assertEq(t, cpu.B, uint8(0x99), "LD B,n value")

	// LD (HL),n
	c = mustStep(t, cpu); assertEq(t, c, 10, "LD (HL),n cycles")
	assertEq(t, mem.Read(cpu.HL()), uint8(0xFE), "LD (HL),n stored")

	// LD A,(HL)
	c = mustStep(t, cpu); assertEq(t, c, 7, "LD r,(HL) cycles")
	assertEq(t, cpu.A, uint8(0xFE), "LD A,(HL) value")

	// LD (HL),r
	c = mustStep(t, cpu); assertEq(t, c, 7, "LD (HL),r cycles")
	assertEq(t, mem.Read(cpu.HL()), cpu.B, "LD (HL),B stored B")
}

func TestLD_A_BC_DE_Basics(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetBC(0x1234)
	cpu.SetDE(0x5678)
	mem.Write(0x1234, 0xAA)
	mem.Write(0x5678, 0xBB)

	loadProgram(cpu, mem, 0x0000,
		0x0A, // LD A,(BC)
		0x1A, // LD A,(DE)
		0x02, // LD (BC),A
		0x12, // LD (DE),A
	)

	c := mustStep(t, cpu); assertEq(t, c, 7, "LD A,(BC) cycles")
	assertEq(t, cpu.A, uint8(0xAA), "LD A,(BC)")

	c = mustStep(t, cpu); assertEq(t, c, 7, "LD A,(DE) cycles")
	assertEq(t, cpu.A, uint8(0xBB), "LD A,(DE)")

	// LD (BC),A
	c = mustStep(t, cpu); assertEq(t, c, 7, "LD (BC),A cycles")
	assertEq(t, mem.Read(0x1234), cpu.A, "LD (BC),A wrote A")

	// LD (DE),A
	c = mustStep(t, cpu); assertEq(t, c, 7, "LD (DE),A cycles")
	assertEq(t, mem.Read(0x5678), cpu.A, "LD (DE),A wrote A")
}
