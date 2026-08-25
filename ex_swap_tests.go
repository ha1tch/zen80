package z80

import "testing"

// Validate EX AF,AF' behavior.
func TestEX_AF_AFPrime(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.A, cpu.F = 0x12, 0x34
	cpu.A_, cpu.F_ = 0xAB, 0xCD
	loadProgram(cpu, mem, 0x0000, 0x08) // EX AF,AF'
	mustStep(t, cpu)
	if cpu.A != 0xAB || cpu.F != 0xCD || cpu.A_ != 0x12 || cpu.F_ != 0x34 {
		t.Fatalf("EX AF,AF' swap failed: A=%02X F=%02X A'=%02X F'=%02X", cpu.A, cpu.F, cpu.A_, cpu.F_)
	}
}
