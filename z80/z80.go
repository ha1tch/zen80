// Package z80 implements an instruction-stepped Z80 CPU emulator.
package z80

// Z80 represents the state of a Z80 CPU.
type Z80 struct {
	// Main registers
	A, F uint8 // Accumulator and Flags
	B, C uint8 // BC register pair
	D, E uint8 // DE register pair
	H, L uint8 // HL register pair

	// Alternate registers
	A_, F_ uint8 // Alternate accumulator and flags
	B_, C_ uint8 // Alternate BC
	D_, E_ uint8 // Alternate DE
	H_, L_ uint8 // Alternate HL

	// Index registers
	IXH, IXL uint8 // IX register (high and low)
	IYH, IYL uint8 // IY register (high and low)

	// Special registers
	I  uint8  // Interrupt vector
	R  uint8  // Memory refresh
	SP uint16 // Stack pointer
	PC uint16 // Program counter

	// Internal registers
	WZ uint16 // Internal temporary register (MEMPTR)

	// Interrupt flip-flops
	IFF1 bool // Interrupt enable flip-flop 1
	IFF2 bool // Interrupt enable flip-flop 2
	IM   uint8 // Interrupt mode (0, 1, or 2)

	// State tracking
	Halted        bool   // CPU is halted
	Cycles        uint64 // Total cycles executed
	pendingEI     bool   // EI instruction just executed
	pendingDI     bool   // DI instruction just executed

	// Memory interface
	Memory MemoryInterface

	// I/O interface
	IO IOInterface

	// Interrupt handling
	NMI     bool // Non-maskable interrupt pending
	INT     bool // Maskable interrupt pending
	nmiEdge bool // For NMI edge detection
}

// MemoryInterface defines the interface for memory access.
type MemoryInterface interface {
	Read(address uint16) uint8
	Write(address uint16, value uint8)
}

// IOInterface defines the interface for I/O port access.
type IOInterface interface {
	In(port uint16) uint8
	Out(port uint16, value uint8)
}

// Flag bits
const (
	FlagC  uint8 = 0x01 // Carry
	FlagN  uint8 = 0x02 // Add/Subtract
	FlagPV uint8 = 0x04 // Parity/Overflow
	FlagH  uint8 = 0x10 // Half Carry
	FlagZ  uint8 = 0x40 // Zero
	FlagS  uint8 = 0x80 // Sign
	
	// Undocumented flags (bits 3 and 5)
	FlagX  uint8 = 0x08 // Copy of bit 3
	FlagY  uint8 = 0x20 // Copy of bit 5
)

// New creates a new Z80 CPU instance.
func New(memory MemoryInterface, io IOInterface) *Z80 {
	return &Z80{
		Memory: memory,
		IO:     io,
		SP:     0xFFFF,
		PC:     0x0000,
		AF:     0xFFFF,
	}
}

// Reset resets the CPU to its initial state.
func (z *Z80) Reset() {
	z.PC = 0x0000
	z.I = 0x00
	z.R = 0x00
	z.IFF1 = false
	z.IFF2 = false
	z.IM = 0
	z.Halted = false
	z.pendingEI = false
	z.pendingDI = false
}

// Step executes one instruction and returns the number of cycles taken.
func (z *Z80) Step() int {
	// Handle interrupts
	if z.handleInterrupts() {
		// Interrupt was serviced
		return z.getLastInstructionCycles()
	}

	// If halted, just count cycles
	if z.Halted {
		z.Cycles += 4
		return 4
	}

	// Handle delayed interrupt enable/disable
	if z.pendingEI {
		z.IFF1 = true
		z.IFF2 = true
		z.pendingEI = false
	}
	if z.pendingDI {
		z.IFF1 = false
		z.IFF2 = false
		z.pendingDI = false
	}

	// Fetch and execute instruction
	opcode := z.fetchByte()
	cycles := z.execute(opcode)
	
	// Update R register (lower 7 bits only)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	
	z.Cycles += uint64(cycles)
	return cycles
}

// Run executes instructions until the CPU halts or the condition function returns false.
func (z *Z80) Run(condition func() bool) {
	for condition() && !z.Halted {
		z.Step()
	}
}

// Register pair getters
func (z *Z80) AF() uint16 { return uint16(z.A)<<8 | uint16(z.F) }
func (z *Z80) BC() uint16 { return uint16(z.B)<<8 | uint16(z.C) }
func (z *Z80) DE() uint16 { return uint16(z.D)<<8 | uint16(z.E) }
func (z *Z80) HL() uint16 { return uint16(z.H)<<8 | uint16(z.L) }
func (z *Z80) IX() uint16 { return uint16(z.IXH)<<8 | uint16(z.IXL) }
func (z *Z80) IY() uint16 { return uint16(z.IYH)<<8 | uint16(z.IYL) }

// Register pair setters
func (z *Z80) SetAF(val uint16) { z.A = uint8(val >> 8); z.F = uint8(val) }
func (z *Z80) SetBC(val uint16) { z.B = uint8(val >> 8); z.C = uint8(val) }
func (z *Z80) SetDE(val uint16) { z.D = uint8(val >> 8); z.E = uint8(val) }
func (z *Z80) SetHL(val uint16) { z.H = uint8(val >> 8); z.L = uint8(val) }
func (z *Z80) SetIX(val uint16) { z.IXH = uint8(val >> 8); z.IXL = uint8(val) }
func (z *Z80) SetIY(val uint16) { z.IYH = uint8(val >> 8); z.IYL = uint8(val) }

// Memory access helpers
func (z *Z80) fetchByte() uint8 {
	val := z.Memory.Read(z.PC)
	z.PC++
	return val
}

func (z *Z80) fetchWord() uint16 {
	low := z.fetchByte()
	high := z.fetchByte()
	return uint16(high)<<8 | uint16(low)
}

func (z *Z80) readWord(addr uint16) uint16 {
	low := z.Memory.Read(addr)
	high := z.Memory.Read(addr + 1)
	return uint16(high)<<8 | uint16(low)
}

func (z *Z80) writeWord(addr uint16, val uint16) {
	z.Memory.Write(addr, uint8(val))
	z.Memory.Write(addr+1, uint8(val>>8))
}

// Stack operations
func (z *Z80) push(val uint16) {
	z.SP--
	z.Memory.Write(z.SP, uint8(val>>8))
	z.SP--
	z.Memory.Write(z.SP, uint8(val))
}

func (z *Z80) pop() uint16 {
	low := z.Memory.Read(z.SP)
	z.SP++
	high := z.Memory.Read(z.SP)
	z.SP++
	return uint16(high)<<8 | uint16(low)
}

// Flag helpers
func (z *Z80) getFlag(flag uint8) bool {
	return (z.F & flag) != 0
}

func (z *Z80) setFlag(flag uint8, value bool) {
	if value {
		z.F |= flag
	} else {
		z.F &^= flag
	}
}

// Condition code helpers
func (z *Z80) testCondition(cc uint8) bool {
	switch cc {
	case 0: return !z.getFlag(FlagZ)  // NZ
	case 1: return z.getFlag(FlagZ)   // Z
	case 2: return !z.getFlag(FlagC)  // NC
	case 3: return z.getFlag(FlagC)   // C
	case 4: return !z.getFlag(FlagPV) // PO
	case 5: return z.getFlag(FlagPV)  // PE
	case 6: return !z.getFlag(FlagS)  // P
	case 7: return z.getFlag(FlagS)   // M
	default: return false
	}
}

// Helper to track last instruction cycles (simplified)
func (z *Z80) getLastInstructionCycles() int {
	// This is a simplification - in a real implementation
	// we'd track the actual cycles of the last instruction
	return 11 // Default interrupt acknowledge cycles
}

// handleInterrupts checks and processes pending interrupts
func (z *Z80) handleInterrupts() bool {
	// Check for NMI (edge-triggered)
	if z.NMI && !z.nmiEdge {
		z.nmiEdge = true
		z.Halted = false
		z.IFF1 = false
		z.push(z.PC)
		z.PC = 0x0066
		z.WZ = z.PC
		return true
	}
	if !z.NMI {
		z.nmiEdge = false
	}

	// Check for maskable interrupt
	if z.INT && z.IFF1 && !z.pendingEI && !z.pendingDI {
		z.Halted = false
		z.IFF1 = false
		z.IFF2 = false
		
		switch z.IM {
		case 0:
			// Mode 0: Execute instruction on data bus (simplified - execute RST 38H)
			z.push(z.PC)
			z.PC = 0x0038
		case 1:
			// Mode 1: RST 38H
			z.push(z.PC)
			z.PC = 0x0038
		case 2:
			// Mode 2: Vectored interrupt (simplified)
			z.push(z.PC)
			vector := uint16(z.I)<<8 | 0xFF
			z.PC = z.readWord(vector)
		}
		z.WZ = z.PC
		return true
	}

	return false
}