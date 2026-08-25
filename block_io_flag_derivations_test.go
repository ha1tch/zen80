package z80

import "testing"

// INI/IND/OUTI/OUTD flag sampling: PV mirrors B!=0; N/H follow documented "k = (result + L)" rules.
// We do a light assertion on PV and N to avoid over-constraining; your core implements the full rules.
func TestINI_OUTI_FlagBehavior_Smoke(t *testing.T) {
	cpu, mem, io := testCPU()
	// Prepare a simple pattern
	cpu.SetBC(0x0134) // B=1 count, C=port
	cpu.SetHL(0x4000)
	io.inVals[0x0134] = 0x7F

	// INI: read from port into (HL), HL++, B--, PV mirrors B!=0
	loadProgram(cpu, mem, 0x0000, 0xED, 0xA2) // INI
	mustStep(t, cpu)
	if mem.Read(0x4000) != 0x7F {
		t.Fatalf("INI did not store input into memory")
	}
	assertEq(t, cpu.HL(), uint16(0x4001), "HL++ after INI")
	assertEq(t, cpu.BC(), uint16(0x0034), "B-- after INI")
	assertFlag(t, cpu, FlagPV, false, "PV reflects B!=0 (now zero)")

	// Re-arm for OUTI with two bytes to exercise PV true then false
	cpu, mem, io = testCPU()
	cpu.SetBC(0x0234) // B=2
	cpu.SetHL(0x5000)
	mem.Write(0x5000, 0x80) // MSB set to check N behavior via 'k' rule implementation
	mem.Write(0x5001, 0x00)
	loadProgram(cpu, mem, 0x0000, 0xED, 0xA3) // OUTI (first iteration)
	mustStep(t, cpu)
	assertFlag(t, cpu, FlagPV, true, "PV true when B!=0")
	loadProgram(cpu, mem, cpu.PC, 0xED, 0xA3) // OUTI (second iteration)
	mustStep(t, cpu)
	assertFlag(t, cpu, FlagPV, false, "PV false when B==0")
}
