package z80

import "testing"

// A minimal InterruptController for tests
type mockIC struct {
	mockIO
	vector    uint8
	mode0Inst []uint8
}

func (m *mockIC) GetInterruptVector() uint8    { return m.vector }
func (m *mockIC) GetMode0Instruction() []uint8 { return m.mode0Inst }

// IM2: verify vector fetch, PC transfer, SP push order, cycles=19.
func TestIM2_Vectoring_JumpAndCycles(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Replace IO with an InterruptController
	ic := &mockIC{mockIO: *newMockIO(), vector: 0x22}
	cpu.IO = ic

	// Arrange I, vector table entry, and target ISR address 0x3456
	cpu.I = 0x40
	tableAddr := uint16(cpu.I)<<8 | uint16(ic.vector & 0xFE)
	mem.Write(tableAddr, 0x56)       // low byte
	mem.Write(tableAddr+1, 0x34)     // high byte
	isr := uint16(0x3456)

	// Put a NOP at 0000 so we can advance PC first.
	loadProgram(cpu, mem, 0x0000, 0x00)
	cpu.SP = 0xFFFE
	cpu.IFF1, cpu.IFF2 = true, true
	cpu.IM = 2

	// Step NOP first (no INT yet)
	mustStep(t, cpu) // PC=0001 now

	// Now assert INT and accept the interrupt
	cpu.INT = true
	c := cpu.Step()
	if c != 19 {
		t.Fatalf("IM2 cycles=%d want 19", c)
	}
	// PC should now be isr
	if cpu.PC != isr {
		t.Fatalf("IM2 PC=%04X want %04X", cpu.PC, isr)
	}
	// Check push order (little-endian): old PC (after NOP) was 0x0001
	lo := mem.Read(cpu.SP)
	hi := mem.Read(cpu.SP+1)
	if lo != 0x01 || hi != 0x00 {
		t.Fatalf("Pushed PC bytes wrong: [%02X %02X] want [01 00]", lo, hi)
	}
}
