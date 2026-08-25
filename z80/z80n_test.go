package z80

import (
	"strings"
	"testing"
)

// z80nOpcodes is the full carved-out set: byte value -> the instruction
// name its stub should report when Z80N mode routes to it. Table-driven
// so both halves of Step 1's own claim -- "off by default is byte-for-
// byte unchanged" and "on, dispatch reaches the right handler" -- are
// checked against the exact same list, rather than two lists that could
// silently drift apart.
var z80nOpcodes = []struct {
	byte        uint8
	name        string
	implemented bool // false = Step 3 opcode, still a Step 1 stub
}{
	{0x23, "SWAPNIB", true},
	{0x24, "MIRROR A", true},
	{0x27, "TEST $im8", true},
	{0x28, "BSLA DE,B", true},
	{0x29, "BSRA DE,B", true},
	{0x2A, "BSRL DE,B", true},
	{0x2B, "BSRF DE,B", true},
	{0x2C, "BRLC DE,B", true},
	{0x30, "MUL D,E", true},
	{0x31, "ADD HL,A", true},
	{0x32, "ADD DE,A", true},
	{0x33, "ADD BC,A", true},
	{0x34, "ADD HL,$im16", true},
	{0x35, "ADD DE,$im16", true},
	{0x36, "ADD BC,$im16", true},
	{0x8A, "PUSH $im16", true},
	{0x90, "OUTINB", true},
	{0x91, "NEXTREG $im8,$im8", true},
	{0x92, "NEXTREG $im8,A", true},
	{0x93, "PIXELDN", true},
	{0x94, "PIXELAD", true},
	{0x95, "SETAE", true},
	{0x98, "JP (C)", true},
	{0xA4, "LDIX", true},
	{0xA5, "LDWS", true},
	{0xAC, "LDDX", true},
	{0xB4, "LDIRX", true},
	{0xB7, "LDPIRX", true},
	{0xBC, "LDDRX", true},
}

// TestZ80N_OffByDefault_ByteForByteUnchanged confirms the core Step 1
// promise: with z.Z80N left at its default (false), every carved-out
// byte value behaves exactly as it did before this feature existed --
// 8 cycles, PC+2, zero register side effects. This is the same check
// already run by hand against a scratch program before any of this
// file existed (see the plan's own Step 1 note); it lives here now so
// it runs on every future change, not just once.
func TestZ80N_OffByDefault_ByteForByteUnchanged(t *testing.T) {
	for _, tc := range z80nOpcodes {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem, _ := testCPU()
			loadProgram(cpu, mem, 0x0000, 0xED, tc.byte)
			beforeA, beforeF := cpu.A, cpu.F
			beforeB, beforeC := cpu.B, cpu.C
			beforeSP := cpu.SP
			pc := cpu.PC
			c := mustStep(t, cpu)
			assertEq(t, c, 8, "ED "+hex2(tc.byte)+" should still be 8 cycles with Z80N off")
			assertEq(t, cpu.PC, pc+2, "ED "+hex2(tc.byte)+" should still advance PC by 2")
			assertEq(t, cpu.A, beforeA, "ED "+hex2(tc.byte)+" should not touch A with Z80N off")
			assertEq(t, cpu.F, beforeF, "ED "+hex2(tc.byte)+" should not touch F with Z80N off")
			assertEq(t, cpu.B, beforeB, "ED "+hex2(tc.byte)+" should not touch B with Z80N off")
			assertEq(t, cpu.C, beforeC, "ED "+hex2(tc.byte)+" should not touch C with Z80N off")
			assertEq(t, cpu.SP, beforeSP, "ED "+hex2(tc.byte)+" should not touch SP with Z80N off")
		})
	}
}

// TestZ80N_On_DispatchReachesCorrectHandler confirmed, during Steps 1-3,
// that each byte value routes to its own named stub, not some other
// opcode's, before any real semantics existed to check instead: every
// stub panicked with its own instruction name (see z80n.go), which was
// exactly the signal available at the time. As each opcode's stub was
// replaced with real behaviour, its z80nOpcodes entry flipped to
// implemented: true and this test started skipping it (see the loop
// below) -- the intended, visible signal, at each step, that dispatch
// correctness for that opcode had moved to z80n_semantics_test.go
// instead, which checks it against documented behaviour rather than
// just a panic message.
//
// All 29 opcodes are implemented as of Step 3, so every entry is now
// skipped and this test currently runs zero sub-tests -- expected, not
// a bug: the table and the skip-if-implemented loop stay in place
// deliberately, so that if a future opcode were ever reverted to a stub
// (or a new one added and not yet implemented), this test would
// immediately start exercising it again with no further changes needed.
func TestZ80N_On_DispatchReachesCorrectHandler(t *testing.T) {
	for _, tc := range z80nOpcodes {
		if tc.implemented {
			continue // real semantics now exist -- see z80n_semantics_test.go
		}
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem, _ := testCPU()
			cpu.Z80N = true
			loadProgram(cpu, mem, 0x0000, 0xED, tc.byte)
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("ED %s (%s): expected the Step 1 stub to panic "+
						"(no real implementation exists yet); got no panic "+
						"at all -- dispatch may not be reaching this opcode",
						hex2(tc.byte), tc.name)
				}
				msg, ok := r.(string)
				if !ok || !strings.Contains(msg, tc.name) {
					t.Fatalf("ED %s: panic did not name %q -- dispatch "+
						"likely routed to the wrong handler (got: %v)",
						hex2(tc.byte), tc.name, r)
				}
			}()
			cpu.Step()
		})
	}
}

func hex2(b uint8) string {
	const hexDigits = "0123456789ABCDEF"
	return "0x" + string(hexDigits[b>>4]) + string(hexDigits[b&0xF])
}
