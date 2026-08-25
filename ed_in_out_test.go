package z80

import "testing"

// Improved test: verify OUT (C),r writes to the CURRENT BC port after a prior IN changes B.
func TestED_IN_OUT_PortAndValue(t *testing.T) {
	cpu, mem, io := testCPU()
	// Arrange: BC=0x1234, IN will load 0x80 into B -> BC becomes 0x8034.
	cpu.SetBC(0x1234)
	io.inVals[0x1234] = 0x80

	// ED 40 = IN B,(C); ED 41 = OUT (C),B
	loadProgram(cpu, mem, 0x0000, 0xED, 0x40, 0xED, 0x41)

	// IN B,(C)
	mustStep(t, cpu)
	if cpu.B != 0x80 {
		t.Fatalf("IN B,(C) expected B=0x80, got %02X", cpu.B)
	}

	// OUT (C),B should use *current* BC (0x8034) and write B (0x80)
	mustStep(t, cpu)
	port := cpu.BC()
	val, ok := io.lastOut[port]
	if !ok {
		t.Fatalf("OUT (C),B wrote nothing to port %04X", port)
	}
	if val != cpu.B {
		t.Fatalf("OUT (C),B wrote %02X, want %02X", val, cpu.B)
	}

	// Also verify WZ behavior matches spec for IN/OUT: WZ = BC + 1
	if cpu.WZ != (cpu.BC()+1) {
		t.Fatalf("WZ after OUT should be BC+1: got %04X want %04X", cpu.WZ, cpu.BC()+1)
	}
}
