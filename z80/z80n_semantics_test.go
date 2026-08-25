package z80

import "testing"

// Each test loads Z80N=true directly (bypassing dispatch, already proven
// correct in z80n_test.go) and checks documented behaviour: the exact
// register/memory/flag outcome the SpecNext wiki specifies, including
// the specific corrections noted in each instruction's own z80n.go
// comment where the wiki's tables and its dated observations disagree.

func TestZ80N_Swapnib(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0xA5
	loadProgram(cpu, mem, 0, 0xED, 0x23)
	c := mustStep(t, cpu)
	assertEq(t, cpu.A, uint8(0x5A), "SWAPNIB swaps nibbles")
	assertEq(t, c, 8, "SWAPNIB cycles")
}

func TestZ80N_Mirror(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0b10110001
	loadProgram(cpu, mem, 0, 0xED, 0x24)
	mustStep(t, cpu)
	assertEq(t, cpu.A, uint8(0b10001101), "MIRROR A reverses bit order")
}

func TestZ80N_Test(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0xF0
	loadProgram(cpu, mem, 0, 0xED, 0x27, 0x0F) // A & $0F = 0
	mustStep(t, cpu)
	assertEq(t, cpu.A, uint8(0xF0), "TEST does not modify A")
	assertFlag(t, cpu, FlagZ, true, "TEST: A&n==0 sets Z")
	assertFlag(t, cpu, FlagC, false, "TEST always clears C, like AND")
	assertFlag(t, cpu, FlagH, true, "TEST always sets H, like AND")
}

func TestZ80N_BarrelShifts(t *testing.T) {
	t.Run("BSLA", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x0001)
		cpu.B = 4
		loadProgram(cpu, mem, 0, 0xED, 0x28)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x0010), "BSLA DE,B: DE<<4")
	})
	t.Run("BSRA_sign_preserved", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x8000) // negative as int16
		cpu.B = 4
		loadProgram(cpu, mem, 0, 0xED, 0x29)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0xF800), "BSRA sign-extends the top bit")
	})
	t.Run("BSRL_zero_fill", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x8000)
		cpu.B = 4
		loadProgram(cpu, mem, 0, 0xED, 0x2A)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x0800), "BSRL zero-fills regardless of sign")
	})
	t.Run("BSRF_one_fill", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x0000)
		cpu.B = 4
		loadProgram(cpu, mem, 0, 0xED, 0x2B)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0xF000), "BSRF fills with ones even from all-zero input")
	})
	t.Run("BRLC_rotate_wraps", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0x8001)
		cpu.B = 1
		loadProgram(cpu, mem, 0, 0xED, 0x2C)
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x0003), "BRLC rotates the top bit around to the bottom")
	})
	t.Run("Shift_amount_masking_31_vs_15", func(t *testing.T) {
		// BSLA/BSRA/BSRL/BSRF use B&31; BRLC uses B&15. B=16 must
		// therefore behave as amount=16 for the shifts (clearing a
		// 16-bit value) but as amount=0 for BRLC (a no-op rotate).
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetDE(0xFFFF)
		cpu.B = 16
		loadProgram(cpu, mem, 0, 0xED, 0x28) // BSLA
		mustStep(t, cpu)
		assertEq(t, cpu.DE(), uint16(0x0000), "BSLA with B=16 (&31=16) shifts out everything")

		cpu2, mem2, _ := testCPU()
		cpu2.Z80N = true
		cpu2.SetDE(0x1234)
		cpu2.B = 16
		loadProgram(cpu2, mem2, 0, 0xED, 0x2C) // BRLC
		mustStep(t, cpu2)
		assertEq(t, cpu2.DE(), uint16(0x1234), "BRLC with B=16 (&15=0) is a no-op rotate")
	})
}

func TestZ80N_Mul(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.D, cpu.E = 12, 12
	loadProgram(cpu, mem, 0, 0xED, 0x30)
	mustStep(t, cpu)
	assertEq(t, cpu.DE(), uint16(144), "MUL D,E: 12*12=144")
}

func TestZ80N_AddRR_A(t *testing.T) {
	for _, tc := range []struct {
		name string
		byte uint8
		get  func(*Z80) uint16
	}{
		{"HL", 0x31, (*Z80).HL},
		{"DE", 0x32, (*Z80).DE},
		{"BC", 0x33, (*Z80).BC},
	} {
		t.Run("ADD_"+tc.name+",A", func(t *testing.T) {
			cpu, mem, _ := testCPU()
			cpu.Z80N = true
			cpu.setFlag(FlagC, true) // prove it is left alone, not cleared -- confirmed via CSpect ground truth
			switch tc.byte {
			case 0x31:
				cpu.SetHL(0x1000)
			case 0x32:
				cpu.SetDE(0x1000)
			case 0x33:
				cpu.SetBC(0x1000)
			}
			cpu.A = 0x22
			loadProgram(cpu, mem, 0, 0xED, tc.byte)
			mustStep(t, cpu)
			assertEq(t, tc.get(cpu), uint16(0x1022), "ADD "+tc.name+",A adds A zero-extended")
			assertFlag(t, cpu, FlagC, true,
				"ADD "+tc.name+",A leaves carry unchanged (confirmed via CSpect ground truth, reversing the earlier 2025-01-25 wiki-based assumption)")
		})
	}
}

// TestZ80N_AddRR_A_CarryStartsClear is the symmetric case to
// TestZ80N_AddRR_A above: "unchanged" needs checking in both directions,
// not just that a set carry survives -- a clear carry must also survive,
// including across an addition that would set carry under ordinary
// 16-bit ADD rules (0x1000+0xFF here does not itself overflow 16 bits,
// but the point is the same: this opcode does not compute a carry out of
// the addition at all, so nothing should set the flag either).
func TestZ80N_AddRR_A_CarryStartsClear(t *testing.T) {
	for _, tc := range []struct {
		name string
		byte uint8
		get  func(*Z80) uint16
	}{
		{"HL", 0x31, (*Z80).HL},
		{"DE", 0x32, (*Z80).DE},
		{"BC", 0x33, (*Z80).BC},
	} {
		t.Run("ADD_"+tc.name+",A", func(t *testing.T) {
			cpu, mem, _ := testCPU()
			cpu.Z80N = true
			cpu.setFlag(FlagC, false)
			switch tc.byte {
			case 0x31:
				cpu.SetHL(0x1000)
			case 0x32:
				cpu.SetDE(0x1000)
			case 0x33:
				cpu.SetBC(0x1000)
			}
			cpu.A = 0xFF
			loadProgram(cpu, mem, 0, 0xED, tc.byte)
			mustStep(t, cpu)
			assertEq(t, tc.get(cpu), uint16(0x10FF), "ADD "+tc.name+",A adds A zero-extended")
			assertFlag(t, cpu, FlagC, false,
				"ADD "+tc.name+",A leaves a clear carry clear too -- unchanged means both directions")
		})
	}
}

func TestZ80N_AddRR_im16(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.SetHL(0x1000)
	loadProgram(cpu, mem, 0, 0xED, 0x34, 0x34, 0x12) // ADD HL,$1234
	mustStep(t, cpu)
	assertEq(t, cpu.HL(), uint16(0x2234), "ADD HL,$im16")
}

func TestZ80N_Outinb(t *testing.T) {
	cpu, mem, io := testCPU()
	cpu.Z80N = true
	cpu.B, cpu.C = 0x05, 0x10
	cpu.SetHL(0x8000)
	mem.Write(0x8000, 0x77)
	loadProgram(cpu, mem, 0, 0xED, 0x90)
	mustStep(t, cpu)
	assertEq(t, cpu.B, uint8(0x05), "OUTINB does not decrement B")
	assertEq(t, cpu.HL(), uint16(0x8001), "OUTINB increments HL")
	assertEq(t, io.lastOut[0x0510], uint8(0x77), "OUTINB writes the byte at HL to port BC")
}

func TestZ80N_Pixeldn(t *testing.T) {
	// The three-way branch, one case each.
	t.Run("plain_increment", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetHL(0x4000) // $0700 bits clear -> +=256
		loadProgram(cpu, mem, 0, 0xED, 0x93)
		mustStep(t, cpu)
		assertEq(t, cpu.HL(), uint16(0x4100), "PIXELDN plain-row case")
	})
	t.Run("mid_wrap", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetHL(0x4700) // $0700 set, $00E0 clear -> (HL&$F8FF)+$20
		loadProgram(cpu, mem, 0, 0xED, 0x93)
		mustStep(t, cpu)
		// (0x4700 & 0xF8FF) + 0x20 = 0x4000 + 0x20 = 0x4020. Verified by
		// hand computation, not just re-derived from the implementation
		// itself, after the first version of this test asserted 0x4720
		// (a genuine arithmetic mistake in the test, caught by actually
		// running it rather than trusting the number on sight).
		assertEq(t, cpu.HL(), uint16(0x4020), "PIXELDN mid-third-wrap case")
	})
	t.Run("full_wrap", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.SetHL(0x47E0) // both $0700 and $00E0 set -> (HL&$F81F)+$0800
		loadProgram(cpu, mem, 0, 0xED, 0x93)
		mustStep(t, cpu)
		assertEq(t, cpu.HL(), uint16(0x4800), "PIXELDN full-wrap-to-next-third case")
	})
}

func TestZ80N_Pixelad(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.D, cpu.E = 0, 0 // Y=0, X=0 -> top-left pixel, address $4000
	loadProgram(cpu, mem, 0, 0xED, 0x94)
	mustStep(t, cpu)
	assertEq(t, cpu.HL(), uint16(0x4000), "PIXELAD(0,0) = $4000")
}

func TestZ80N_Setae(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.E = 3
	loadProgram(cpu, mem, 0, 0xED, 0x95)
	mustStep(t, cpu)
	assertEq(t, cpu.A, uint8(0x10), "SETAE: E=3 -> A=$80>>3=$10")
}

func TestZ80N_Ldix(t *testing.T) {
	t.Run("flags_from_ZEsarUX_cross_check", func(t *testing.T) {
		// Confirms the ZEsarUX-derived correction: X/Y come from the
		// raw byte read (bit3->X, bit1->Y), NOT byte+A as this file
		// originally assumed by reusing ldi()'s own formula. 0x0B has
		// both source bits set; 0x04 has neither, isolating the check
		// from any val+A coincidence.
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.A = 0x00
		cpu.SetHL(0x8000)
		cpu.SetDE(0x9000)
		cpu.SetBC(2) // != 0 after decrement, so PV should end up set
		mem.Write(0x8000, 0x0B) // bit3=1, bit1=1
		loadProgram(cpu, mem, 0, 0xED, 0xA4)
		mustStep(t, cpu)
		assertFlag(t, cpu, FlagX, true, "LDIX: X from byte's own bit 3, byte=0x0B")
		assertFlag(t, cpu, FlagY, true, "LDIX: Y from byte's own bit 1, byte=0x0B")
		assertFlag(t, cpu, FlagPV, true, "LDIX: PV set, BC!=0 after decrement")
		assertFlag(t, cpu, FlagH, false, "LDIX: H always cleared")
		assertFlag(t, cpu, FlagN, false, "LDIX: N always cleared")

		cpu2, mem2, _ := testCPU()
		cpu2.Z80N = true
		cpu2.A = 0x00
		cpu2.SetHL(0x8000)
		cpu2.SetDE(0x9000)
		cpu2.SetBC(1) // == 0 after decrement, PV should end up clear
		mem2.Write(0x8000, 0x04) // bit3=0, bit1=0
		loadProgram(cpu2, mem2, 0, 0xED, 0xA4)
		mustStep(t, cpu2)
		assertFlag(t, cpu2, FlagX, false, "LDIX: X clear, byte=0x04 has no bit 3")
		assertFlag(t, cpu2, FlagY, false, "LDIX: Y clear, byte=0x04 has no bit 1")
		assertFlag(t, cpu2, FlagPV, false, "LDIX: PV clear, BC==0 after decrement")
	})
	t.Run("copies_when_different", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.A = 0x00
		cpu.SetHL(0x8000)
		cpu.SetDE(0x9000)
		cpu.SetBC(1)
		mem.Write(0x8000, 0x55)
		mem.Write(0x9000, 0xAA)
		loadProgram(cpu, mem, 0, 0xED, 0xA4)
		mustStep(t, cpu)
		assertEq(t, mem.Read(0x9000), uint8(0x55), "LDIX copies when byte != A")
		assertEq(t, cpu.HL(), uint16(0x8001), "LDIX advances HL")
		assertEq(t, cpu.DE(), uint16(0x9001), "LDIX advances DE")
	})
	t.Run("suppresses_when_equal_to_A", func(t *testing.T) {
		cpu, mem, _ := testCPU()
		cpu.Z80N = true
		cpu.A = 0x55
		cpu.SetHL(0x8000)
		cpu.SetDE(0x9000)
		cpu.SetBC(1)
		mem.Write(0x8000, 0x55) // equals A
		mem.Write(0x9000, 0xAA) // must NOT be overwritten
		loadProgram(cpu, mem, 0, 0xED, 0xA4)
		mustStep(t, cpu)
		assertEq(t, mem.Read(0x9000), uint8(0xAA),
			"LDIX suppresses the write when the source byte equals A")
		assertEq(t, cpu.HL(), uint16(0x8001), "LDIX still advances HL when suppressed")
		assertEq(t, cpu.DE(), uint16(0x9001), "LDIX still advances DE when suppressed")
	})
}

func TestZ80N_Ldws(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.H, cpu.L = 0x80, 0xFE
	cpu.D, cpu.E = 0x90, 0x10
	mem.Write(cpu.HL(), 0x42)
	loadProgram(cpu, mem, 0, 0xED, 0xA5)
	mustStep(t, cpu)
	assertEq(t, mem.Read(0x9010), uint8(0x42), "LDWS copies the byte")
	assertEq(t, cpu.L, uint8(0xFF), "LDWS increments L")
	assertEq(t, cpu.H, uint8(0x80), "LDWS does NOT touch H")
	assertEq(t, cpu.D, uint8(0x91), "LDWS increments D")
	assertEq(t, cpu.E, uint8(0x10), "LDWS does NOT touch E")
}

func TestZ80N_Lddx(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0x00
	cpu.SetHL(0x8000)
	cpu.SetDE(0x9000)
	cpu.SetBC(1)
	mem.Write(0x8000, 0x55)
	loadProgram(cpu, mem, 0, 0xED, 0xAC)
	mustStep(t, cpu)
	assertEq(t, mem.Read(0x9000), uint8(0x55), "LDDX copies")
	assertEq(t, cpu.HL(), uint16(0x7FFF), "LDDX decrements HL")
	assertEq(t, cpu.DE(), uint16(0x9001), "LDDX INCREMENTS DE (not a mirror of LDIX)")
}

func TestZ80N_Ldirx_RepeatsUntilBCZero(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0x00
	cpu.SetHL(0x8000)
	cpu.SetDE(0x9000)
	cpu.SetBC(3)
	mem.Write(0x8000, 0x01)
	mem.Write(0x8001, 0x02)
	mem.Write(0x8002, 0x03)
	loadProgram(cpu, mem, 0, 0xED, 0xB4)
	for cpu.BC() != 0 {
		mustStep(t, cpu)
	}
	assertEq(t, mem.Read(0x9000), uint8(0x01), "LDIRX byte 1")
	assertEq(t, mem.Read(0x9001), uint8(0x02), "LDIRX byte 2")
	assertEq(t, mem.Read(0x9002), uint8(0x03), "LDIRX byte 3")
	assertEq(t, cpu.HL(), uint16(0x8003), "LDIRX final HL")
	assertEq(t, cpu.DE(), uint16(0x9003), "LDIRX final DE")
}

func TestZ80N_Lddrx_RepeatsUntilBCZero(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0x00
	cpu.SetHL(0x8002)
	cpu.SetDE(0x9000)
	cpu.SetBC(3)
	mem.Write(0x8000, 0x01)
	mem.Write(0x8001, 0x02)
	mem.Write(0x8002, 0x03)
	loadProgram(cpu, mem, 0, 0xED, 0xBC)
	for cpu.BC() != 0 {
		mustStep(t, cpu)
	}
	// DE increments (per the wiki: LDDX/LDDRX advance DE like LDI),
	// HL decrements -- so byte order at the destination is reversed
	// relative to source, not mirrored.
	assertEq(t, mem.Read(0x9000), uint8(0x03), "LDDRX first-copied byte lands at DE's start")
	assertEq(t, mem.Read(0x9001), uint8(0x02), "LDDRX second byte")
	assertEq(t, mem.Read(0x9002), uint8(0x01), "LDDRX third byte")
	assertEq(t, cpu.HL(), uint16(0x7FFF), "LDDRX final HL")
	assertEq(t, cpu.DE(), uint16(0x9003), "LDDRX final DE")
}

func TestZ80N_Ldpirx(t *testing.T) {
	// HL never advances; source is (HL&$FFF8)|(E&7), wrapping through
	// an 8-byte-aligned table as DE's low 3 bits increment.
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.A = 0x00
	cpu.SetHL(0x8000) // base of an 8-byte-aligned table
	cpu.SetDE(0x9000)
	cpu.SetBC(3)
	mem.Write(0x8000, 0xAA) // table[0], since E&7 starts at 0
	mem.Write(0x8001, 0xBB) // table[1]
	mem.Write(0x8002, 0xCC) // table[2]
	loadProgram(cpu, mem, 0, 0xED, 0xB7)
	for cpu.BC() != 0 {
		mustStep(t, cpu)
	}
	assertEq(t, mem.Read(0x9000), uint8(0xAA), "LDPIRX table[0]")
	assertEq(t, mem.Read(0x9001), uint8(0xBB), "LDPIRX table[1]")
	assertEq(t, mem.Read(0x9002), uint8(0xCC), "LDPIRX table[2]")
	assertEq(t, cpu.HL(), uint16(0x8000), "LDPIRX never advances HL -- it's a fixed base")
	assertEq(t, cpu.DE(), uint16(0x9003), "LDPIRX advances DE normally")
}

// TestZ80N_Push_BigEndianOperand is the specific check the z80nPush doc
// comment promises: the operand is encoded high-byte-then-low-byte in the
// instruction stream (the one place in the whole Z80/Z80N set that is),
// so a naive reuse of fetchWord() would silently byte-swap it. Encoding
// ED 8A $12 $34 must push $1234, not $3412.
func TestZ80N_Push_BigEndianOperand(t *testing.T) {
	cpu, mem, _ := testCPU()
	cpu.Z80N = true
	cpu.SP = 0x9000
	loadProgram(cpu, mem, 0, 0xED, 0x8A, 0x12, 0x34)
	c := mustStep(t, cpu)
	assertEq(t, cpu.SP, uint16(0x8FFE), "PUSH $im16 decrements SP by 2")
	assertEq(t, mem.Read(0x8FFE), uint8(0x34), "low byte at the lower stack address")
	assertEq(t, mem.Read(0x8FFF), uint8(0x12), "high byte at the higher stack address")
	assertEq(t, cpu.readWord(cpu.SP), uint16(0x1234),
		"the pushed value reads back as $1234, not a byte-swapped $3412")
	assertEq(t, c, 23, "PUSH $im16 cycles")
}

// TestZ80N_NextregNN checks NEXTREG $im8,$im8 resolves to exactly the two
// documented port writes, register-select then data, in that order and
// with the right values -- not just "some I/O activity happened".
func TestZ80N_NextregNN(t *testing.T) {
	cpu, mem, io := testCPU()
	cpu.Z80N = true
	loadProgram(cpu, mem, 0, 0xED, 0x91, 0x07, 0x99)
	c := mustStep(t, cpu)
	assertEq(t, io.lastOut[0x243B], uint8(0x07), "NEXTREG selects register 0x07 via port 0x243B")
	assertEq(t, io.lastOut[0x253B], uint8(0x99), "NEXTREG writes 0x99 via port 0x253B")
	assertEq(t, c, 20, "NEXTREG $im8,$im8 cycles")
}

// TestZ80N_NextregA is NextregNN's sibling: same register-select port,
// but the value written comes from A rather than a second immediate byte.
func TestZ80N_NextregA(t *testing.T) {
	cpu, mem, io := testCPU()
	cpu.Z80N = true
	cpu.A = 0x42
	loadProgram(cpu, mem, 0, 0xED, 0x92, 0x15)
	c := mustStep(t, cpu)
	assertEq(t, io.lastOut[0x243B], uint8(0x15), "NEXTREG selects register 0x15 via port 0x243B")
	assertEq(t, io.lastOut[0x253B], uint8(0x42), "NEXTREG writes A's value (0x42) via port 0x253B")
	assertEq(t, c, 17, "NEXTREG $im8,A cycles")
}

// TestZ80N_JpC checks the documented formula directly: PC := (PC & $C000)
// | (IN(C) << 6), where "current PC" is the address immediately after
// this instruction's own two opcode bytes. Starting the code at 0x8000
// (rather than 0x0000) makes the "preserve the top 2 bits" half of the
// formula a real, non-trivial check -- at 0x0000 a bug that zeroed those
// bits entirely would pass by accident.
func TestZ80N_JpC(t *testing.T) {
	cpu, mem, io := testCPU()
	cpu.Z80N = true
	cpu.SetBC(0x1234)
	io.inVals[0x1234] = 0x05
	loadProgram(cpu, mem, 0x8000, 0xED, 0x98)
	c := mustStep(t, cpu)
	// PC after fetching ED 98 is 0x8002; & $C000 = 0x8000. IN(C)=0x05,
	// <<6 = 0x0140. Expected: 0x8000 | 0x0140 = 0x8140.
	assertEq(t, cpu.PC, uint16(0x8140), "JP (C) preserves top 2 bits of PC, replaces the rest with IN(C)<<6")
	assertEq(t, c, 13, "JP (C) cycles")
}

// TestZ80N_JpC_ReadsFullBC confirms the I/O read uses the full 16-bit BC
// pair as the port address (the standard Z80 "(C)"-form convention),
// not just the 8-bit C register in isolation -- a mock that only responds
// on the exact BC value would fail if the implementation read some other
// address (e.g. 0x00C0 instead of the full BC).
func TestZ80N_JpC_ReadsFullBC(t *testing.T) {
	cpu, mem, io := testCPU()
	cpu.Z80N = true
	cpu.SetBC(0xABCD)
	io.inVals[0xABCD] = 0x01
	io.inVals[0x00CD] = 0xFF // a plausible wrong answer if only C were used
	loadProgram(cpu, mem, 0x0000, 0xED, 0x98)
	mustStep(t, cpu)
	assertEq(t, cpu.PC, uint16(0x0040), "JP (C) must read port BC (0xABCD), not just C (0x00CD)")
}
