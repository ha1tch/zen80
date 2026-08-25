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

	// Z80N (ZX Spectrum Next) extended instruction set. Off by default:
	// every ED-prefixed opcode this ISA defines occupies a byte value
	// that plain Z80 leaves undefined (acts as an 8-cycle NOP), so this
	// flag only ever adds behaviour -- with it false, dispatch is
	// byte-for-byte identical to a real classic Z80.
	Z80N bool

	// Debug hooks
	M1Hook func(pc uint16, opcode uint8, context string) // Called on M1 cycles when DEBUG_M1 is true

	// Memory interface
	Memory MemoryInterface

	// I/O interface
	IO IOInterface

	// FastMem and FastPort are an optional addition alongside Memory/IO
	// above, not a replacement for them -- Memory/IO stay exactly as they
	// are for every normal caller. When FastMem is non-nil, memRead/
	// memWrite (and FastPort for ioIn/ioOut) use it directly instead of
	// going through the Memory/IO interfaces, letting the Go compiler
	// inline a plain array access instead of an interface's indirect
	// call. Intended for a caller that needs to run a large, closed
	// number of instructions fast and correctly with no bank-switching
	// concerns (a full 64K flat address space) -- e.g. a tape-loading
	// fast path -- and is willing to reconcile FastMem/FastPort against
	// its own real memory model itself; zen80 does not manage that
	// reconciliation. nil (the default) is always a complete no-op:
	// every existing caller is completely unaffected.
	FastMem  *[65536]byte
	FastPort *[65536]byte

	// FastPortReadIn is an optional addition alongside FastPort: checked
	// first on every ioIn call while FastPort is active, before falling
	// back to the flat FastPort array. Exists because a real ULA-style
	// port (keyboard + tape EAR, combined dynamically from the port
	// address's high byte) can't be kept correct as a static array
	// without rewriting it across every matching address on every EAR
	// transition -- prohibitively expensive at the rate a loader toggles
	// it. A hook lets the caller compute such a value on the rare event
	// (an actual IN instruction) rather than the frequent one (an EAR
	// transition). Return ok=false to fall through to FastPort normally.
	// nil (the default) is always a complete no-op.
	FastPortReadIn func(port uint16) (value uint8, ok bool)

	// FastPortWriteOut is a further optional addition alongside FastPort:
	// called after every ioOut while FastPort is active (FastPort[port]
	// already updated by the time this fires). Exists for a caller whose
	// memory model has state driven by a specific port write -- a memory
	// paging register being the motivating case, where the flat FastMem
	// array has no way to represent banked memory on its own and needs
	// to be told, at the exact moment paging changes, to swap which
	// physical bank's data it currently holds. nil (the default) is
	// always a complete no-op.
	FastPortWriteOut func(port uint16, value uint8)

	// ContendedMemDelay is an optional hook: called on every memory
	// read and write with the address and the access's estimated
	// T-state position: the CPU's cycle count at the start of the
	// current instruction plus a within-instruction offset built from
	// per-access base costs (opcode fetch 4/5/6 per firstMCycleCost,
	// prefixed second fetch 4, other memory accesses 3, I/O 4) and
	// any contention delays already applied earlier in the same
	// instruction. Measured against real execution (Speedlock loader
	// workload), the residual position error -- from mid-instruction
	// internal cycles the base costs can't see -- bounds at roughly
	// 0.33% of total memory contention delay. Returns the number of
	// additional T-states the ULA would hold the CPU for at this
	// access; 0 for no delay. nil (the default) is always a complete
	// no-op.
	ContendedMemDelay func(addr uint16, cyclesBefore uint64) int

	// ContendedIODelay is the same mechanism as ContendedMemDelay, for
	// I/O port accesses (IN and OUT) rather than memory. nil (the
	// default) is always a complete no-op.
	ContendedIODelay func(port uint16, cyclesBefore uint64) int

	// pendingContention accumulates delay from ContendedMemDelay/
	// ContendedIODelay across all the accesses a single instruction
	// makes; finishStep folds it into that instruction's own cycle
	// count once, at the end, then resets it. Callers never see this
	// field directly.
	pendingContention int

	// Within-instruction access-position tracking for the contention
	// hooks: accessOffset is reset each Step and advanced by every
	// access's base cost plus any contention delay applied to it, so
	// later accesses in the same instruction are checked at their true
	// T-state positions rather than the instruction's start.
	// instrReadIndex and prefixPending drive the opcode-aware base
	// cost of the first one or two fetches; contentionActive caches
	// whether either hook is set so the bookkeeping is skipped
	// entirely when contention modeling is off.
	accessOffset     uint64
	instrReadIndex   int
	prefixPending    bool
	contentionActive bool

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
	z.accessOffset = 0
	z.instrReadIndex = 0
	z.prefixPending = false
	z.contentionActive = z.ContendedMemDelay != nil || z.ContendedIODelay != nil

	// Check if we're in the middle of executing a Mode 0 interrupt instruction
	if z.mode0Buffer != nil && z.mode0Index < len(z.mode0Buffer) {
		return z.finishStep(z.executeMode0Instruction())
	}

	// Handle interrupts
	if cycles, handled := z.handleInterrupts(); handled {
		return z.finishStep(cycles)
	}

	// If halted, just count cycles
	if z.Halted {
		return z.finishStep(4)
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

	return z.finishStep(cycles)
}

// finishStep folds any accumulated contention delay (from
// ContendedMemDelay/ContendedIODelay firing during this instruction's own
// memory/IO accesses) into its final cycle count, exactly once, then
// resets the accumulator for the next instruction.
func (z *Z80) finishStep(cycles int) int {
	cycles += z.pendingContention
	z.pendingContention = 0
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
	val := z.memRead(z.PC)
	z.PC++
	return val
}

func (z *Z80) fetchWord() uint16 {
	low := z.fetchByte()
	high := z.fetchByte()
	return uint16(high)<<8 | uint16(low)
}

func (z *Z80) readWord(addr uint16) uint16 {
	low := z.memRead(addr)
	high := z.memRead(addr + 1)
	return uint16(high)<<8 | uint16(low)
}

func (z *Z80) writeWord(addr uint16, val uint16) {
	z.memWrite(addr, uint8(val))
	z.memWrite(addr+1, uint8(val>>8))
}

// Stack operations
func (z *Z80) push(val uint16) {
	z.SP--
	z.memWrite(z.SP, uint8(val>>8))
	z.SP--
	z.memWrite(z.SP, uint8(val))
}

func (z *Z80) pop() uint16 {
	low := z.memRead(z.SP)
	z.SP++
	high := z.memRead(z.SP)
	z.SP++
	return uint16(high)<<8 | uint16(low)
}

// memRead/memWrite/ioIn/ioOut are the single indirection point for every
// memory/IO access in this package (see FastMem/FastPort's own doc
// comment on the Z80 struct for why they exist). Every access in this
// package goes through these four functions rather than z.Memory/z.IO
// directly, so FastMem/FastPort apply uniformly regardless of which
// instruction is doing the accessing.
// firstMCycleCost gives the T-state length of each unprefixed opcode's
// first M-cycle (the opcode fetch): 4 for most, 5 for DJNZ / PUSH qq /
// RST n / RET cc / LD SP,HL, 6 for INC/DEC dd -- per the WoS FAQ
// instruction breakdown table. Used only for contention position
// tracking; instruction cycle totals are unaffected.
var firstMCycleCost [256]uint8

func init() {
	for i := range firstMCycleCost {
		firstMCycleCost[i] = 4
	}
	for _, op := range []uint8{0x10, 0xC5, 0xD5, 0xE5, 0xF5,
		0xC7, 0xCF, 0xD7, 0xDF, 0xE7, 0xEF, 0xF7, 0xFF,
		0xC0, 0xC8, 0xD0, 0xD8, 0xE0, 0xE8, 0xF0, 0xF8, 0xF9} {
		firstMCycleCost[op] = 5
	}
	for _, op := range []uint8{0x03, 0x0B, 0x13, 0x1B, 0x23, 0x2B, 0x33, 0x3B} {
		firstMCycleCost[op] = 6
	}
}

func (z *Z80) memRead(addr uint16) uint8 {
	if z.ContendedMemDelay != nil {
		d := z.ContendedMemDelay(addr, z.Cycles+z.accessOffset)
		z.pendingContention += d
		z.accessOffset += uint64(d)
	}
	var val uint8
	if z.FastMem != nil {
		val = z.FastMem[addr]
	} else {
		val = z.Memory.Read(addr)
	}
	if z.contentionActive {
		base := uint64(3)
		if z.instrReadIndex == 0 {
			// First read of the instruction is the opcode fetch; its
			// length depends on the opcode just read.
			base = uint64(firstMCycleCost[val])
			if val == 0xCB || val == 0xDD || val == 0xED || val == 0xFD {
				z.prefixPending = true
			}
		} else if z.instrReadIndex == 1 && z.prefixPending {
			base = 4 // second fetch of a prefixed instruction
		}
		z.instrReadIndex++
		z.accessOffset += base
	}
	return val
}

func (z *Z80) memWrite(addr uint16, val uint8) {
	if z.ContendedMemDelay != nil {
		d := z.ContendedMemDelay(addr, z.Cycles+z.accessOffset)
		z.pendingContention += d
		z.accessOffset += uint64(d)
	}
	if z.contentionActive {
		z.accessOffset += 3
	}
	if z.FastMem != nil {
		z.FastMem[addr] = val
		return
	}
	z.Memory.Write(addr, val)
}

func (z *Z80) ioIn(port uint16) uint8 {
	if z.ContendedIODelay != nil {
		d := z.ContendedIODelay(port, z.Cycles+z.accessOffset)
		z.pendingContention += d
		z.accessOffset += uint64(d)
	}
	if z.contentionActive {
		z.accessOffset += 4
	}
	if z.FastPort != nil {
		if z.FastPortReadIn != nil {
			if v, ok := z.FastPortReadIn(port); ok {
				return v
			}
		}
		return z.FastPort[port]
	}
	return z.IO.In(port)
}

func (z *Z80) ioOut(port uint16, val uint8) {
	if z.ContendedIODelay != nil {
		d := z.ContendedIODelay(port, z.Cycles+z.accessOffset)
		z.pendingContention += d
		z.accessOffset += uint64(d)
	}
	if z.contentionActive {
		z.accessOffset += 4
	}
	if z.FastPort != nil {
		z.FastPort[port] = val
		if z.FastPortWriteOut != nil {
			z.FastPortWriteOut(port, val)
		}
		return
	}
	z.IO.Out(port, val)
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
