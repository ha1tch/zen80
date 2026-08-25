package z80

import "testing"

func TestR_Increments_On_ED_Neg(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000, 0xED, 0x44) // NEG
	cpu.R = 0
	r0 := cpu.R & 0x7F
	mustStep(t, cpu)
	r1 := cpu.R & 0x7F
	// Expect 2 M1s: ED fetch after the initial opcode fetch
	inc := int((r1 - r0) & 0x7F)
	if inc != 2 {
		t.Fatalf("R should increment by 2 for ED-prefixed single op, got %d", inc)
	}
}

func TestR_Increments_On_DD_DD_Nop(t *testing.T) {
	cpu, mem, _ := testCPU()
	// DD DD <anything> -- our executeDD returns 4 for a second DD as NOP;
	// we only execute one Step here: first DD, then post-prefix M1 fetches second DD, then stop.
	loadProgram(cpu, mem, 0x0000, 0xDD, 0xDD)
	cpu.R = 0
	r0 := cpu.R & 0x7F
	mustStep(t, cpu)
	r1 := cpu.R & 0x7F
	// Should see 2 M1 increments (first DD fetch + second DD as post-prefix fetch)
	if int((r1-r0)&0x7F) != 2 {
		t.Fatalf("R should increment by 2 on DD DD, got %d", int((r1-r0)&0x7F))
	}
}
