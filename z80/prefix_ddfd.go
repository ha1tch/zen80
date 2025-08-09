package z80

// executeDD handles DD-prefixed instructions (IX operations)
func (z *Z80) executeDD() int {
	opcode := z.fetchByte()
	// Increment R for the post-prefix opcode fetch (M1)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	
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
	
	// Increment R for CB opcode fetch
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	// Increment R for the post-prefix opcode fetch (M1)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	addr := uint16(int32(z.IX()) + int32(d))
	z.WZ = addr
	
	return z.executeDDFDCBOperation(opcode, addr)
}

// executeFDCB handles FDCB-prefixed instructions (IY bit operations)
func (z *Z80) executeFDCB() int {
	d := int8(z.fetchByte())
	opcode := z.fetchByte()
	
	// Increment R for CB opcode fetch
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	// Increment R for the post-prefix opcode fetch (M1)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
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
		
	case 1: // BIT y,(IX/IY+d)
		z.bit(val, y)
		// For BIT with DDCB/FDCB, flags use WZ high byte
		hi := uint8(z.WZ >> 8)
		z.setFlag(FlagX, hi&FlagX != 0)
		z.setFlag(FlagY, hi&FlagY != 0)
		
	case 2: // RES y,(IX/IY+d)
		val &^= (1 << y)
		z.Memory.Write(addr, val)
		// Undocumented: also copy result to register
		if z_val != 6 {
			*z.getRegister8(z_val) = val
		}
		
	case 3: // SET y,(IX/IY+d)
		val |= (1 << y)
		z.Memory.Write(addr, val)
		// Undocumented: also copy result to register
		if z_val != 6 {
			*z.getRegister8(z_val) = val
		}
	}
	
	return 23
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
		opcode == 0xE9 || // JP (HL) - but this doesn't get displaced!
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