package z80

import "testing"

// Adversarial cases, each targeting a real risk category rather than
// repeating what z80n_semantics_test.go already covers: shared-plumbing
// correctness Z80N opcodes inherit but never independently proved, the
// exact boundary a bitmask flips at (not just "a large value"), a
// documented real-hardware danger zone (BC=0 at the start of a
// repeating block op), and 16-bit wraparound arithmetic.

// Z80N opcodes share executeED()'s single entry point with every classic
// ED instruction, which increments R before dispatch even looks at the
// opcode -- so R-correctness for Z80N should already hold for free. It's
// never been checked directly, though, and "should already hold" is
// exactly the kind of claim worth confirming rather than trusting.
func TestZ80N_Adversarial_RRegisterIncrements(t *testing.T) {
	// Caught by cross-checking before trusting the number: the first
	// version of this test expected +1. R actually increments TWICE
	// for any two-byte ED-prefixed instruction -- once for the 0xED
	// prefix fetch itself (Step()'s own main loop, z80.go), once more
	// for the opcode byte after it (executeED()'s own increment,
	// prefix_ed.go) -- confirmed against a known classic ED instruction
	// (NEG, ED 44) independently before fixing this test, not just
	// re-derived from the Z80N implementation being tested.
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.R = 0x00
	loadProgram(cpu, mem, 0, 0xED, 0x23) // SWAPNIB
	mustStep(t, cpu)
	assertEq(t, cpu.R, uint8(0x02), "R increments by 2 for a two-byte Z80N opcode -- prefix fetch + opcode fetch, same as any classic ED instruction")
}

// Z80N being on must not disturb classic instruction decoding for
// byte values Z80N does NOT define -- the two dispatch paths share the
// same opcode space and must not bleed into each other.
func TestZ80N_Adversarial_ClassicAndZ80NInterleaved(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0x00
	// NEG (ED 44, classic) ; SWAPNIB (ED 23, Z80N) ; NEG (ED 44, classic)
	loadProgram(cpu, mem, 0, 0xED, 0x44, 0xED, 0x23, 0xED, 0x44)
	mustStep(t, cpu) // NEG: A = 0-0 = 0
	assertEq(t, cpu.A, uint8(0x00), "first NEG unaffected by Z80N being on")
	cpu.A = 0xA5
	mustStep(t, cpu) // SWAPNIB
	assertEq(t, cpu.A, uint8(0x5A), "SWAPNIB fires correctly between two classic instructions")
	mustStep(t, cpu) // NEG: A = 0-0x5A
	assertEq(t, cpu.A, uint8(0xA6), "second NEG unaffected by the Z80N instruction that just ran")
}

// Classic LDI/LDIR's own well-known danger: starting with BC=0 does not
// mean "copy nothing" -- BC-- wraps to $FFFF and the loop (for the
// repeating forms) runs a full 64KiB. LDIX/LDIRX share the same BC--
// mechanics and must reproduce this exactly, not "helpfully" avoid it.
func TestZ80N_Adversarial_LdixBCStartsAtZero(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0x00
	cpu.SetHL(0x8000)
	cpu.SetDE(0x9000)
	cpu.SetBC(0)
	mem.Write(0x8000, 0x77)
	loadProgram(cpu, mem, 0, 0xED, 0xA4) // LDIX (single, non-repeating)
	mustStep(t, cpu)
	assertEq(t, mem.Read(0x9000), uint8(0x77), "the copy itself still happens with BC=0")
	assertEq(t, cpu.BC(), uint16(0xFFFF), "BC=0 wraps to 0xFFFF on decrement, exactly like classic LDI -- not clamped to 0")
}

// LDWS's own documented constraint: "source data are read only from a
// single 256B (aligned) block of memory, because only L is incremented,
// not HL". The adversarial case is L wrapping 0xFF -> 0x00 -- H must
// stay exactly where it was, not silently carry the way a real 16-bit
// increment would.
func TestZ80N_Adversarial_LdwsLWrapsWithoutTouchingH(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.H, cpu.L = 0x80, 0xFF // L at the top of its range
	cpu.D, cpu.E = 0x90, 0x00
	mem.Write(cpu.HL(), 0x99)
	loadProgram(cpu, mem, 0, 0xED, 0xA5)
	mustStep(t, cpu)
	assertEq(t, cpu.L, uint8(0x00), "L wraps 0xFF -> 0x00")
	assertEq(t, cpu.H, uint8(0x80), "H is untouched by the wrap -- LDWS never carries into it, by design")
}

// Every barrel shift at amount=0 must be a true no-op: the register
// pair unchanged, bit for bit. Easy to get wrong if a shift-by-zero
// path isn't special-cased and a language's shift operator does
// something unexpected at the boundary.
func TestZ80N_Adversarial_BarrelShiftsAtZeroAreNoOps(t *testing.T) {
	for _, tc := range []struct {
		name string
		byte uint8
	}{
		{"BSLA", 0x28}, {"BSRA", 0x29}, {"BSRL", 0x2A}, {"BSRF", 0x2B}, {"BRLC", 0x2C},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem, _ := testCPU()
			cpu.Z80N = true
			cpu.SetDE(0xBEEF)
			cpu.B = 0
			loadProgram(cpu, mem, 0, 0xED, tc.byte)
			mustStep(t, cpu)
			assertEq(t, cpu.DE(), uint16(0xBEEF), tc.name+" with B=0 must not change DE at all")
		})
	}
}

// The masking boundary itself, not just "some large B": BSLA/BSRA/BSRL/
// BSRF use B&31 (so B=32 wraps to amount=0, a true no-op -- 32 is NOT
// "shift by 32", it's "shift by 0"), while BRLC uses B&15 (so B=16
// wraps to amount=0). Getting either mask off by one silently turns a
// no-op into a full clear or a full-width shift, which is exactly the
// kind of bug that only shows up at the exact power-of-two boundary.
func TestZ80N_Adversarial_ShiftMaskBoundaries(t *testing.T) {
	t.Run("BSLA_B32_wraps_to_noop", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x1234)
		cpu.B = 32 // &31 = 0
		loadProgram(cpu, mem, 0, 0xED, 0x28)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x1234), "BSLA B=32 (&31=0) must be a no-op, not a full clear")
	})
	t.Run("BSLA_B31_exceeds_register_width", func(t *testing.T) {
		// Caught by cross-checking independently before trusting this
		// test: the first version asserted 0x8000, on the wrong
		// assumption that B=31 would move bit 0 to bit 15 (that needs
		// a shift of exactly 15, not 31). B&31 permits amounts past
		// the register's own 16-bit width -- at B=31, every bit is
		// shifted out and the correct result is a full clear, not a
		// partial shift. This is actually the more interesting
		// adversarial fact this case demonstrates: the mask is
		// deliberately wider than the register it shifts.
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x0001)
		cpu.B = 31 // the maximum this mask permits -- exceeds DE's 16 bits
		loadProgram(cpu, mem, 0, 0xED, 0x28)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x0000),
			"BSLA B=31 exceeds the 16-bit register width: everything shifts out, full clear")
	})
	t.Run("BRLC_B16_wraps_to_noop", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x1234)
		cpu.B = 16 // &15 = 0 -- a DIFFERENT boundary than the shifts' B=32
		loadProgram(cpu, mem, 0, 0xED, 0x2C)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x1234), "BRLC B=16 (&15=0) must be a no-op")
	})
	t.Run("BRLC_B15_rotates_almost_full_circle", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x8000)
		cpu.B = 15
		loadProgram(cpu, mem, 0, 0xED, 0x2C)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x4000), "BRLC B=15: bit 15 rotated all the way back to bit 14")
	})
}

// 16-bit wraparound, not just small in-range additions.
func TestZ80N_Adversarial_AddWraparound(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.SetHL(0xFFFF)
	cpu.A = 0x01
	cpu.setFlag(FlagC, true) // deliberately set beforehand: confirmed unchanged even across a wrap, not just in the ordinary case
	loadProgram(cpu, mem, 0, 0xED, 0x31) // ADD HL,A
	mustStep(t, cpu)
	assertEq(t, cpu.HL(), uint16(0x0000), "ADD HL,A wraps 0xFFFF+1 to 0x0000, plain modular 16-bit arithmetic")
	assertFlag(t, cpu, FlagC, true, "carry is left unchanged (confirmed ground truth via CSpect) even on a wrap, not reset")
}

// Maximum-magnitude multiply: confirms the 8x8->16 result isn't
// silently truncated anywhere in the widening.
func TestZ80N_Adversarial_MulMaxValues(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.D, cpu.E = 255, 255
	loadProgram(cpu, mem, 0, 0xED, 0x30)
	mustStep(t, cpu)
	assertEq(t, cpu.DE(), uint16(65025), "255*255=65025 must survive the 8x8->16 widen intact")
}
