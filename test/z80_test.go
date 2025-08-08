package test

import (
	"testing"

	"github.com/ha1tch/zen80/io"
	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

// TestUndocumentedFlags tests the undocumented X and Y flags
func TestUndocumentedFlags(t *testing.T) {
	tests := []struct {
		name     string
		code     []uint8
		checkA   uint8
		checkF   uint8
		desc     string
	}{
		{
			"X/Y flags from result",
			[]uint8{
				0x3E, 0x28,  // LD A,28h (bit 3 and 5 set)
				0x87,        // ADD A,A
				0x76,        // HALT
			},
			0x50,
			z80.FlagY | z80.FlagPV, // Y flag (bit 5) set, plus overflow
			"X and Y flags should copy from result bits 3 and 5",
		},
		{
			"INC preserves carry",
			[]uint8{
				0x3E, 0xFF,  // LD A,FFh
				0x37,        // SCF (set carry)
				0x3C,        // INC A
				0x76,        // HALT
			},
			0x00,
			z80.FlagZ | z80.FlagH | z80.FlagC, // Zero, half-carry, and carry preserved
			"INC should preserve carry flag",
		},
		{
			"BIT instruction X/Y flags",
			[]uint8{
				0x21, 0x00, 0x80,  // LD HL,8000h
				0x3E, 0xA5,        // LD A,A5h
				0x36, 0x0F,        // LD (HL),0Fh
				0xCB, 0x7E,        // BIT 7,(HL)
				0x76,              // HALT
			},
			0xA5,
			z80.FlagZ | z80.FlagH | z80.FlagX, // Z set (bit 7 not set), H always set, X from high byte of address
			"BIT n,(HL) should set X/Y from high byte of address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			cpu := z80.New(mem, io)
			
			mem.Load(0x0000, tt.code)
			
			cycles := 0
			for !cpu.Halted && cycles < 1000 {
				c := cpu.Step()
				cycles += c
			}
			
			if cpu.A != tt.checkA {
				t.Errorf("A = %02X, want %02X", cpu.A, tt.checkA)
			}
			
			if (cpu.F & (z80.FlagX | z80.FlagY | z80.FlagZ | z80.FlagH | z80.FlagPV | z80.FlagC)) != tt.checkF {
				t.Errorf("Flags = %02X, want %02X (%s)", cpu.F, tt.checkF, tt.desc)
			}
		})
	}
}

// TestBlockInstructions tests complex block operations
func TestBlockInstructions(t *testing.T) {
	tests := []struct {
		name      string
		code      []uint8
		setupMem  map[uint16]uint8
		checkMem  map[uint16]uint8
		checkRegs map[string]uint16
	}{
		{
			"LDIR block copy",
			[]uint8{
				0x21, 0x00, 0x40,  // LD HL,4000h (source)
				0x11, 0x00, 0x50,  // LD DE,5000h (dest)
				0x01, 0x10, 0x00,  // LD BC,0010h (count)
				0xED, 0xB0,        // LDIR
				0x76,              // HALT
			},
			map[uint16]uint8{
				0x4000: 0x11, 0x4001: 0x22, 0x4002: 0x33, 0x4003: 0x44,
				0x4004: 0x55, 0x4005: 0x66, 0x4006: 0x77, 0x4007: 0x88,
				0x4008: 0x99, 0x4009: 0xAA, 0x400A: 0xBB, 0x400B: 0xCC,
				0x400C: 0xDD, 0x400D: 0xEE, 0x400E: 0xFF, 0x400F: 0x12,
			},
			map[uint16]uint8{
				0x5000: 0x11, 0x5001: 0x22, 0x5002: 0x33, 0x5003: 0x44,
				0x5004: 0x55, 0x5005: 0x66, 0x5006: 0x77, 0x5007: 0x88,
				0x5008: 0x99, 0x5009: 0xAA, 0x500A: 0xBB, 0x500B: 0xCC,
				0x500C: 0xDD, 0x500D: 0xEE, 0x500E: 0xFF, 0x500F: 0x12,
			},
			map[string]uint16{
				"HL": 0x4010, "DE": 0x5010, "BC": 0x0000,
			},
		},
		{
			"CPIR block search",
			[]uint8{
				0x21, 0x00, 0x40,  // LD HL,4000h
				0x01, 0x20, 0x00,  // LD BC,0020h
				0x3E, 0x55,        // LD A,55h (search for this)
				0xED, 0xB1,        // CPIR
				0x76,              // HALT
			},
			map[uint16]uint8{
				0x4000: 0x11, 0x4001: 0x22, 0x4002: 0x33, 0x4003: 0x44,
				0x4004: 0x55, // Found here!
			},
			nil,
			map[string]uint16{
				"HL": 0x4005, "BC": 0x001B, // BC decremented 5 times
			},
		},
		{
			"LDDR reverse copy",
			[]uint8{
				0x21, 0x0F, 0x40,  // LD HL,400Fh (source end)
				0x11, 0x1F, 0x40,  // LD DE,401Fh (dest end)
				0x01, 0x10, 0x00,  // LD BC,0010h (count)
				0xED, 0xB8,        // LDDR
				0x76,              // HALT
			},
			map[uint16]uint8{
				0x4000: 0x11, 0x4001: 0x22, 0x4002: 0x33, 0x4003: 0x44,
				0x4004: 0x55, 0x4005: 0x66, 0x4006: 0x77, 0x4007: 0x88,
				0x4008: 0x99, 0x4009: 0xAA, 0x400A: 0xBB, 0x400B: 0xCC,
				0x400C: 0xDD, 0x400D: 0xEE, 0x400E: 0xFF, 0x400F: 0x12,
			},
			map[uint16]uint8{
				0x4010: 0x11, 0x4011: 0x22, 0x4012: 0x33, 0x4013: 0x44,
				0x4014: 0x55, 0x4015: 0x66, 0x4016: 0x77, 0x4017: 0x88,
				0x4018: 0x99, 0x4019: 0xAA, 0x401A: 0xBB, 0x401B: 0xCC,
				0x401C: 0xDD, 0x401D: 0xEE, 0x401E: 0xFF, 0x401F: 0x12,
			},
			map[string]uint16{
				"HL": 0x3FFF, "DE": 0x400F, "BC": 0x0000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			cpu := z80.New(mem, io)
			
			// Setup memory
			for addr, val := range tt.setupMem {
				mem.Write(addr, val)
			}
			
			mem.Load(0x0000, tt.code)
			
			cycles := 0
			for !cpu.Halted && cycles < 10000 {
				c := cpu.Step()
				cycles += c
			}
			
			// Check memory
			for addr, expected := range tt.checkMem {
				actual := mem.Read(addr)
				if actual != expected {
					t.Errorf("Memory[%04X] = %02X, want %02X", addr, actual, expected)
				}
			}
			
			// Check registers
			for reg, expected := range tt.checkRegs {
				var actual uint16
				switch reg {
				case "HL": actual = cpu.HL()
				case "DE": actual = cpu.DE()
				case "BC": actual = cpu.BC()
				}
				if actual != expected {
					t.Errorf("%s = %04X, want %04X", reg, actual, expected)
				}
			}
		})
	}
}

// TestInterrupts tests interrupt handling
func TestInterrupts(t *testing.T) {
	tests := []struct {
		name     string
		code     []uint8
		setupISR []uint8
		triggerINT func(*z80.Z80, int)
		checkPC  uint16
		checkSP  uint16
	}{
		{
			"Mode 1 interrupt",
			[]uint8{
				0x31, 0x00, 0x80,  // LD SP,8000h
				0xED, 0x56,        // IM 1
				0xFB,              // EI
				0x00,              // NOP (interrupt happens after this)
				0x00,              // NOP
				0x00,              // NOP
				0x76,              // HALT (shouldn't reach)
			},
			[]uint8{ // ISR at 0038h
				0x3E, 0x42,  // LD A,42h
				0xED, 0x45,  // RETN
			},
			func(cpu *z80.Z80, cycle int) {
				if cycle > 20 { // After EI and NOP
					cpu.INT = true
				}
			},
			0x0038, // Should jump to ISR
			0x7FFE, // SP should be decremented by 2
		},
		{
			"Mode 2 interrupt",
			[]uint8{
				0x31, 0x00, 0x80,  // LD SP,8000h
				0x3E, 0x40,        // LD A,40h
				0xED, 0x47,        // LD I,A
				0xED, 0x5E,        // IM 2
				0xFB,              // EI
				0x00,              // NOP
				0x00,              // NOP
				0x76,              // HALT
			},
			[]uint8{ // ISR at 5000h
				0x3E, 0x99,  // LD A,99h
				0xED, 0x45,  // RETN
			},
			func(cpu *z80.Z80, cycle int) {
				if cycle > 30 {
					cpu.INT = true
					// In Mode 2, we'd need to set up vector table
					// For simplicity, we'll test that it tries to read from I:FF
				}
			},
			0x5000, // Should jump to vectored address
			0x7FFE,
		},
		{
			"NMI interrupt",
			[]uint8{
				0x31, 0x00, 0x80,  // LD SP,8000h
				0xF3,              // DI (NMI still works)
				0x00,              // NOP
				0x00,              // NOP
				0x76,              // HALT
			},
			[]uint8{ // NMI ISR at 0066h
				0x3E, 0x66,  // LD A,66h
				0xED, 0x45,  // RETN
			},
			func(cpu *z80.Z80, cycle int) {
				if cycle > 20 {
					cpu.NMI = true
				}
			},
			0x0066, // Should jump to NMI handler
			0x7FFE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			cpu := z80.New(mem, io)
			
			// Setup ISR
			if tt.name == "Mode 1 interrupt" {
				mem.Load(0x0038, tt.setupISR)
			} else if tt.name == "Mode 2 interrupt" {
				// Setup vector table
				mem.Write(0x40FF, 0x00) // Low byte of vector
				mem.Write(0x4100, 0x50) // High byte of vector
				mem.Load(0x5000, tt.setupISR)
			} else if tt.name == "NMI interrupt" {
				mem.Load(0x0066, tt.setupISR)
			}
			
			mem.Load(0x0000, tt.code)
			
			cycles := 0
			for cycles < 100 {
				if tt.triggerINT != nil {
					tt.triggerINT(cpu, cycles)
				}
				c := cpu.Step()
				cycles += c
				
				// Check if we jumped to ISR
				if cpu.PC == tt.checkPC {
					break
				}
			}
			
			if cpu.PC != tt.checkPC {
				t.Errorf("PC = %04X, want %04X (interrupt not handled)", cpu.PC, tt.checkPC)
			}
			
			if cpu.SP != tt.checkSP {
				t.Errorf("SP = %04X, want %04X (stack not properly updated)", cpu.SP, tt.checkSP)
			}
		})
	}
}

// TestIndexedInstructions tests IX/IY operations
func TestIndexedInstructions(t *testing.T) {
	tests := []struct {
		name     string
		code     []uint8
		checkMem map[uint16]uint8
		checkA   uint8
	}{
		{
			"IX indexed load/store",
			[]uint8{
				0xDD, 0x21, 0x00, 0x40,  // LD IX,4000h
				0x3E, 0x42,              // LD A,42h
				0xDD, 0x77, 0x05,        // LD (IX+5),A
				0xDD, 0x7E, 0x05,        // LD A,(IX+5)
				0x76,                    // HALT
			},
			map[uint16]uint8{
				0x4005: 0x42,
			},
			0x42,
		},
		{
			"IY indexed arithmetic",
			[]uint8{
				0xFD, 0x21, 0x00, 0x50,  // LD IY,5000h
				0x3E, 0x10,              // LD A,10h
				0xFD, 0x36, 0x03, 0x25,  // LD (IY+3),25h
				0xFD, 0x86, 0x03,        // ADD A,(IY+3)
				0x76,                    // HALT
			},
			map[uint16]uint8{
				0x5003: 0x25,
			},
			0x35,
		},
		{
			"DDCB bit operations",
			[]uint8{
				0xDD, 0x21, 0x00, 0x60,  // LD IX,6000h
				0xDD, 0x36, 0x02, 0xFF,  // LD (IX+2),FFh
				0xDD, 0xCB, 0x02, 0x86,  // RES 0,(IX+2)
				0xDD, 0x7E, 0x02,        // LD A,(IX+2)
				0x76,                    // HALT
			},
			map[uint16]uint8{
				0x6002: 0xFE,
			},
			0xFE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			cpu := z80.New(mem, io)
			
			mem.Load(0x0000, tt.code)
			
			cycles := 0
			for !cpu.Halted && cycles < 1000 {
				c := cpu.Step()
				cycles += c
			}
			
			// Check memory
			for addr, expected := range tt.checkMem {
				actual := mem.Read(addr)
				if actual != expected {
					t.Errorf("Memory[%04X] = %02X, want %02X", addr, actual, expected)
				}
			}
			
			if cpu.A != tt.checkA {
				t.Errorf("A = %02X, want %02X", cpu.A, tt.checkA)
			}
		})
	}
}

// TestDAA tests decimal adjust accumulator
func TestDAA(t *testing.T) {
	tests := []struct {
		name   string
		code   []uint8
		checkA uint8
		checkF uint8
	}{
		{
			"BCD addition",
			[]uint8{
				0x3E, 0x29,  // LD A,29h
				0x06, 0x13,  // LD B,13h
				0x80,        // ADD A,B (29h + 13h = 3Ch)
				0x27,        // DAA (should adjust to 42h)
				0x76,        // HALT
			},
			0x42,
			z80.FlagPV, // Parity flag
		},
		{
			"BCD subtraction",
			[]uint8{
				0x3E, 0x42,  // LD A,42h
				0x06, 0x13,  // LD B,13h
				0x90,        // SUB B (42h - 13h = 2Fh)
				0x27,        // DAA (should adjust to 29h)
				0x76,        // HALT
			},
			0x29,
			z80.FlagN, // Subtract flag remains
		},
		{
			"BCD with carry",
			[]uint8{
				0x3E, 0x99,  // LD A,99h
				0x06, 0x01,  // LD B,01h
				0x80,        // ADD A,B (99h + 01h = 9Ah)
				0x27,        // DAA (should adjust to 00h with carry)
				0x76,        // HALT
			},
			0x00,
			z80.FlagZ | z80.FlagC | z80.FlagPV, // Zero, Carry, Parity
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			cpu := z80.New(mem, io)
			
			mem.Load(0x0000, tt.code)
			
			cycles := 0
			for !cpu.Halted && cycles < 1000 {
				c := cpu.Step()
				cycles += c
			}
			
			if cpu.A != tt.checkA {
				t.Errorf("A = %02X, want %02X", cpu.A, tt.checkA)
			}
			
			relevantFlags := cpu.F & (z80.FlagZ | z80.FlagC | z80.FlagN | z80.FlagPV)
			if relevantFlags != tt.checkF {
				t.Errorf("Flags = %02X, want %02X", relevantFlags, tt.checkF)
			}
		})
	}
}

// TestStackOperations tests complex stack operations
func TestStackOperations(t *testing.T) {
	tests := []struct {
		name      string
		code      []uint8
		checkRegs map[string]uint16
		checkMem  map[uint16]uint8
	}{
		{
			"EX (SP),HL",
			[]uint8{
				0x31, 0x00, 0x80,        // LD SP,8000h
				0x21, 0x34, 0x12,        // LD HL,1234h
				0x11, 0x78, 0x56,        // LD DE,5678h
				0xD5,                    // PUSH DE
				0xE3,                    // EX (SP),HL
				0xE1,                    // POP HL
				0x76,                    // HALT
			},
			map[string]uint16{
				"HL": 0x1234, // HL gets what was pushed (DE value)
				"DE": 0x5678, // DE unchanged
			},
			map[uint16]uint8{
				0x7FFE: 0x34, // Stack should have HL value
				0x7FFF: 0x12,
			},
		},
		{
			"EX AF,AF' and EXX",
			[]uint8{
				0x3E, 0x11,              // LD A,11h
				0x01, 0x22, 0x33,        // LD BC,3322h
				0x11, 0x44, 0x55,        // LD DE,5544h
				0x21, 0x66, 0x77,        // LD HL,7766h
				0x08,                    // EX AF,AF'
				0xD9,                    // EXX
				0x3E, 0xAA,              // LD A,AAh
				0x01, 0xBB, 0xCC,        // LD BC,CCBBh
				0x11, 0xDD, 0xEE,        // LD DE,EEDDh
				0x21, 0xFF, 0x99,        // LD HL,99FFh
				0x08,                    // EX AF,AF'
				0xD9,                    // EXX
				0x76,                    // HALT
			},
			map[string]uint16{
				"A":  0x11,
				"BC": 0x3322,
				"DE": 0x5544,
				"HL": 0x7766,
			},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			cpu := z80.New(mem, io)
			
			mem.Load(0x0000, tt.code)
			
			cycles := 0
			for !cpu.Halted && cycles < 1000 {
				c := cpu.Step()
				cycles += c
			}
			
			// Check registers
			for reg, expected := range tt.checkRegs {
				var actual uint16
				switch reg {
				case "A":  actual = uint16(cpu.A)
				case "HL": actual = cpu.HL()
				case "DE": actual = cpu.DE()
				case "BC": actual = cpu.BC()
				}
				if actual != expected {
					t.Errorf("%s = %04X, want %04X", reg, actual, expected)
				}
			}
			
			// Check memory
			for addr, expected := range tt.checkMem {
				actual := mem.Read(addr)
				if actual != expected {
					t.Errorf("Memory[%04X] = %02X, want %02X", addr, actual, expected)
				}
			}
		})
	}
}

// TestEdgeCase tests edge cases and corner conditions
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		code   []uint8
		checkA uint8
		checkF uint8
		desc   string
	}{
		{
			"SCF/CCF flag behavior",
			[]uint8{
				0x3E, 0x00,  // LD A,00h
				0x37,        // SCF (set carry)
				0x3F,        // CCF (complement carry)
				0x3F,        // CCF (complement carry again)
				0x76,        // HALT
			},
			0x00,
			z80.FlagC, // Carry should be set
			"SCF sets carry, CCF complements it",
		},
		{
			"Overflow detection in ADD",
			[]uint8{
				0x3E, 0x7F,  // LD A,7Fh
				0x06, 0x01,  // LD B,01h
				0x80,        // ADD A,B (7F + 01 = 80, overflow)
				0x76,        // HALT
			},
			0x80,
			z80.FlagS | z80.FlagPV | z80.FlagH, // Sign, Overflow, Half-carry
			"Adding positive numbers resulting in negative should set overflow",
		},
		{
			"Zero flag with INC/DEC",
			[]uint8{
				0x3E, 0xFF,  // LD A,FFh
				0x3C,        // INC A (FF + 1 = 00)
				0x76,        // HALT
			},
			0x00,
			z80.FlagZ | z80.FlagH, // Zero and half-carry
			"INC from FF to 00 should set zero flag",
		},
		{
			"Parity flag",
			[]uint8{
				0x3E, 0x00,  // LD A,00h
				0xEE, 0x81,  // XOR 81h (result has 2 bits set)
				0x76,        // HALT
			},
			0x81,
			z80.FlagS | z80.FlagX, // Sign set, parity even (2 bits)
			"XOR should set parity flag based on result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			cpu := z80.New(mem, io)
			
			mem.Load(0x0000, tt.code)
			
			cycles := 0
			for !cpu.Halted && cycles < 1000 {
				c := cpu.Step()
				cycles += c
			}
			
			if cpu.A != tt.checkA {
				t.Errorf("A = %02X, want %02X", cpu.A, tt.checkA)
			}
			
			relevantFlags := cpu.F & (z80.FlagS | z80.FlagZ | z80.FlagH | z80.FlagPV | z80.FlagN | z80.FlagC | z80.FlagX | z80.FlagY)
			if relevantFlags != tt.checkF {
				t.Errorf("Flags = %02X, want %02X (%s)", relevantFlags, tt.checkF, tt.desc)
			}
		})
	}
}

// BenchmarkInstructionExecution benchmarks various instruction types
func BenchmarkInstructionExecution(b *testing.B) {
	benchmarks := []struct {
		name string
		code []uint8
	}{
		{
			"Simple arithmetic",
			[]uint8{
				0x3E, 0x01,  // LD A,1
				0x06, 0x02,  // LD B,2
				0x80,        // ADD A,B
				0x76,        // HALT
			},
		},
		{
			"Memory access",
			[]uint8{
				0x21, 0x00, 0x40,  // LD HL,4000h
				0x36, 0x42,        // LD (HL),42h
				0x7E,              // LD A,(HL)
				0x76,              // HALT
			},
		},
		{
			"Block operation",
			[]uint8{
				0x21, 0x00, 0x40,  // LD HL,4000h
				0x11, 0x00, 0x50,  // LD DE,5000h
				0x01, 0x10, 0x00,  // LD BC,10h
				0xED, 0xB0,        // LDIR
				0x76,              // HALT
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			mem := memory.NewRAM()
			io := io.NewNullIO()
			
			mem.Load(0x0000, bm.code)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cpu := z80.New(mem, io)
				for !cpu.Halted {
					cpu.Step()
				}
			}
		})
	}
}