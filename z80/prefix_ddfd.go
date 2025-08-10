package z80

// executeDD handles DD-prefixed instructions (IX operations)
func (z *Z80) executeDD() int {
	opcode := z.fetchByte()
	// Increment R for the post-prefix opcode fetch (M1)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	// Debug M1 trace for post-prefix fetch
	if DEBUG_M1 && z.M1Hook != nil {
		z.M1Hook(z.PC-1, opcode, "post-DD")
	}

	
	// Handle special cases
	switch opcode {
	case 0xCB: // DDCB prefix - IX bit operations
		return z.executeDDCB()
	case 0xDD: // Another DD prefix - acts as NOP
		return 4
	case 0xED: // ED after DD - DD is ignored
		return z.executeED() + 4
	case 0xFD: // FD after DD - DD is ignored
		return z.executeFD() + 4
	}
	
	// Check for undocumented IXH/IXL register access opcodes
	if z.handleIXHIXL(opcode) {
		// Instruction was handled as IXH/IXL operation
		return z.getIXHIXLCycles(opcode)
	}
	
	// For most instructions, DD prefix replaces HL with IX
	// We need to decode the opcode to see if it involves HL
	x := opcode >> 6
	y := (opcode >> 3) & 7
	z_val := opcode & 7
	
	// Check for instructions that use HL
	if involvesHL(opcode) {
		if opcode == 0x36 { // LD (IX+d),n
			d := int8(z.fetchByte())
			n := z.fetchByte()
			addr := uint16(int32(z.IX()) + int32(d))
			z.Memory.Write(addr, n)
			z.WZ = addr
			return 19
		} else if (x == 1 && (y == 6 || z_val == 6)) || // LD r,(IX+d) or LD (IX+d),r
			(x == 2 && z_val == 6) { // ALU operations with (IX+d)
			d := int8(z.fetchByte())
			addr := uint16(int32(z.IX()) + int32(d))
			z.WZ = addr
			
			if x == 1 {
				// LD r,(IX+d) or LD (IX+d),r
				if y == 6 {
					// LD (IX+d),r
					val := *z.getRegister8(z_val)
					z.Memory.Write(addr, val)
				} else if z_val == 6 {
					// LD r,(IX+d)
					val := z.Memory.Read(addr)
					*z.getRegister8(y) = val
				}
				return 19
			} else if x == 2 {
				// ALU operation with (IX+d)
				val := z.Memory.Read(addr)
				switch y {
				case 0: z.add8(val)
				case 1: z.adc8(val)
				case 2: z.sub8(val)
				case 3: z.sbc8(val)
				case 4: z.and8(val)
				case 5: z.xor8(val)
				case 6: z.or8(val)
				case 7: z.cp8(val)
				}
				return 19
			}
		} else if opcode == 0x34 { // INC (IX+d)
			d := int8(z.fetchByte())
			addr := uint16(int32(z.IX()) + int32(d))
			val := z.Memory.Read(addr)
			z.Memory.Write(addr, z.inc8(val))
			z.WZ = addr
			return 23
		} else if opcode == 0x35 { // DEC (IX+d)
			d := int8(z.fetchByte())
			addr := uint16(int32(z.IX()) + int32(d))
			val := z.Memory.Read(addr)
			z.Memory.Write(addr, z.dec8(val))
			z.WZ = addr
			return 23
		}
	}
	
	// Execute instruction with IX substituted for HL
	return z.executeDDFDInstruction(opcode, true) + 4
}

// executeFD handles FD-prefixed instructions (IY operations)
func (z *Z80) executeFD() int {
	opcode := z.fetchByte()
	// Increment R for the post-prefix opcode fetch (M1)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	// Debug M1 trace for post-prefix fetch
	if DEBUG_M1 && z.M1Hook != nil {
		z.M1Hook(z.PC-1, opcode, "post-FD")
	}

	
	// Handle special cases
	switch opcode {
	case 0xCB: // FDCB prefix - IY bit operations
		return z.executeFDCB()
	case 0xDD: // DD after FD - FD is ignored
		return z.executeDD() + 4
	case 0xED: // ED after FD - FD is ignored
		return z.executeED() + 4
	case 0xFD: // Another FD prefix - acts as NOP
		return 4
	}
	
	// Check for undocumented IYH/IYL register access opcodes
	if z.handleIYHIYL(opcode) {
		// Instruction was handled as IYH/IYL operation
		return z.getIYHIYLCycles(opcode)
	}
	
	// Similar logic to DD but with IY
	x := opcode >> 6
	y := (opcode >> 3) & 7
	z_val := opcode & 7
	
	if involvesHL(opcode) {
		if opcode == 0x36 { // LD (IY+d),n
			d := int8(z.fetchByte())
			n := z.fetchByte()
			addr := uint16(int32(z.IY()) + int32(d))
			z.Memory.Write(addr, n)
			z.WZ = addr
			return 19
		} else if (x == 1 && (y == 6 || z_val == 6)) ||
			(x == 2 && z_val == 6) {
			d := int8(z.fetchByte())
			addr := uint16(int32(z.IY()) + int32(d))
			z.WZ = addr
			
			if x == 1 {
				if y == 6 {
					val := *z.getRegister8(z_val)
					z.Memory.Write(addr, val)
				} else if z_val == 6 {
					val := z.Memory.Read(addr)
					*z.getRegister8(y) = val
				}
				return 19
			} else if x == 2 {
				val := z.Memory.Read(addr)
				switch y {
				case 0: z.add8(val)
				case 1: z.adc8(val)
				case 2: z.sub8(val)
				case 3: z.sbc8(val)
				case 4: z.and8(val)
				case 5: z.xor8(val)
				case 6: z.or8(val)
				case 7: z.cp8(val)
				}
				return 19
			}
		} else if opcode == 0x34 { // INC (IY+d)
			d := int8(z.fetchByte())
			addr := uint16(int32(z.IY()) + int32(d))
			val := z.Memory.Read(addr)
			z.Memory.Write(addr, z.inc8(val))
			z.WZ = addr
			return 23
		} else if opcode == 0x35 { // DEC (IY+d)
			d := int8(z.fetchByte())
			addr := uint16(int32(z.IY()) + int32(d))
			val := z.Memory.Read(addr)
			z.Memory.Write(addr, z.dec8(val))
			z.WZ = addr
			return 23
		}
	}
	
	return z.executeDDFDInstruction(opcode, false) + 4
}

// handleIXHIXL handles undocumented opcodes that access IXH and IXL as separate registers
func (z *Z80) handleIXHIXL(opcode uint8) bool {
	x := opcode >> 6
	y := (opcode >> 3) & 7
	z_val := opcode & 7
	
	// Check if this is an opcode that should access IXH/IXL
	// These are instructions that normally access H and L (indices 4 and 5)
	
	// LD r,r' instructions (0x40-0x7F except 0x76)
	if x == 1 && opcode != 0x76 {
		// Check if source or dest is H (4) or L (5) or (HL) (6)
		// We only handle H and L, not (HL) which becomes (IX+d)
		if (y == 4 || y == 5) && z_val != 6 {
			// LD IXH/IXL,r
			val := *z.getRegisterForIX(z_val)
			if y == 4 {
				z.IXH = val
			} else {
				z.IXL = val
			}
			return true
		} else if (z_val == 4 || z_val == 5) && y != 6 {
			// LD r,IXH/IXL
			var val uint8
			if z_val == 4 {
				val = z.IXH
			} else {
				val = z.IXL
			}
			*z.getRegisterForIX(y) = val
			return true
		} else if (y == 4 || y == 5) && (z_val == 4 || z_val == 5) {
			// LD IXH,IXL or LD IXL,IXH
			if y == 4 && z_val == 5 {
				z.IXH = z.IXL
			} else if y == 5 && z_val == 4 {
				z.IXL = z.IXH
			} else if y == 4 && z_val == 4 {
				// LD IXH,IXH - NOP effectively
			} else if y == 5 && z_val == 5 {
				// LD IXL,IXL - NOP effectively
			}
			return true
		}
	}
	
	// ALU operations with IXH/IXL (0x80-0xBF)
	if x == 2 && (z_val == 4 || z_val == 5) {
		var val uint8
		if z_val == 4 {
			val = z.IXH
		} else {
			val = z.IXL
		}
		
		switch y {
		case 0: z.add8(val)  // ADD A,IXH/IXL
		case 1: z.adc8(val)  // ADC A,IXH/IXL
		case 2: z.sub8(val)  // SUB IXH/IXL
		case 3: z.sbc8(val)  // SBC A,IXH/IXL
		case 4: z.and8(val)  // AND IXH/IXL
		case 5: z.xor8(val)  // XOR IXH/IXL
		case 6: z.or8(val)   // OR IXH/IXL
		case 7: z.cp8(val)   // CP IXH/IXL
		}
		return true
	}
	
	// INC/DEC IXH/IXL
	if opcode == 0x24 { // INC IXH
		z.IXH = z.inc8(z.IXH)
		return true
	} else if opcode == 0x25 { // DEC IXH
		z.IXH = z.dec8(z.IXH)
		return true
	} else if opcode == 0x2C { // INC IXL
		z.IXL = z.inc8(z.IXL)
		return true
	} else if opcode == 0x2D { // DEC IXL
		z.IXL = z.dec8(z.IXL)
		return true
	}
	
	// LD IXH/IXL,n
	if opcode == 0x26 { // LD IXH,n
		z.IXH = z.fetchByte()
		return true
	} else if opcode == 0x2E { // LD IXL,n
		z.IXL = z.fetchByte()
		return true
	}
	
	return false
}

// handleIYHIYL handles undocumented opcodes that access IYH and IYL as separate registers
func (z *Z80) handleIYHIYL(opcode uint8) bool {
	x := opcode >> 6
	y := (opcode >> 3) & 7
	z_val := opcode & 7
	
	// LD r,r' instructions (0x40-0x7F except 0x76)
	if x == 1 && opcode != 0x76 {
		if (y == 4 || y == 5) && z_val != 6 {
			// LD IYH/IYL,r
			val := *z.getRegisterForIY(z_val)
			if y == 4 {
				z.IYH = val
			} else {
				z.IYL = val
			}
			return true
		} else if (z_val == 4 || z_val == 5) && y != 6 {
			// LD r,IYH/IYL
			var val uint8
			if z_val == 4 {
				val = z.IYH
			} else {
				val = z.IYL
			}
			*z.getRegisterForIY(y) = val
			return true
		} else if (y == 4 || y == 5) && (z_val == 4 || z_val == 5) {
			// LD IYH,IYL or LD IYL,IYH
			if y == 4 && z_val == 5 {
				z.IYH = z.IYL
			} else if y == 5 && z_val == 4 {
				z.IYL = z.IYH
			}
			// LD IYH,IYH and LD IYL,IYL are effectively NOPs
			return true
		}
	}
	
	// ALU operations with IYH/IYL (0x80-0xBF)
	if x == 2 && (z_val == 4 || z_val == 5) {
		var val uint8
		if z_val == 4 {
			val = z.IYH
		} else {
			val = z.IYL
		}
		
		switch y {
		case 0: z.add8(val)
		case 1: z.adc8(val)
		case 2: z.sub8(val)
		case 3: z.sbc8(val)
		case 4: z.and8(val)
		case 5: z.xor8(val)
		case 6: z.or8(val)
		case 7: z.cp8(val)
		}
		return true
	}
	
	// INC/DEC IYH/IYL
	if opcode == 0x24 { // INC IYH
		z.IYH = z.inc8(z.IYH)
		return true
	} else if opcode == 0x25 { // DEC IYH
		z.IYH = z.dec8(z.IYH)
		return true
	} else if opcode == 0x2C { // INC IYL
		z.IYL = z.inc8(z.IYL)
		return true
	} else if opcode == 0x2D { // DEC IYL
		z.IYL = z.dec8(z.IYL)
		return true
	}
	
	// LD IYH/IYL,n
	if opcode == 0x26 { // LD IYH,n
		z.IYH = z.fetchByte()
		return true
	} else if opcode == 0x2E { // LD IYL,n
		z.IYL = z.fetchByte()
		return true
	}
	
	return false
}

// getRegisterForIX returns a pointer to a register, treating H and L as IXH and IXL
func (z *Z80) getRegisterForIX(index uint8) *uint8 {
	switch index {
	case 0: return &z.B
	case 1: return &z.C
	case 2: return &z.D
	case 3: return &z.E
	case 4: return &z.IXH  // H becomes IXH
	case 5: return &z.IXL  // L becomes IXL
	case 7: return &z.A
	default: return &z.A
	}
}

// getRegisterForIY returns a pointer to a register, treating H and L as IYH and IYL
func (z *Z80) getRegisterForIY(index uint8) *uint8 {
	switch index {
	case 0: return &z.B
	case 1: return &z.C
	case 2: return &z.D
	case 3: return &z.E
	case 4: return &z.IYH  // H becomes IYH
	case 5: return &z.IYL  // L becomes IYL
	case 7: return &z.A
	default: return &z.A
	}
}

// getIXHIXLCycles returns the cycle count for IXH/IXL operations
func (z *Z80) getIXHIXLCycles(opcode uint8) int {
	x := opcode >> 6
	
	// LD r,r' instructions
	if x == 1 {
		return 8  // DD prefix (4) + instruction (4)
	}
	
	// ALU operations
	if x == 2 {
		return 8  // DD prefix (4) + instruction (4)
	}
	
	// INC/DEC
	if opcode == 0x24 || opcode == 0x25 || opcode == 0x2C || opcode == 0x2D {
		return 8  // DD prefix (4) + instruction (4)
	}
	
	// LD IXH/IXL,n
	if opcode == 0x26 || opcode == 0x2E {
		return 11  // DD prefix (4) + instruction (7)
	}
	
	return 8  // Default
}

// getIYHIYLCycles returns the cycle count for IYH/IYL operations
func (z *Z80) getIYHIYLCycles(opcode uint8) int {
	// Same timing as IXH/IXL
	return z.getIXHIXLCycles(opcode)
}

// executeDDFDInstruction executes an instruction with IX/IY substituted for HL
func (z *Z80) executeDDFDInstruction(opcode uint8, useIX bool) int {
	// Save HL and substitute IX or IY
	savedHL := z.HL()
	if useIX {
		z.SetHL(z.IX())
	} else {
		z.SetHL(z.IY())
	}
	
	// Execute the instruction
	cycles := z.execute(opcode)
	
	// Store result back and restore HL
	if useIX {
		z.SetIX(z.HL())
	} else {
		z.SetIY(z.HL())
	}
	z.SetHL(savedHL)
	
	return cycles
}

// executeDDCB handles DDCB-prefixed instructions (IX bit operations)
func (z *Z80) executeDDCB() int {
	d := int8(z.fetchByte())
	opcode := z.fetchByte()
	
	// IMPORTANT: Do NOT increment R here!
	// The displacement and sub-opcode are NOT fetched with M1 cycles
	// R was already incremented twice: once for DD, once for CB
	// That's the correct total for DDCB instructions
	
	addr := uint16(int32(z.IX()) + int32(d))
	z.WZ = addr
	
	return z.executeDDFDCBOperation(opcode, addr)
}

// executeFDCB handles FDCB-prefixed instructions (IY bit operations)
func (z *Z80) executeFDCB() int {
	d := int8(z.fetchByte())
	opcode := z.fetchByte()
	
	// IMPORTANT: Do NOT increment R here!
	// The displacement and sub-opcode are NOT fetched with M1 cycles
	// R was already incremented twice: once for FD, once for CB
	// That's the correct total for FDCB instructions
	
	addr := uint16(int32(z.IY()) + int32(d))
	z.WZ = addr
	
	return z.executeDDFDCBOperation(opcode, addr)
}

// executeDDFDCBOperation executes a DDCB/FDCB bit operation
func (z *Z80) executeDDFDCBOperation(opcode uint8, addr uint16) int {
	x := opcode >> 6
	y := (opcode >> 3) & 7
	z_val := opcode & 7
	
	val := z.Memory.Read(addr)
	
	switch x {
	case 0: // Rotation/shift operations
		switch y {
		case 0: val = z.rlc8(val)
		case 1: val = z.rrc8(val)
		case 2: val = z.rl8(val)
		case 3: val = z.rr8(val)
		case 4: val = z.sla8(val)
		case 5: val = z.sra8(val)
		case 6: val = z.sll8(val)
		case 7: val = z.srl8(val)
		}
		z.Memory.Write(addr, val)
		// Undocumented: also copy result to register
		if z_val != 6 {
			*z.getRegister8(z_val) = val
		}
		return 23
		
	case 1: // BIT y,(IX/IY+d)
		z.bit(val, y)
		// For BIT with DDCB/FDCB, flags use WZ high byte
		hi := uint8(z.WZ >> 8)
		z.setFlag(FlagX, hi&FlagX != 0)
		z.setFlag(FlagY, hi&FlagY != 0)
		return 20  // BIT operations are 20 cycles, not 23
		
	case 2: // RES y,(IX/IY+d)
		val &^= (1 << y)
		z.Memory.Write(addr, val)
		// Undocumented: also copy result to register
		if z_val != 6 {
			*z.getRegister8(z_val) = val
		}
		return 23
		
	case 3: // SET y,(IX/IY+d)
		val |= (1 << y)
		z.Memory.Write(addr, val)
		// Undocumented: also copy result to register
		if z_val != 6 {
			*z.getRegister8(z_val) = val
		}
		return 23
	}
	
	return 23  // Should never reach here, but safe default
}

// involvesHL checks if an opcode involves the HL register
func involvesHL(opcode uint8) bool {
	// Check for instructions that use HL or (HL)
	x := opcode >> 6
	y := (opcode >> 3) & 7
	z := opcode & 7
	
	// Special cases
	if opcode == 0x36 || // LD (HL),n
		opcode == 0x34 || // INC (HL)
		opcode == 0x35 || // DEC (HL)
		opcode == 0xE9 || // JP (HL) - returns true but doesn't get displaced (uses HL substitution)
		opcode == 0xF9 { // LD SP,HL
		return true
	}
	
	// LD instructions involving (HL)
	if x == 1 && (y == 6 || z == 6) {
		return true
	}
	
	// ALU operations with (HL)
	if x == 2 && z == 6 {
		return true
	}
	
	// Note: ADD HL,rp and other HL operations are handled by
	// the save/restore mechanism in executeDDFDInstruction
	
	return false
}