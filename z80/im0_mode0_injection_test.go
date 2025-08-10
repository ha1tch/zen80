package z80

import "testing"

// IM0: device injects an instruction (e.g., NOP). First Step returns 0 cycles to "arm" the buffer.
// Next Step executes the injected instruction and returns its cycles.
func TestIM0_Mode0_InstructionInjection_TwoStep(t *testing.T) {
	cpu, _, _ := testCPU()
	ic := &mockIC{mockIO: *newMockIO(), mode0Inst: []uint8{0x00}} // injected NOP
	cpu.IO = ic

	cpu.IM = 0
	cpu.IFF1, cpu.IFF2 = true, true
	cpu.INT = true

	// Arming step: returns 0 cycles and PREPARES buffer; mode0Active remains false until execution.
	c := cpu.Step()
	if c != 0 {
		t.Fatalf("IM0 arming cycles=%d want 0", c)
	}
	if cpu.mode0Active {
		t.Fatalf("IM0 arming should not set mode0Active yet")
	}
	if cpu.mode0Buffer == nil || cpu.mode0Index != 0 {
		t.Fatalf("IM0 arming did not prepare buffer/index correctly")
	}

	// Next step executes injected NOP (4 cycles), then clears mode0Active and empties buffer.
	c = cpu.Step()
	if c != 4 {
		t.Fatalf("IM0 injected instruction cycles=%d want 4", c)
	}
	if cpu.mode0Active {
		t.Fatalf("IM0 should clear mode0Active after executing buffer")
	}
	if cpu.mode0Buffer != nil {
		t.Fatalf("IM0 should clear buffer after executing instruction")
	}
}
