package z80

// executeCB handles CB-prefixed instructions
func (z *Z80) executeCB() int {
	opcode := z.fetchByte()
	// Increment R for the post-prefix opcode fetch (M1)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	
	// Decode CB opcode structure
	x := opcode >> 6        // Bits 7-6
	y := (opcode >> 3) & 7   // Bits 5-3
	z_val := opcode & 7      // Bits 2-0
	
	// Get the operand
	var val uint8
	var addr uint16
	cycles := 8 // Base cycles for CB instructions
	
	if z_val == 6 {
		// (HL) operand
		addr = z.HL()
		val = z.Memory.Read(addr)
		cycles = 15 // CB operations on (HL) take longer
	} else {
		// Register operand
		val = *z.getRegister8(z_val)
	}
	
	switch x {
	case 0: // Rotation/shift operations
		switch y {
		case 0: val = z.rlc8(val)  // RLC
		case 1: val = z.rrc8(val)  // RRC
		case 2: val = z.rl8(val)   // RL
		case 3: val = z.rr8(val)   // RR
		case 4: val = z.sla8(val)  // SLA
		case 5: val = z.sra8(val)  // SRA
		case 6: val = z.sll8(val)  // SLL (undocumented)
		case 7: val = z.srl8(val)  // SRL
		}
		
		// Write result back
		if z_val == 6 {
			z.Memory.Write(addr, val)
		} else {
			*z.getRegister8(z_val) = val
		}
		
			case 1: // BIT y,r
		z.bit(val, y)
		if z_val == 6 {
			cycles = 12 // BIT n,(HL) is faster than other (HL) ops
			// For BIT n,(HL), X and Y flags come from high byte of address
			// The high byte is already in H register
			z.setFlag(FlagX, z.H&FlagX != 0)
			z.setFlag(FlagY, z.H&FlagY != 0)
		}
		case 2: // RES y,r
		val &^= (1 << y)
		if z_val == 6 {
			z.Memory.Write(addr, val)
		} else {
			*z.getRegister8(z_val) = val
		}
		
	case 3: // SET y,r
		val |= (1 << y)
		if z_val == 6 {
			z.Memory.Write(addr, val)
		} else {
			*z.getRegister8(z_val) = val
		}
	}
	
	return cycles
}

// bit tests a bit and sets flags accordingly
func (z *Z80) bit(val uint8, bit uint8) {
	result := val & (1 << bit)
	
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, true)
	z.setFlag(FlagN, false)
	// For BIT, the S flag is set if bit 7 is tested and set
	if bit == 7 {
		z.setFlag(FlagS, result != 0)
	} else {
		z.setFlag(FlagS, false)
	}
	// PV flag is set to same as Z flag for BIT
	z.setFlag(FlagPV, result == 0)
	// X and Y flags copy bits 3 and 5 of the value
	z.setFlag(FlagX, val&FlagX != 0)
	z.setFlag(FlagY, val&FlagY != 0)
}