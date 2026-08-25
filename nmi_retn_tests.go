package z80

import "testing"

// RETN must copy IFF2 -> IFF1
func TestRETN_Copies_IFF2_to_IFF1(t *testing.T) {
	cpu, _, _ := testCPU()
	// Trigger NMI to set up a difference between IFF1 and IFF2 (IFF1 cleared)
	cpu.NMI = true
	mustStep(t, cpu) // Acknowledge NMI
	// After NMI, IFF1 must be 0; set IFF2=1 so we can verify RETN restoring it.
	cpu.IFF2 = true
	loadProgram(cpu, nil, cpu.PC, 0xED, 0x45) // RETN
	mustStep(t, cpu)
	if cpu.IFF1 != cpu.IFF2 || cpu.IFF1 != true {
		t.Fatalf("RETN did not copy IFF2->IFF1; IFF1=%v IFF2=%v", cpu.IFF1, cpu.IFF2)
	}
}

// NMI is edge-triggered: holding NMI high should not retrigger until it drops and rises again.
func TestNMI_EdgeTriggered_NoRetriggerWhileHeld(t *testing.T) {
	cpu, _, _ := testCPU()
	// Hold NMI high for multiple steps
	cpu.NMI = true
	mustStep(t, cpu) // first acknowledge
	pcAfterFirst := cpu.PC
	mustStep(t, cpu) // should not retrigger while still high
	if cpu.PC != pcAfterFirst {
		t.Fatalf("NMI retriggered while held high (PC advanced to %04X)", cpu.PC)
	}
	// Drop then raise again -> retrigger
	cpu.NMI = false
	mustStep(t, cpu) // no NMI now
	cpu.NMI = true
	mustStep(t, cpu) // should trigger again
	if cpu.PC == pcAfterFirst {
		t.Fatalf("NMI did not trigger after edge reassertion")
	}
}
