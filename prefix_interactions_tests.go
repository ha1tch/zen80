package z80

import "testing"

// DD followed by ED: DD is ignored; ED executes; cycles add 4.
func TestPrefix_DD_then_ED_NEG(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000, 0x3E, 0x01, 0xDD, 0xED, 0x44) // LD A,1 ; DD ; ED 44 NEG
	mustStep(t, cpu) // LD A,1
	c := mustStep(t, cpu) // DD+ED 44
	if cpu.A != 0xFF {
		t.Fatalf("NEG after DD ignored should set A=FF, got %02X", cpu.A)
	}
	// NEG is 8 cycles, DD adds 4
	if c != 12 {
		t.Fatalf("Cycles DD+NEG got %d want 12", c)
	}
}

// FD followed by DD opcode: FD ignored; DD executes with IX semantics; cycles include 4 for FD.
func TestPrefix_FD_then_DD_LoadIXd(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetIX(0x3000); mem.Write(0x3004, 0xAB)
	// FD DD 46 04  == FD ignored, then DD 46 04 => LD B,(IX+4)
	loadProgram(cpu, mem, 0x0000, 0xFD, 0xDD, 0x46, 0x04)
	c := mustStep(t, cpu)
	if cpu.B != 0xAB {
		t.Fatalf("LD B,(IX+4) should load 0xAB, got %02X", cpu.B)
	}
	// 19 for LD r,(IX+d) plus 4 for ignored FD
	if c != 23 {
		t.Fatalf("Cycles FD-ignored + DD LD r,(IX+d) got %d want 23", c)
	}
}
