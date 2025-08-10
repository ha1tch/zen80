// Package z80 implements an instruction-stepped Z80 CPU emulator.
package z80

import "log"

// Configuration flags
var (
	DEBUG_TIMING = false // Set to true to enable cycle verification
	DEBUG_M1     = true  // Set to true to enable M1 cycle tracing
)

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
	IFF1 bool  // Interrupt enable flip-flop 1
	IFF2 bool  // Interrupt enable flip-flop 2
	IM   uint8 // Interrupt mode (0, 1, or 2)

	// State tracking
	Halted     bool   // CPU is halted
	Cycles     uint64 // Total cycles executed
	pendingEI  bool   // EI instruction just executed
	pendingDI  bool   // DI instruction just executed
	lastPrefix uint8  // Last prefix for cycle verification (0=none, 0xCB, 0xDD, 0xED, 0xFD)
	lastCycles int    // Cycles from last executed instruction

	// Debug hooks
	M1Hook func(pc uint16, opcode uint8, context string) // Called on M1 cycles when DEBUG_M1 is true

	// Memory interface
	Memory MemoryInterface

	// I/O interface
	IO IOInterface

	// Interrupt handling
	NMI     bool // Non-maskable interrupt pending
	INT     bool // Maskable interrupt pending (level-triggered - must be cleared by external hardware)
	nmiEdge bool // For NMI edge detection (prevents re-triggering while held high)

	// Mode 0 interrupt instruction buffer
	mode0Buffer []uint8 // Instruction bytes for Mode 0 interrupt
	mode0Index  int     // Current position in mode0Buffer
	mode0Active bool    // True when executing from mode0Buffer
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

// InterruptController is an optional interface for devices that provide
// interrupt vectors and Mode 0 instructions
type InterruptController interface {
	IOInterface
	// GetInterruptVector returns the interrupt vector for Mode 2
	// The Z80 will combine this with the I register to form the full address
	GetInterruptVector() uint8
	// GetMode0Instruction returns the instruction bytes to execute for Mode 0
	// Can return 1-4 bytes for a complete instruction
	GetMode0Instruction() []uint8
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
	FlagX uint8 = 0x08 // Copy of bit 3
	FlagY uint8 = 0x20 // Copy of bit 5
)

// Interrupt acknowledge cycle counts
// Based on official Z80 documentation and verified against real hardware:
// - NMI: 5 cycles (M1 acknowledge) + 6 cycles (2x memory write for PUSH) = 11
// - IM1: 7 cycles (M1 acknowledge) + 6 cycles (2x memory write for PUSH) = 13
// - IM2: 7 cycles (M1 acknowledge) + 6 cycles (PUSH) + 6 cycles (vector read) = 19
const (
	NMI_CYCLES = 11 // NMI acknowledge + push PC
	IM1_CYCLES = 13 // Mode 1: interrupt acknowledge + RST 38H
	IM2_CYCLES = 19 // Mode 2: interrupt acknowledge + read vector + jump
)

// New creates a new Z80 CPU instance.
func New(memory MemoryInterface, io IOInterface) *Z80 {
	return &Z80{
		Memory: memory,
		IO:     io,
		SP:     0xFFFF,
		PC:     0x0000,
		A:      0xFF,
		F:      0xFF,
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
	z.lastPrefix = 0
	z.lastCycles = 0
	z.mode0Buffer = nil
	z.mode0Index = 0
	z.mode0Active = false
	// Don't reset Cycles - keep the total count
}

// Step executes one instruction and returns the number of cycles taken.
func (z *Z80) Step() int {
	// Check if we're in the middle of executing a Mode 0 interrupt instruction
	if z.mode0Buffer != nil && z.mode0Index < len(z.mode0Buffer) {
		cycles := z.executeMode0Instruction()
		z.lastCycles = cycles
		z.Cycles += uint64(cycles)
		return cycles
	}

	// Handle interrupts
	if cycles, handled := z.handleInterrupts(); handled {
		z.lastCycles = cycles
		z.Cycles += uint64(cycles)
		return cycles
	}

	// If halted, just count cycles
	if z.Halted {
		z.lastCycles = 4
		z.Cycles += 4
		return 4
	}

	// Handle delayed interrupt enable/disable
	// IMPORTANT: This happens AFTER interrupt checking, so an interrupt cannot
	// occur on the instruction immediately following EI. This is correct Z80 behavior -
	// EI delays interrupt recognition by one instruction to allow setting up SP safely.
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
	startPC := z.PC // Save for debugging
	opcode := z.fetchByte()

	// Increment R register immediately after M1 cycle (opcode fetch)
	// This ensures LD A,R sees the post-increment value
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)

	// Debug M1 trace
	if DEBUG_M1 && z.M1Hook != nil {
		z.M1Hook(startPC, opcode, "normal")
	}

	// Track prefix for cycle verification
	z.lastPrefix = 0
	if opcode == 0xCB || opcode == 0xDD || opcode == 0xED || opcode == 0xFD {
		z.lastPrefix = opcode
	}

	cycles := z.execute(opcode)

	// Verify cycle timing if enabled
	if DEBUG_TIMING {
		// For conditional instructions, we can't easily determine if branch was taken
		// without more complex tracking, so we just verify that cycles is reasonable
		if !z.VerifyInstructionTiming(opcode, z.lastPrefix, cycles) {
			log.Printf("WARNING: Cycle count mismatch at PC=%04X, opcode=%02X, prefix=%02X, cycles=%d",
				startPC, opcode, z.lastPrefix, cycles)
		}
	}

	z.lastCycles = cycles
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
	// Check if we're executing from Mode 0 buffer
	if z.mode0Active && z.mode0Buffer != nil && z.mode0Index < len(z.mode0Buffer) {
		val := z.mode0Buffer[z.mode0Index]
		z.mode0Index++
		return val
	}

	// Normal memory fetch
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
	case 0:
		return !z.getFlag(FlagZ) // NZ
	case 1:
		return z.getFlag(FlagZ) // Z
	case 2:
		return !z.getFlag(FlagC) // NC
	case 3:
		return z.getFlag(FlagC) // C
	case 4:
		return !z.getFlag(FlagPV) // PO
	case 5:
		return z.getFlag(FlagPV) // PE
	case 6:
		return !z.getFlag(FlagS) // P
	case 7:
		return z.getFlag(FlagS) // M
	default:
		return false
	}
}

// handleInterrupts checks and processes pending interrupts
// Returns (cycles, handled) where cycles is the number of cycles consumed
// and handled is true if an interrupt was processed.
//
// IMPORTANT: The INT line is level-triggered. External hardware/peripherals must
// clear the INT signal after the interrupt is serviced, otherwise it will
// continuously re-trigger. This matches real Z80 behavior.
func (z *Z80) handleInterrupts() (int, bool) {
	// Check for NMI (edge-triggered, low-to-high transition)
	if z.NMI && !z.nmiEdge {
		z.nmiEdge = true
		z.Halted = false
		z.IFF1 = false
		z.push(z.PC)
		z.PC = 0x0066
		z.WZ = z.PC
		// Increment R for the NMI acknowledge M1 cycle
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		// Debug M1 trace
		if DEBUG_M1 && z.M1Hook != nil {
			z.M1Hook(0x0066, 0x00, "NMI")
		}
		return NMI_CYCLES, true
	}
	if !z.NMI {
		z.nmiEdge = false
	}

	// Check for maskable interrupt (INT is level-triggered and must be
	// cleared by external hardware after servicing)
	if z.INT && z.IFF1 && !z.pendingEI && !z.pendingDI {
		z.Halted = false
		z.IFF1 = false
		z.IFF2 = false

		switch z.IM {
		case 0:
			// Mode 0: Execute instruction provided by interrupting device
			// The instruction will be fetched and executed in the next Step()
			if ic, ok := z.IO.(InterruptController); ok {
				// Get instruction from interrupt controller
				inst := ic.GetMode0Instruction()
				if len(inst) > 0 {
					// Store instruction for execution
					z.mode0Buffer = inst
					z.mode0Index = 0
					z.mode0Active = false // Will be set true in executeMode0Instruction
					// IMPORTANT: Return 0 cycles BY DESIGN - this is not a bug!
					// Mode 0 interrupt handling is split into two phases:
					// 1. This "arming" phase (0 cycles) that prepares the instruction
					// 2. The next Step() that executes it and counts its actual cycles
					// This ensures the provided instruction's cycles are counted correctly
					// and R is incremented exactly once during that instruction's M1 cycle
					return 0, true
				}
			}
			// Fallback: If no instruction provided, execute RST 38H
			z.push(z.PC)
			z.PC = 0x0038
			z.WZ = z.PC
			// Increment R for the interrupt acknowledge M1 cycle
			z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
			// Debug M1 trace
			if DEBUG_M1 && z.M1Hook != nil {
				z.M1Hook(0x0038, 0xFF, "IM0-fallback")
			}
			return IM1_CYCLES, true

		case 1:
			// Mode 1: RST 38H (fixed vector at 0x0038)
			z.push(z.PC)
			z.PC = 0x0038
			z.WZ = z.PC
			// Increment R for the interrupt acknowledge M1 cycle
			z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
			// Debug M1 trace
			if DEBUG_M1 && z.M1Hook != nil {
				z.M1Hook(0x0038, 0xFF, "IM1")
			}
			return IM1_CYCLES, true

		case 2:
			// Mode 2: Vectored interrupt
			// The interrupting device supplies the low byte of the vector
			z.push(z.PC)
			var vector uint8
			if ic, ok := z.IO.(InterruptController); ok {
				vector = ic.GetInterruptVector()
			} else {
				vector = 0xFF // Default if no controller
			}
			addr := uint16(z.I)<<8 | uint16(vector&0xFE) // Low bit forced to 0
			z.PC = z.readWord(addr)
			z.WZ = z.PC
			// Increment R for the interrupt acknowledge M1 cycle
			z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
			// Debug M1 trace
			if DEBUG_M1 && z.M1Hook != nil {
				z.M1Hook(z.PC, vector, "IM2")
			}
			return IM2_CYCLES, true
		}
	}

	return 0, false
}

// executeMode0Instruction executes a Mode 0 interrupt instruction from the buffer
func (z *Z80) executeMode0Instruction() int {
	if z.mode0Buffer == nil || z.mode0Index >= len(z.mode0Buffer) {
		// Clear the buffer if we're done
		z.mode0Buffer = nil
		z.mode0Index = 0
		z.mode0Active = false
		return 0 // Return 0 to avoid phantom cycles in safety net case
	}

	// Set mode0Active flag so fetchByte will read from buffer
	z.mode0Active = true

	// Get the first opcode (already in buffer at current index)
	opcode := z.mode0Buffer[z.mode0Index]
	z.mode0Index++

	// Increment R register for the Mode 0 instruction's M1 cycle (opcode fetch)
	// This is the only R increment for the entire instruction, regardless of length
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)

	// Debug M1 trace
	if DEBUG_M1 && z.M1Hook != nil {
		z.M1Hook(z.PC, opcode, "IM0")
	}

	// Execute the instruction (fetchByte will now read from buffer)
	// Since we're in Mode 0 context, fetchByte doesn't advance PC,
	// so non-branching instructions leave PC unchanged (which is correct)
	// Branching instructions (JP/CALL/RET) will modify PC as expected
	cycles := z.execute(opcode)

	// Clear buffer if we've executed all bytes
	if z.mode0Index >= len(z.mode0Buffer) {
		z.mode0Buffer = nil
		z.mode0Index = 0
		z.mode0Active = false
	}

	return cycles
}

// SetMode0Instruction allows setting the instruction to execute for Mode 0 interrupts
// This is primarily for testing purposes
func (z *Z80) SetMode0Instruction(instruction []uint8) {
	z.mode0Buffer = instruction
	z.mode0Index = 0
	z.mode0Active = false
}

// DataBusInterface is an optional interface for I/O implementations
// that support providing data bus values for Mode 0 interrupts
// DEPRECATED: Use InterruptController interface instead
type DataBusInterface interface {
	IOInterface
	GetDataBus() uint8
}

// getDataBus returns the value on the data bus during interrupt acknowledge
// DEPRECATED: This method is kept for backward compatibility
func (z *Z80) getDataBus() uint8 {
	// Check new InterruptController interface first
	if ic, ok := z.IO.(InterruptController); ok {
		inst := ic.GetMode0Instruction()
		if len(inst) > 0 {
			return inst[0] // Return first byte
		}
	}
	// Check old DataBusInterface for backward compatibility
	if dbIO, ok := z.IO.(DataBusInterface); ok {
		return dbIO.GetDataBus()
	}
	// Default to 0xFF (RST 38H) if not supported
	return 0xFF
}
