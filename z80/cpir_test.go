package z80

import "testing"

// CPIR/CPDR repeat-cycle accounting: repeating step is 21 cycles, final match step is 16.
// We run steps until Z=1 or BC=0 and assert total cycles and final state.
func TestCPIR_CycleProfile_AndBehavior(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Setup HL points to 0x4000: [0xAA, 0x55]; A=0x55; BC=2
	mem.Write(0x4000, 0xAA)
	mem.Write(0x4001, 0x55)
	cpu.SetHL(0x4000)
	cpu.SetBC(2)
	cpu.A = 0x55

	// ED B1 = CPIR
	loadProgram(cpu, mem, 0x0000, 0xED, 0xB1)

	total := 0
	for {
		c := mustStep(t, cpu)
		total += c
		if cpu.getFlag(FlagZ) || cpu.BC() == 0 {
			break
		}
	}

	// After CPIR completes: should stop with Z=1, HL=0x4002, BC=0
	assertEq(t, cpu.HL(), uint16(0x4002), "HL after CPIR")
	assertEq(t, cpu.BC(), uint16(0), "BC after CPIR")
	assertFlag(t, cpu, FlagZ, true, "Z set when match found")
	// Timing: 21 (first repeat) + 16 (final) = 37
	assertEq(t, total, 37, "CPIR total cycles for one repeat + final")
}
