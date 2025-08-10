package z80

import "testing"

// R must increment once per M1, including post-prefix opcode fetches.
func TestR_Increments_On_DDCB(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetIX(0x2000); mem.Write(0x2001, 0x01)
	loadProgram(cpu, mem, 0x0000, 0xDD, 0xCB, 0x01, 0x06) // RLC (IX+1)
	cpu.R = 0
	r0 := cpu.R & 0x7F
	mustStep(t, cpu)
	r1 := cpu.R & 0x7F
	inc := int((r1 - r0) & 0x7F)
	if inc != 3 {
		t.Fatalf("R should increment by 3 M1 cycles (DD, CB, op), got %d", inc)
	}
}
