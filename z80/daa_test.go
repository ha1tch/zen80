package z80

import "testing"

// A focused set of DAA edge cases (after ADD and after SUB).
// These are well-known tricky corners and good regression targets.
func TestDAA_AfterAdd_LowerNibbleOverflow(t *testing.T) {
	cpu, mem, _ := testCPU()
	// A=09h; ADD A,01h => 0Ah; DAA => 10h, C=0, H adjusted, N=0
	loadProgram(cpu, mem, 0x0000,
		0x3E, 0x09,       // LD A,09
		0xC6, 0x01,       // ADD A,01
		0x27,             // DAA
	)
	mustStep(t, cpu) // LD
	mustStep(t, cpu) // ADD
	mustStep(t, cpu) // DAA
	if cpu.A != 0x10 {
		t.Errorf("DAA after 09+01 => got A=%02X want 10", cpu.A)
	}
	assertFlag(t, cpu, FlagN, false, "N cleared after DAA on addition")
	assertFlag(t, cpu, FlagC, false, "C should be 0 for 10h here")
}

func TestDAA_AfterAdd_UpperAdjustSetsCarry(t *testing.T) {
	cpu, mem, _ := testCPU()
	// A=0x90; ADD A,0x90 => 0x20 (with carry in BCD terms) ; DAA should add 0x60 -> A=0x80, C=1
	loadProgram(cpu, mem, 0x0000,
		0x3E, 0x90,       // LD A,90
		0xC6, 0x90,       // ADD A,90 -> A=0x20 (binary), needs +0x60
		0x27,             // DAA
	)
	mustStep(t, cpu)
	mustStep(t, cpu)
	mustStep(t, cpu)
	if cpu.A != 0x80 {
		t.Errorf("DAA after 90+90 => got A=%02X want 80", cpu.A)
	}
	assertFlag(t, cpu, FlagC, true, "DAA should set C when adding 0x60")
	assertFlag(t, cpu, FlagN, false, "N cleared on add-style DAA")
}

func TestDAA_AfterSub_HexToDecimalBorrow(t *testing.T) {
	cpu, mem, _ := testCPU()
	// A=0x10; SUB 0x01 => 0x0F; N=1; DAA should subtract 0x06 -> A=0x09 (BCD: 10 - 1 = 09)
	loadProgram(cpu, mem, 0x0000,
		0x3E, 0x10,       // LD A,10
		0xD6, 0x01,       // SUB 01
		0x27,             // DAA
	)
	mustStep(t, cpu)
	mustStep(t, cpu)
	mustStep(t, cpu)
	if cpu.A != 0x09 {
		t.Errorf("DAA after 10-01 => got A=%02X want 09", cpu.A)
	}
	assertFlag(t, cpu, FlagN, true, "N remains set for subtraction DAA path")
}
