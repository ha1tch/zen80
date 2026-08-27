package z80

import "testing"

// BIT n,(HL) takes X/Y from the PRE-EXISTING WZ high byte -- the
// inherited MEMPTR residue of earlier instructions -- and must leave WZ
// unchanged. A regression here previously refreshed WZ=HL+1 before the
// flag computation, making the flags a function of HL; Speedlock's
// keystream (Batman) detects exactly that.
func TestBIT_HL_XYFromWZHigh(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Put value at 0x4000, set HL=0x4000, then CB 7E = BIT 7,(HL).
	// Pre-load WZ with residue whose high byte differs from H in both
	// X and Y bits: 0x2800 has Y(bit5)=1 X(bit3)=1; H=0x40 has both 0.
	mem.Write(0x4000, 0x80) // bit 7 set
	cpu.SetHL(0x4000)
	cpu.WZ = 0x2800
	loadProgram(cpu, mem, 0x0000, 0xCB, 0x7E)
	c := mustStep(t, cpu)

	// For BIT 7,(HL), Z=0, S=1, PV mirrors Z, H=1, N=0.
	assertFlag(t, cpu, FlagZ, false, "BIT Z")
	// X/Y come from the INHERITED WZ high byte (0x28), not from HL.
	assertFlag(t, cpu, FlagX, (0x28&FlagX) != 0, "X from inherited WZ high")
	assertFlag(t, cpu, FlagY, (0x28&FlagY) != 0, "Y from inherited WZ high")
	// And the operation must not have modified WZ.
	assertEq(t, cpu.WZ, uint16(0x2800), "plain CB (HL) leaves WZ unchanged")

	assertEq(t, c, 12, "cycles for BIT n,(HL)")
}

// RLC (HL): verify write-back and timing 15 cycles.
func TestRLC_HL_WriteBackAndTiming(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.SetHL(0x2000)
	mem.Write(0x2000, 0x81)                   // 1000 0001 -> RLC -> 0000 0011, C=1
	loadProgram(cpu, mem, 0x0000, 0xCB, 0x06) // RLC (HL)
	c := mustStep(t, cpu)
	assertEq(t, mem.Read(0x2000), uint8(0x03), "RLC(HL) write-back")
	assertFlag(t, cpu, FlagC, true, "C set after RLC")
	assertEq(t, c, 15, "cycles for RLC (HL)")
}

// A halted Z80 executes NOP M1 cycles: R must advance once per halted
// step, preserving bit 7. A regression froze R during HALT; Speedlock
// keystreams (Batman) read R after HALT frame-syncs and detect it.
func TestHALT_RAdvances(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000, 0x76) // HALT
	mustStep(t, cpu)                    // executes HALT
	r0 := cpu.R & 0x7F
	hi := cpu.R & 0x80
	for i := 0; i < 200; i++ {
		mustStep(t, cpu) // halted NOP M1 cycles
	}
	assertEq(t, cpu.R&0x7F, (r0+200)&0x7F, "R advances per halted step")
	assertEq(t, cpu.R&0x80, hi, "R bit 7 preserved during HALT")
}

// JP cc,nn and CALL cc,nn must set WZ = nn from the operand fetch even
// when the condition is NOT met -- MEMPTR is a side effect of fetching
// the address, not of branching. A regression left WZ stale on the
// not-taken path for both instructions.
func TestJPcc_NotTaken_StillSetsWZ(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.setFlag(FlagZ, true) // NZ condition false -> not taken
	cpu.WZ = 0x0000
	loadProgram(cpu, mem, 0x0000, 0xC2, 0x34, 0x12) // JP NZ,0x1234
	pcBefore := cpu.PC
	c := mustStep(t, cpu)
	assertEq(t, cpu.PC, pcBefore+3, "PC advances past the not-taken JP cc")
	assertEq(t, cpu.WZ, uint16(0x1234), "WZ set from operand even when not taken")
	assertEq(t, c, 10, "cycles for JP cc,nn not taken")
}

func TestCALLcc_NotTaken_StillSetsWZ(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.setFlag(FlagZ, true) // NZ condition false -> not taken
	cpu.WZ = 0x0000
	cpu.SP = 0xFFF0
	loadProgram(cpu, mem, 0x0000, 0xC4, 0x34, 0x12) // CALL NZ,0x1234
	spBefore := cpu.SP
	c := mustStep(t, cpu)
	assertEq(t, cpu.SP, spBefore, "SP unchanged: not-taken CALL cc pushes nothing")
	assertEq(t, cpu.WZ, uint16(0x1234), "WZ set from operand even when not taken")
	assertEq(t, c, 10, "cycles for CALL cc,nn not taken")
}
