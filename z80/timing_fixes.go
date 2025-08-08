package z80

// Timing tables for accurate cycle counting
// Based on documented Z80 timings

// Standard instruction timings (for verification)
var baseTimings = map[uint8]int{
	0x00: 4,   // NOP
	0x01: 10,  // LD BC,nn
	0x02: 7,   // LD (BC),A
	0x03: 6,   // INC BC
	0x04: 4,   // INC B
	0x05: 4,   // DEC B
	0x06: 7,   // LD B,n
	0x07: 4,   // RLCA
	0x08: 4,   // EX AF,AF'
	0x09: 11,  // ADD HL,BC
	0x0A: 7,   // LD A,(BC)
	0x0B: 6,   // DEC BC
	0x0C: 4,   // INC C
	0x0D: 4,   // DEC C
	0x0E: 7,   // LD C,n
	0x0F: 4,   // RRCA
	0x10: -1,  // DJNZ d (8/13)
	0x18: 12,  // JR d
	0x20: -1,  // JR NZ,d (7/12)
	0x28: -1,  // JR Z,d (7/12)
	0x30: -1,  // JR NC,d (7/12)
	0x38: -1,  // JR C,d (7/12)
	0x76: 4,   // HALT
	0xC3: 10,  // JP nn
	0xC9: 10,  // RET
	0xCD: 17,  // CALL nn
	0xCB: 4,   // CB prefix (plus CB instruction time)
	0xDD: 4,   // DD prefix (plus next instruction time)
	0xED: 4,   // ED prefix (plus ED instruction time)
	0xFD: 4,   // FD prefix (plus next instruction time)
}

// ED instruction timings
var edTimings = map[uint8]int{
	0x40: 12,  // IN B,(C)
	0x41: 12,  // OUT (C),B
	0x42: 15,  // SBC HL,BC
	0x43: 20,  // LD (nn),BC
	0x44: 8,   // NEG
	0x45: 14,  // RETN
	0x46: 8,   // IM 0
	0x47: 9,   // LD I,A
	0x48: 12,  // IN C,(C)
	0x49: 12,  // OUT (C),C
	0x4A: 15,  // ADC HL,BC
	0x4B: 20,  // LD BC,(nn)
	0x4D: 14,  // RETI
	0x4F: 9,   // LD R,A
	0x52: 15,  // SBC HL,DE
	0x53: 20,  // LD (nn),DE
	0x56: 8,   // IM 1
	0x57: 9,   // LD A,I
	0x5A: 15,  // ADC HL,DE
	0x5B: 20,  // LD DE,(nn)
	0x5E: 8,   // IM 2
	0x5F: 9,   // LD A,R
	0x62: 15,  // SBC HL,HL
	0x63: 20,  // LD (nn),HL
	0x67: 18,  // RRD
	0x6A: 15,  // ADC HL,HL
	0x6B: 20,  // LD HL,(nn)
	0x6F: 18,  // RLD
	0x72: 15,  // SBC HL,SP
	0x73: 20,  // LD (nn),SP
	0x78: 12,  // IN A,(C)
	0x79: 12,  // OUT (C),A
	0x7A: 15,  // ADC HL,SP
	0x7B: 20,  // LD SP,(nn)
	0xA0: 16,  // LDI
	0xA1: 16,  // CPI
	0xA2: 16,  // INI
	0xA3: 16,  // OUTI
	0xA8: 16,  // LDD
	0xA9: 16,  // CPD
	0xAA: 16,  // IND
	0xAB: 16,  // OUTD
	0xB0: -1,  // LDIR (21/16)
	0xB1: -1,  // CPIR (21/16)
	0xB2: -1,  // INIR (21/16)
	0xB3: -1,  // OTIR (21/16)
	0xB8: -1,  // LDDR (21/16)
	0xB9: -1,  // CPDR (21/16)
	0xBA: -1,  // INDR (21/16)
	0xBB: -1,  // OTDR (21/16)
}

// GetInstructionCycles returns the cycle count for an instruction
// This can be used to verify our implementation
func GetInstructionCycles(opcode uint8, prefix uint8, taken bool) int {
	if prefix == 0xED {
		if cycles, ok := edTimings[opcode]; ok {
			if cycles == -1 {
				// Block instruction - depends on whether it repeats
				if taken {
					return 21 // Repeating
				}
				return 16 // Not repeating
			}
			return cycles
		}
		return 8 // Unknown ED instruction
	}
	
	if prefix == 0xCB {
		// CB instructions
		if (opcode & 0x07) == 6 {
			// (HL) operations
			if (opcode & 0xC0) == 0x40 {
				return 12 // BIT n,(HL)
			}
			return 15 // Other (HL) operations
		}
		return 8 // Register operations
	}
	
	if prefix == 0xDD || prefix == 0xFD {
		// IX/IY operations add cycles
		switch opcode {
		case 0x34: return 23  // INC (IX+d)
		case 0x35: return 23  // DEC (IX+d)
		case 0x36: return 19  // LD (IX+d),n
		case 0x46, 0x4E, 0x56, 0x5E, 0x66, 0x6E, 0x7E:
			return 19  // LD r,(IX+d)
		case 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x77:
			return 19  // LD (IX+d),r
		case 0x86: return 19  // ADD A,(IX+d)
		case 0x8E: return 19  // ADC A,(IX+d)
		case 0x96: return 19  // SUB (IX+d)
		case 0x9E: return 19  // SBC A,(IX+d)
		case 0xA6: return 19  // AND (IX+d)
		case 0xAE: return 19  // XOR (IX+d)
		case 0xB6: return 19  // OR (IX+d)
		case 0xBE: return 19  // CP (IX+d)
		case 0xCB: return 0   // DDCB/FDCB handled separately
		default:
			// Other instructions execute normally with prefix overhead
			if cycles, ok := baseTimings[opcode]; ok && cycles != -1 {
				return cycles + 4
			}
		}
	}
	
	// Base instruction
	if cycles, ok := baseTimings[opcode]; ok {
		if cycles == -1 {
			// Conditional instruction
			switch opcode {
			case 0x10: // DJNZ
				if taken {
					return 13
				}
				return 8
			case 0x20, 0x28, 0x30, 0x38: // JR cc,d
				if taken {
					return 12
				}
				return 7
			}
		}
		return cycles
	}
	
	// Estimate based on pattern
	x := opcode >> 6
	y := (opcode >> 3) & 7
	z := opcode & 7
	
	switch x {
	case 0:
		return 4 // Most are 4 cycles
	case 1:
		if y == 6 || z == 6 {
			return 7 // LD r,(HL) or LD (HL),r
		}
		return 4 // LD r,r'
	case 2:
		if z == 6 {
			return 7 // ALU with (HL)
		}
		return 4 // ALU with register
	case 3:
		// Complex instructions
		switch z {
		case 0: // RET cc
			if taken {
				return 11
			}
			return 5
		case 1:
			if (opcode & 0x0F) == 0x09 {
				return 10 // RET
			}
			return 10 // POP
		case 2: // JP cc,nn
			return 10
		case 3:
			switch opcode {
			case 0xC3: return 10  // JP nn
			case 0xE9: return 4   // JP (HL)
			case 0xF9: return 6   // LD SP,HL
			default: return 11    // OUT/IN
			}
		case 4: // CALL cc,nn
			if taken {
				return 17
			}
			return 10
		case 5:
			if (opcode & 0x0F) == 0x0D {
				return 17 // CALL nn
			}
			return 11 // PUSH
		case 6:
			return 7 // ALU n
		case 7:
			return 11 // RST
		}
	}
	
	return 4 // Default fallback
}

// VerifyInstructionTiming checks if our emulator's timing matches expected
func (z *Z80) VerifyInstructionTiming(opcode uint8, prefix uint8, actualCycles int) bool {
	// For conditional instructions, we'd need to know if branch was taken
	// This is a simplified check
	expected := GetInstructionCycles(opcode, prefix, true)
	expectedNotTaken := GetInstructionCycles(opcode, prefix, false)
	
	return actualCycles == expected || actualCycles == expectedNotTaken
}