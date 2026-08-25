package z80

import "testing"

// 16-bit loads and increments/decrements.
func TestLD_rp_nn_AND_INC_DEC_rp(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000,
		0x01, 0x34, 0x12, // LD BC,1234
		0x11, 0x78, 0x56, // LD DE,5678
		0x21, 0xCD, 0xAB, // LD HL,ABCD
		0x31, 0xFE, 0xFF, // LD SP,FFFE
		0x03, // INC BC
		0x13, // INC DE
		0x23, // INC HL
		0x33, // INC SP
		0x0B, // DEC BC
		0x1B, // DEC DE
		0x2B, // DEC HL
		0x3B, // DEC SP
	)

	c := mustStep(t, cpu); assertEq(t, c, 10, "LD BC,nn cycles"); assertEq(t, cpu.BC(), uint16(0x1234), "LD BC")
	c = mustStep(t, cpu); assertEq(t, c, 10, "LD DE,nn cycles"); assertEq(t, cpu.DE(), uint16(0x5678), "LD DE")
	c = mustStep(t, cpu); assertEq(t, c, 10, "LD HL,nn cycles"); assertEq(t, cpu.HL(), uint16(0xABCD), "LD HL")
	c = mustStep(t, cpu); assertEq(t, c, 10, "LD SP,nn cycles"); assertEq(t, cpu.SP, uint16(0xFFFE), "LD SP")

	assertEq(t, mustStep(t, cpu), 6, "INC BC cycles"); assertEq(t, cpu.BC(), uint16(0x1235), "INC BC")
	assertEq(t, mustStep(t, cpu), 6, "INC DE cycles"); assertEq(t, cpu.DE(), uint16(0x5679), "INC DE")
	assertEq(t, mustStep(t, cpu), 6, "INC HL cycles"); assertEq(t, cpu.HL(), uint16(0xABCE), "INC HL")
	assertEq(t, mustStep(t, cpu), 6, "INC SP cycles"); assertEq(t, cpu.SP, uint16(0xFFFF), "INC SP")

	assertEq(t, mustStep(t, cpu), 6, "DEC BC cycles"); assertEq(t, cpu.BC(), uint16(0x1234), "DEC BC")
	assertEq(t, mustStep(t, cpu), 6, "DEC DE cycles"); assertEq(t, cpu.DE(), uint16(0x5678), "DEC DE")
	assertEq(t, mustStep(t, cpu), 6, "DEC HL cycles"); assertEq(t, cpu.HL(), uint16(0xABCD), "DEC HL")
	assertEq(t, mustStep(t, cpu), 6, "DEC SP cycles"); assertEq(t, cpu.SP, uint16(0xFFFE), "DEC SP")
}
