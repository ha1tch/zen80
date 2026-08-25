package z80

import "testing"

// Highlights blind spot: BIT n,r should update X/Y from the operand (register form).
// Current implementation only fixes the (HL) form, so this test will fail until fixed.
func TestBIT_Reg_SetsXYFromOperand(t *testing.T) {
	cpu, mem, _ := testCPU()
	// Load: LD B,0x28 (0010_1000: bit3=1 -> X=1, bit5=1 -> Y=1)
	loadProgram(cpu, mem, 0x0000,
		0x06, 0x28,       // LD B,28h
		0xCB, 0x40,       // BIT 0,B  (arbitrary bit test that does not force Z)
	)
	mustStep(t, cpu) // LD
	mustStep(t, cpu) // BIT 0,B
	// Expect X/Y to mirror operand (B) on BIT n,r.
	assertFlag(t, cpu, FlagX, true, "BIT n,r should set X from tested register")
	assertFlag(t, cpu, FlagY, true, "BIT n,r should set Y from tested register")
}
