package z80

// Z80N (ZX Spectrum Next) extended instruction bodies.
//
// This file exists so the dispatch table in prefix_ed.go stays the only
// thing that changes there -- every instruction's actual behaviour lives
// here instead. All 29 opcodes are now implemented, completing the
// three-step plan this file started with (dispatch surgery + mode flag +
// verification harness; then the ~20 mechanical opcodes; then the three
// genuine edge cases -- PUSH's unique big-endian operand, JP (C)'s new
// IO-to-PC plumbing, and NEXTREG). z.Z80N gates all of it and defaults to
// false, so a plain Z80 core is unaffected unless a caller opts in
// explicitly (see zenzx's EnableZ80N(), the CSpect/ZenZX cross-checked
// oracle harness these implementations were verified against, and
// Z80N_ZESARUX_CROSSCHECK.md for the verification trail behind the
// mechanical opcodes specifically).
//
// notImplementedZ80N is kept, unused, as a reminder of the discipline
// this file was built under: every stub, while it existed, panicked
// clearly rather than silently doing nothing, so an accidental call
// during development failed loudly instead of producing a wrong result
// that looked like a real one.

func notImplementedZ80N(name string) int {
	panic("z80n: " + name + " not yet implemented (Step 1 stub)")
}

// SWAPNIB ED 23 -- swap the two nibbles of A. Flags: all unchanged
// (SpecNext wiki: "-" across the board).
func (z *Z80) z80nSwapnib() int {
	z.A = (z.A << 4) | (z.A >> 4)
	return 8
}

// MIRROR A ED 24 -- bit-reverse A. Flags: all unchanged.
func (z *Z80) z80nMirror() int {
	a := z.A
	var r uint8
	for i := 0; i < 8; i++ {
		r = (r << 1) | (a & 1)
		a >>= 1
	}
	z.A = r
	return 8
}

// TEST $im8 ED 27 -- AND A,n and set flags; A itself is not affected.
// SpecNext wiki's summary table: "Change flags as AND A but A stays
// unaffected" -- taken as authoritative over the page's own detailed
// flag table for this opcode, whose C/H columns ("S") don't match a
// real bitwise AND's own well-established C=0,H=1 pattern and read as
// a copy-paste inconsistency against the prose right below it ("Similar
// to CP, but performs an AND instead of a subtraction" -- CP does SUB's
// flag work without writing A; this should be the same shape for AND).
func (z *Z80) z80nTest() int {
	n := z.fetchByte()
	result := z.A & n
	z.setFlag(FlagC, false)
	z.setFlag(FlagN, false)
	z.setFlag(FlagH, true)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagS, result&0x80 != 0)
	return 11
}

// BSLA DE,B ED 28 -- barrel-shift DE left by (B & 31) places. Flags:
// all unchanged. Shift amount uses bits 4..0 of B (SpecNext wiki: "Shift
// instructions use only bits 4..0 of B").
func (z *Z80) z80nBsla() int {
	amt := z.B & 0x1F
	z.SetDE(z.DE() << amt)
	return 8
}

// BSRA DE,B ED 29 -- barrel-shift DE right, arithmetic (sign-preserving)
// by (B & 31). Flags: all unchanged.
func (z *Z80) z80nBsra() int {
	amt := z.B & 0x1F
	z.SetDE(uint16(int16(z.DE()) >> amt))
	return 8
}

// BSRL DE,B ED 2A -- barrel-shift DE right, logical (zero-fill) by
// (B & 31). Flags: all unchanged.
func (z *Z80) z80nBsrl() int {
	amt := z.B & 0x1F
	z.SetDE(z.DE() >> amt)
	return 8
}

// BSRF DE,B ED 2B -- barrel-shift DE right, filling with ones, by
// (B & 31). Flags: all unchanged. SpecNext wiki's own C caveat: computed
// via the bitwise-NOT trick (~(~DE >> amt)) rather than a signed shift,
// since a naive implementation risks a language's own implicit sign
// extension doing the wrong thing here (the wiki calls this out
// explicitly for C implementers; Go's uint16 makes it moot, but the
// NOT-based form is kept to match the documented derivation exactly).
func (z *Z80) z80nBsrf() int {
	amt := z.B & 0x1F
	z.SetDE(^(^z.DE() >> amt))
	return 8
}

// BRLC DE,B ED 2C -- rotate (not shift) DE left by (B & 15) places.
// Flags: all unchanged. Uses bits 3..0 of B, unlike the four shift
// instructions above which use bits 4..0 -- confirmed from the wiki's
// own explicit note distinguishing the two. No separate right-rotate
// opcode exists; the wiki's own workaround is B=16-places to rotate
// right instead, left to the caller.
func (z *Z80) z80nBrlc() int {
	amt := z.B & 0x0F
	de := z.DE()
	if amt == 0 {
		return 8
	}
	z.SetDE((de << amt) | (de >> (16 - amt)))
	return 8
}

// MUL D,E ED 30 -- unsigned 8x8 multiply, D*E -> DE. No classic-Z80
// equivalent exists at all; this is genuinely new arithmetic capability,
// not a faster encoding of something that already existed. Flags: all
// unchanged (SpecNext wiki: "Does not alter any flags").
func (z *Z80) z80nMul() int {
	z.SetDE(uint16(z.D) * uint16(z.E))
	return 8
}

// ADD HL,A ED 31 -- HL += A (zero-extended). Carry: UNCHANGED. This
// reverses an earlier implementation (C:=0 always) that was built on the
// SpecNext wiki's 2025-01-25 hardware-test note ("most probably always
// reset"), which itself had superseded an older 2021-09-16 wiki note
// ("does NOT preserve carry unlike classic ADD HL,rr"). Both wiki claims
// are now understood to be wrong, established by a real ground-truth run,
// not by further reading: a Z80N test binary that SCFs (carry:=1) then
// executes ADD HL,A was built with three independently-developed
// assemblers producing byte-identical machine code (SNasm, zenas,
// sjasmplus), run to completion on real CSpect (driven through NextZXOS,
// screen-OCR'd for the result, no manual reading), and separately run to
// completion directly on this package's own core via ZenZX headless
// (-bin loading, memory read back once execution had reached a stable,
// non-drifting state -- confirmed stable at three different frame counts
// after the same load, not just read once). CSpect reported carry still
// set after the ADD; it was zen80's own ZenZX-embedded core, not either
// wiki note, that disagreed, which is what sent this back for a second
// look. ZEsarUX (instruccion_ed_49, z80_codpred.c) was checked earlier
// and implements plain HL+=A with no flags touched at all -- it turns
// out to have been right, not merely first: git-blame shows that line
// unchanged since ZEsarUX's very first commit (2022-03-04), nearly three
// years before either wiki note existed to contradict it, and the CSpect
// ground truth now confirms it over both.
func (z *Z80) z80nAddHLA() int {
	z.SetHL(z.HL() + uint16(z.A))
	return 8
}

// ADD DE,A ED 32 -- same carry treatment as ADD HL,A above (unchanged).
func (z *Z80) z80nAddDEA() int {
	z.SetDE(z.DE() + uint16(z.A))
	return 8
}

// ADD BC,A ED 33 -- same carry treatment as ADD HL,A above (unchanged).
func (z *Z80) z80nAddBCA() int {
	z.SetBC(z.BC() + uint16(z.A))
	return 8
}

// ADD HL,$im16 ED 34 -- HL += a 16-bit immediate. Flags: all unchanged
// (SpecNext wiki: "-" across the board, no ambiguity like the ,A forms).
func (z *Z80) z80nAddHLnn() int {
	z.SetHL(z.HL() + z.fetchWord())
	return 16
}

// ADD DE,$im16 ED 35 -- DE += a 16-bit immediate. Flags: all unchanged.
func (z *Z80) z80nAddDEnn() int {
	z.SetDE(z.DE() + z.fetchWord())
	return 16
}

// ADD BC,$im16 ED 36 -- BC += a 16-bit immediate. Flags: all unchanged.
func (z *Z80) z80nAddBCnn() int {
	z.SetBC(z.BC() + z.fetchWord())
	return 16
}

// PUSH $im16 ED 8A -- push a 16-bit immediate onto the stack. Confirmed
// from the SpecNext wiki: the ONLY operand in the entire Z80/Z80N
// instruction set encoded big-endian in the instruction stream itself
// ("ED 8A high low"). Reusing this package's existing little-endian
// fetchWord() here would silently byte-swap the value, so the two bytes
// are fetched explicitly in their documented order and combined by hand.
// Once assembled into a 16-bit value, though, it goes onto the stack via
// the same push() every other PUSH uses -- the stack's own layout (low
// byte at the lower address) is completely ordinary; it is only the
// instruction's operand encoding that is unique, not the effect it has.
func (z *Z80) z80nPush() int {
	high := z.fetchByte()
	low := z.fetchByte()
	z.push(uint16(high)<<8 | uint16(low))
	return 23
}

// OUTINB ED 90 -- like OUTI, but does not decrement B. Flags: the
// SpecNext wiki marks every flag "?" for this specific opcode -- no
// confirmed hardware behaviour documented, unlike OUTI's own flags
// (well-established, and this exact codebase's outi() already
// implements them correctly, including the 2022 wiki erratum that
// classic OUTI/OUTD/OTIR/OTDR do affect carry despite official Z80
// documentation long claiming otherwise). Rather than invent an
// independent flag algorithm for an undocumented case, this reuses
// outi()'s exact, already-verified computation and simply skips the
// B-- step -- the most defensible reading of "OUTI, but don't change
// B" when the flag behaviour itself isn't independently confirmed.
func (z *Z80) z80nOutinb() int {
	val := z.memRead(z.HL())
	z.ioOut(z.BC(), val)
	z.SetHL(z.HL() + 1)

	k := int(val) + int(z.L)
	z.setFlag(FlagZ, z.B == 0)
	z.setFlag(FlagS, z.B&0x80 != 0)
	z.setFlag(FlagN, (val&0x80) != 0)
	z.setFlag(FlagH, k > 0xFF)
	z.setFlag(FlagC, k > 0xFF)
	pvVal := uint8(k&0x07) ^ z.B
	z.setFlag(FlagPV, parity(pvVal))
	z.F = (z.F & 0xD7) | (z.B & (FlagX | FlagY))

	return 16
}

// nextRegSelectPort and nextRegDataPort are the two I/O ports NEXTREG's
// register-select-then-write behaviour is built from -- confirmed against
// multiple independent sources (the tbblue project's own port/register
// docs, the SpecNext wiki's Board_feature_control page, and the
// ZXSpectrumNextTests reference suite), not just carried over from an
// earlier, unverified guess: 0x243B selects which Next register is being
// addressed, 0x253B reads or writes that register's value. Real hardware
// implements NEXTREG as a direct internal register write, bypassing the
// port mechanism entirely -- but the *observable effect* on a running
// program is exactly the same as those two OUT instructions in sequence,
// and this emulator has no separate "internal Next register" write path
// of its own outside the IOInterface abstraction, so implementing NEXTREG
// as literally those two OUT calls is the correct, and only sensible,
// choice here.
const (
	nextRegSelectPort = 0x243B
	nextRegDataPort   = 0x253B
)

// NEXTREG $im8,$im8 ED 91 -- select a Next register and write an
// immediate value to it.
func (z *Z80) z80nNextregNN() int {
	reg := z.fetchByte()
	value := z.fetchByte()
	z.ioOut(nextRegSelectPort, reg)
	z.ioOut(nextRegDataPort, value)
	return 20
}

// NEXTREG $im8,A ED 92 -- select a Next register and write A to it.
func (z *Z80) z80nNextregA() int {
	reg := z.fetchByte()
	z.ioOut(nextRegSelectPort, reg)
	z.ioOut(nextRegDataPort, z.A)
	return 17
}

// PIXELDN ED 93 -- move HL down one pixel row in ULA screen memory,
// replicating the display's own non-linear Y-to-address bit layout.
// Exact formula from the SpecNext wiki:
//
//	if HL&$0700 != $0700: HL += 256
//	else if HL&$00E0 != $00E0: HL := (HL&$F8FF) + $20
//	else: HL := (HL&$F81F) + $0800
//
// Flags: all unchanged.
func (z *Z80) z80nPixeldn() int {
	hl := z.HL()
	switch {
	case hl&0x0700 != 0x0700:
		hl += 256
	case hl&0x00E0 != 0x00E0:
		hl = (hl & 0xF8FF) + 0x20
	default:
		hl = (hl & 0xF81F) + 0x0800
	}
	z.SetHL(hl)
	return 8
}

// PIXELAD ED 94 -- compute the ULA screen address for pixel (D,E) into
// HL: D is Y (0-191), E is X (0-255). Exact formula from the SpecNext
// wiki: HL := $4000 + ((D&$C0)<<5) + ((D&$07)<<8) + ((D&$38)<<2) + (E>>3).
// Same non-linear layout this session already hand-derived once, in
// scenerun.go's pxset() during the Oakhollow work -- confirmed
// structurally identical, D/Y and E/X roles matching. Flags: all
// unchanged.
func (z *Z80) z80nPixelad() int {
	d, e := uint16(z.D), uint16(z.E)
	z.SetHL(0x4000 + ((d & 0xC0) << 5) + ((d & 0x07) << 8) + ((d & 0x38) << 2) + (e >> 3))
	return 8
}

// SETAE ED 95 -- set A to the bit mask for a pixel's position within
// its screen byte, from E's low 3 bits, counted top-to-bottom (E=0 ->
// A=$80, E=7 -> A=$01). Flags: all unchanged.
func (z *Z80) z80nSetae() int {
	z.A = 0x80 >> (z.E & 7)
	return 8
}

// JP (C) ED 98 -- read I/O port BC (despite the mnemonic naming only C;
// the port address is the full 16-bit BC pair, the same convention every
// other Z80 "(C)"-form I/O instruction uses) and jump using the result.
// Confirmed formula from the SpecNext wiki, given twice in slightly
// different but equivalent forms -- the summary table's "PC:=PC&$C000+
// IN(C)<<6" and the detailed description's "sets bottom 14 bits of
// current PC to value read from I/O port": the top 2 bits of PC (which
// 16K quadrant) are preserved, the bottom 14 bits become the 8-bit input
// value shifted left 6 (so the destination always lands on a 64-byte
// boundary within that same quadrant -- "jump to a 64 byte section", per
// the wiki's own one-line description). "Current PC" is explicitly noted
// as the address of the instruction *after* this one -- i.e. PC as it
// already stands once this instruction's own 2 opcode bytes have been
// fetched, which is exactly z.PC's value at this point given the normal
// fetch-advances-PC convention this package already uses everywhere else,
// so no separate bookkeeping is needed to get that part right.
//
// The one instruction in the whole set with no classic-Z80 shape at all:
// an I/O read feeding directly into PC, rather than into a general
// register first -- new plumbing, not a reused jump code path.
//
// Flags: the wiki marks every flag "?" for this specific opcode, with no
// hardware-tested note establishing anything more specific (unlike, say,
// OUTINB, which had a well-established analog -- OUTI -- to borrow a
// verified computation from). Left entirely unchanged rather than
// guessing: the conservative choice for a genuinely undocumented case,
// not a claim that "unchanged" is itself a confirmed hardware finding.
func (z *Z80) z80nJpC() int {
	val := z.ioIn(z.BC())
	z.PC = (z.PC & 0xC000) | (uint16(val) << 6)
	return 13
}

// LDIX ED A4 -- like LDI, but suppresses the destination write if the
// source byte equals A (pointers still advance either way). Flags: a
// real, documented contradiction on the SpecNext wiki, resolved in
// favour of the newer, hardware-tested claim -- the page's own detailed
// instruction table marks every flag "-" (unaffected), but its dated
// observations section (2025-01-25, "Testing 3.02.x") explicitly
// corrects that: LDIX/LDDX/LDIRX/LDDRX "affect the flags similarly to
// LDI, LDD, LDIR and LDDR" -- WITHOUT specifying the exact X/Y (undoc-
// umented bit-3/bit-5) formula. That gap was originally filled here by
// assuming "similarly" meant reusing ldi()'s own val+A derivation --
// checked against ZEsarUX's real, independent implementation
// (z80_codpred.c, instruccion_ed_164, unchanged since the file's first
// commit) and that assumption was wrong: ZEsarUX derives X/Y from the
// raw byte read (byte_leido&8, byte_leido&2), never adding A. Unlike
// the ADD HL/DE/BC,A carry case below, there's no newer, dated,
// specific claim to weigh against ZEsarUX here -- the wiki simply never
// commits to a formula for this one -- so this is a genuine correction
// to a prior guess, not a source-preference call.
func (z *Z80) z80nLdix() int {
	val := z.memRead(z.HL())
	if val != z.A {
		z.memWrite(z.DE(), val)
	}
	z.SetDE(z.DE() + 1)
	z.SetHL(z.HL() + 1)
	z.SetBC(z.BC() - 1)

	z.F &= FlagS | FlagZ | FlagC // clears H, N, PV, X, Y; preserves S, Z, C
	if z.BC() != 0 {
		z.F |= FlagPV
	}
	if val&0x08 != 0 {
		z.F |= FlagX
	}
	if val&0x02 != 0 {
		z.F |= FlagY
	}
	return 16
}

// LDWS ED A5 -- copies the byte at HL to DE, then increments ONLY L
// and D (not H, not E) -- deliberately confined to a single 256-byte-
// aligned block, used for vertical copies into the Layer 2 display.
// Flags: per the SpecNext wiki, "identical to what the INC D
// instruction would produce" -- reuses this package's own inc8(),
// the exact classic-INC-r flag+value computation, applied to D.
func (z *Z80) z80nLdws() int {
	val := z.memRead(z.HL())
	z.memWrite(z.DE(), val)
	z.L++
	z.D = z.inc8(z.D)
	return 14
}

// LDDX ED AC -- like LDIX, but HL decrements while DE still increments
// (confirmed from the wiki's own prose: "LDDX/LDDRX advance DE by
// incrementing it (like LDI), while HL is decremented (like LDD)" --
// NOT a simple mirror of LDIX). Flags: same ZEsarUX-corrected X/Y
// derivation as LDIX above (raw byte read, not byte+A) -- confirmed
// identical pattern in ZEsarUX's instruccion_ed_172, same file, same
// original commit.
func (z *Z80) z80nLddx() int {
	val := z.memRead(z.HL())
	if val != z.A {
		z.memWrite(z.DE(), val)
	}
	z.SetDE(z.DE() + 1)
	z.SetHL(z.HL() - 1)
	z.SetBC(z.BC() - 1)

	z.F &= FlagS | FlagZ | FlagC
	if z.BC() != 0 {
		z.F |= FlagPV
	}
	if val&0x08 != 0 {
		z.F |= FlagX
	}
	if val&0x02 != 0 {
		z.F |= FlagY
	}
	return 16
}

// LDIRX ED B4 -- repeated LDIX until BC=0, same 21-while-repeating/
// 16-on-final T-state shape as classic LDIR.
func (z *Z80) z80nLdirx() int {
	c := z.z80nLdix()
	if z.BC() != 0 {
		z.PC -= 2
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return c
}

// LDPIRX ED B7 -- pattern-fill copy. Unlike every other LDxX opcode,
// HL never advances -- it's a fixed base address. Each iteration reads
// from ((HL & $FFF8) | (E & 7)), not HL itself: HL's top 13 bits pick
// an 8-byte-aligned lookup table, DE's low 3 bits (wrapping 0..7) index
// into it. Only DE and BC advance. Flags: the 2025-01-25 correction
// note explicitly lists LDIX/LDDX/LDIRX/LDDRX -- LDPIRX is conspicuously
// absent from that list, so this implements the main table's own
// claim ("-", unaffected) rather than assuming the same correction
// silently extends to an opcode the note doesn't actually name. Lower
// confidence than the other LDxX flag choices for exactly that reason;
// worth re-checking if real-hardware testing ever covers it directly.
func (z *Z80) z80nLdpirx() int {
	src := (z.HL() & 0xFFF8) | uint16(z.E&0x07)
	val := z.memRead(src)
	if val != z.A {
		z.memWrite(z.DE(), val)
	}
	z.SetDE(z.DE() + 1)
	z.SetBC(z.BC() - 1)
	if z.BC() != 0 {
		z.PC -= 2
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

// LDDRX ED BC -- repeated LDDX until BC=0, same 21/16 T-state shape.
func (z *Z80) z80nLddrx() int {
	c := z.z80nLddx()
	if z.BC() != 0 {
		z.PC -= 2
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return c
}
