package z80

// executeED handles ED-prefixed instructions
func (z *Z80) executeED() int {
	opcode := z.fetchByte()
	// Increment R for the post-prefix opcode fetch (M1)
	z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
	
	// Debug M1 trace for post-prefix fetch
	if DEBUG_M1 && z.M1Hook != nil {
		z.M1Hook(z.PC-1, opcode, "post-ED")
	}
	
	// Decode ED opcode structure
	x := opcode >> 6        // Bits 7-6
	y := (opcode >> 3) & 7   // Bits 5-3
	z_val := opcode & 7      // Bits 2-0
	p := y >> 1              // Bits 5-4
	q := y & 1               // Bit 3
	
	// Handle undefined ED opcodes that duplicate other instructions
	// Based on actual Z80 hardware behavior
	switch opcode {
	// ED 00-3F range - mostly undefined, act as NOPs
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	     0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
	     0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	     0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	     0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27,
	     0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F,
	     0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37,
	     0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F:
		return 8 // NOP behavior
		
	// ED 4C, 54, 5C, 64, 6C, 74, 7C - duplicate NEG (ED 44)
	case 0x4C, 0x54, 0x5C, 0x64, 0x6C, 0x74, 0x7C:
		z.neg()
		return 8
		
	// ED 4E, 6E - duplicate IM 0 (NMOS compatibility)
	case 0x4E:
		z.IM = 0
		return 8
	case 0x6E:
		z.IM = 0
		return 8
		
	// ED 66 - duplicate IM 0
	case 0x66:
		z.IM = 0
		return 8
		
	// ED 76 - duplicate IM 1
	case 0x76:
		z.IM = 1
		return 8
		
	// ED 77, 7F - undefined, act as NOP
	case 0x77, 0x7F:
		return 8
		
	// ED 80-9F range - mostly undefined, act as NOPs
	case 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
	     0x88, 0x89, 0x8A, 0x8B, 0x8C, 0x8D, 0x8E, 0x8F,
	     0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97,
	     0x98, 0x99, 0x9A, 0x9B, 0x9C, 0x9D, 0x9E, 0x9F:
		return 8 // NOP behavior
		
	// ED A4-A7, AC-AF, B4-B7, BC-BF - undefined, act as NOPs
	case 0xA4, 0xA5, 0xA6, 0xA7, 0xAC, 0xAD, 0xAE, 0xAF,
	     0xB4, 0xB5, 0xB6, 0xB7, 0xBC, 0xBD, 0xBE, 0xBF:
		return 8 // NOP behavior
		
	// ED C0-FF range - mostly undefined, act as NOPs
	case 0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7,
	     0xC8, 0xC9, 0xCA, 0xCB, 0xCC, 0xCD, 0xCE, 0xCF,
	     0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7,
	     0xD8, 0xD9, 0xDA, 0xDB, 0xDC, 0xDD, 0xDE, 0xDF,
	     0xE0, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7,
	     0xE8, 0xE9, 0xEA, 0xEB, 0xEC, 0xED, 0xEE, 0xEF,
	     0xF0, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7,
	     0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF:
		return 8 // NOP behavior
	}
	
	// Handle defined ED instructions
	switch x {
	case 1:
		switch z_val {
		case 0: // IN r,(C) or IN (C)
			val := z.IO.In(z.BC())
			if y != 6 {
				*z.getRegister8(y) = val
			}
			z.setFlag(FlagS, val&0x80 != 0)
			z.setFlag(FlagZ, val == 0)
			z.setFlag(FlagH, false)
			z.setFlag(FlagPV, parity(val))
			z.setFlag(FlagN, false)
			z.setFlag(FlagX, val&FlagX != 0)
			z.setFlag(FlagY, val&FlagY != 0)
			z.WZ = z.BC() + 1
			return 12
			
		case 1: // OUT (C),r or OUT (C),0
			val := uint8(0)
			if y != 6 {
				val = *z.getRegister8(y)
			}
			z.IO.Out(z.BC(), val)
			z.WZ = z.BC() + 1
			return 12
			
		case 2: // SBC/ADC HL,rp
			var val uint16
			switch p {
			case 0: val = z.BC()
			case 1: val = z.DE()
			case 2: val = z.HL()
			case 3: val = z.SP
			}
			
			if q == 0 { // SBC HL,rp
				z.SetHL(z.sbc16(z.HL(), val))
			} else { // ADC HL,rp
				z.SetHL(z.adc16(z.HL(), val))
			}
			return 15
			
		case 3: // LD (nn),rp / LD rp,(nn)
			addr := z.fetchWord()
			if q == 0 { // LD (nn),rp
				var val uint16
				switch p {
				case 0: val = z.BC()
				case 1: val = z.DE()
				case 2: val = z.HL()
				case 3: val = z.SP
				}
				z.writeWord(addr, val)
				z.WZ = addr + 1
			} else { // LD rp,(nn)
				val := z.readWord(addr)
				switch p {
				case 0: z.SetBC(val)
				case 1: z.SetDE(val)
				case 2: z.SetHL(val)
				case 3: z.SP = val
				}
				z.WZ = addr + 1
			}
			return 20
			
		case 4: // NEG
			z.neg()
			return 8
			
		case 5: // RETN/RETI
			z.PC = z.pop()
			z.WZ = z.PC
			z.IFF1 = z.IFF2
			// RETI (y=1) would signal to peripherals, but we don't distinguish here
			return 14
			
		case 6: // IM n
			switch y {
			case 0, 4: z.IM = 0
			case 1, 5: z.IM = 0 // Actually IM 0/1
			case 2, 6: z.IM = 1
			case 3, 7: z.IM = 2
			}
			return 8
			
		case 7: // Special cases
			switch y {
			case 0: // LD I,A
				z.I = z.A
				return 9
			case 1: // LD R,A
				// R was already incremented after the opcode fetch
				// Simply write A to R (preserving bit 7 as usual is handled by the write)
				z.R = z.A
				return 9
			case 2: // LD A,I
				z.A = z.I
				z.setFlag(FlagS, z.A&0x80 != 0)
				z.setFlag(FlagZ, z.A == 0)
				z.setFlag(FlagH, false)
				z.setFlag(FlagPV, z.IFF2)
				z.setFlag(FlagN, false)
				z.setFlag(FlagX, z.A&FlagX != 0)
				z.setFlag(FlagY, z.A&FlagY != 0)
				return 9
			case 3: // LD A,R
				// R was already incremented after the opcode fetch
				// So we read the post-increment value, which is correct
				z.A = z.R
				z.setFlag(FlagS, z.A&0x80 != 0)
				z.setFlag(FlagZ, z.A == 0)
				z.setFlag(FlagH, false)
				z.setFlag(FlagPV, z.IFF2)
				z.setFlag(FlagN, false)
				z.setFlag(FlagX, z.A&FlagX != 0)
				z.setFlag(FlagY, z.A&FlagY != 0)
				return 9
			case 4: // RRD
				z.rrd()
				return 18
			case 5: // RLD
				z.rld()
				return 18
			case 6, 7: // NOP (handled in switch above)
				return 8
			}
		}
		
	case 2:
		// Block instructions
		if z_val <= 3 && y >= 4 {
			return z.executeBlockInstruction(y, z_val)
		}
	}
	
	// If we get here, it's an undefined opcode that wasn't caught above
	// This shouldn't happen with our comprehensive switch, but just in case
	return 8
}

// neg performs two's complement negation of A
func (z *Z80) neg() {
	val := z.A
	z.A = 0
	z.sub8(val)
}

// rrd performs rotate right decimal
func (z *Z80) rrd() {
	addr := z.HL()
	val := z.Memory.Read(addr)
	newVal := ((z.A & 0x0F) << 4) | (val >> 4)
	z.A = (z.A & 0xF0) | (val & 0x0F)
	z.Memory.Write(addr, newVal)
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(z.A))
	z.setFlag(FlagN, false)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
	z.WZ = z.HL() + 1
}

// rld performs rotate left decimal
func (z *Z80) rld() {
	addr := z.HL()
	val := z.Memory.Read(addr)
	newVal := ((val & 0x0F) << 4) | (z.A & 0x0F)
	z.A = (z.A & 0xF0) | (val >> 4)
	z.Memory.Write(addr, newVal)
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(z.A))
	z.setFlag(FlagN, false)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
	z.WZ = z.HL() + 1
}

// executeBlockInstruction handles ED block instructions
func (z *Z80) executeBlockInstruction(y, z_val uint8) int {
	switch y {
	case 4: // LDI, CPI, INI, OUTI
		switch z_val {
		case 0: return z.ldi()
		case 1: return z.cpi()
		case 2: return z.ini()
		case 3: return z.outi()
		}
	case 5: // LDD, CPD, IND, OUTD
		switch z_val {
		case 0: return z.ldd()
		case 1: return z.cpd()
		case 2: return z.ind()
		case 3: return z.outd()
		}
	case 6: // LDIR, CPIR, INIR, OTIR
		switch z_val {
		case 0: return z.ldir()
		case 1: return z.cpir()
		case 2: return z.inir()
		case 3: return z.otir()
		}
	case 7: // LDDR, CPDR, INDR, OTDR
		switch z_val {
		case 0: return z.lddr()
		case 1: return z.cpdr()
		case 2: return z.indr()
		case 3: return z.otdr()
		}
	}
	return 8
}

// Block transfer instructions

func (z *Z80) ldi() int {
	val := z.Memory.Read(z.HL())
	z.Memory.Write(z.DE(), val)
	z.SetHL(z.HL() + 1)
	z.SetDE(z.DE() + 1)
	z.SetBC(z.BC() - 1)
	
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, z.BC() != 0)
	z.setFlag(FlagN, false)
	// X and Y flags are complex for block instructions
	n := val + z.A
	z.F = (z.F & (FlagS | FlagZ | FlagC)) | (n & FlagX) | ((n & 0x02) << 4)
	return 16
}

func (z *Z80) ldd() int {
	val := z.Memory.Read(z.HL())
	z.Memory.Write(z.DE(), val)
	z.SetHL(z.HL() - 1)
	z.SetDE(z.DE() - 1)
	z.SetBC(z.BC() - 1)
	
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, z.BC() != 0)
	z.setFlag(FlagN, false)
	// Y flag calculation
	n := val + z.A
	z.F = (z.F & (FlagS | FlagZ | FlagC)) | (n & FlagX) | ((n & 0x02) << 4)
	return 16
}

func (z *Z80) ldir() int {
	z.ldi()
	if z.BC() != 0 {
		z.PC -= 2 // Repeat instruction
		z.WZ = z.PC + 1
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

func (z *Z80) lddr() int {
	z.ldd()
	if z.BC() != 0 {
		z.PC -= 2 // Repeat instruction
		z.WZ = z.PC + 1
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

// Block search instructions

func (z *Z80) cpi() int {
	val := z.Memory.Read(z.HL())
	result := int16(z.A) - int16(val)
	z.SetHL(z.HL() + 1)
	z.SetBC(z.BC() - 1)
	
	z.setFlag(FlagS, uint8(result)&0x80 != 0)
	z.setFlag(FlagZ, uint8(result) == 0)
	z.setFlag(FlagH, (int8(z.A&0x0F) - int8(val&0x0F)) < 0)
	z.setFlag(FlagPV, z.BC() != 0)
	z.setFlag(FlagN, true)
	
	// Y flag calculation - preserve S, Z, H, PV, N, C flags
	n := uint8(result)
	if z.getFlag(FlagH) {
		n--
	}
	z.F = (z.F & (FlagS | FlagZ | FlagH | FlagPV | FlagN | FlagC)) | (n & FlagX) | ((n & 0x02) << 4)
	z.WZ++
	return 16
}

func (z *Z80) cpd() int {
	val := z.Memory.Read(z.HL())
	result := int16(z.A) - int16(val)
	z.SetHL(z.HL() - 1)
	z.SetBC(z.BC() - 1)
	
	z.setFlag(FlagS, uint8(result)&0x80 != 0)
	z.setFlag(FlagZ, uint8(result) == 0)
	z.setFlag(FlagH, (int8(z.A&0x0F) - int8(val&0x0F)) < 0)
	z.setFlag(FlagPV, z.BC() != 0)
	z.setFlag(FlagN, true)
	
	// Y flag calculation - preserve S, Z, H, PV, N, C flags
	n := uint8(result)
	if z.getFlag(FlagH) {
		n--
	}
	z.F = (z.F & (FlagS | FlagZ | FlagH | FlagPV | FlagN | FlagC)) | (n & FlagX) | ((n & 0x02) << 4)
	z.WZ--
	return 16
}

func (z *Z80) cpir() int {
	z.cpi()
	if z.BC() != 0 && !z.getFlag(FlagZ) {
		z.PC -= 2 // Repeat instruction
		z.WZ = z.PC + 1
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

func (z *Z80) cpdr() int {
	z.cpd()
	if z.BC() != 0 && !z.getFlag(FlagZ) {
		z.PC -= 2 // Repeat instruction
		z.WZ = z.PC + 1
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

// Block I/O instructions

func (z *Z80) ini() int {
	val := z.IO.In(z.BC())
	z.Memory.Write(z.HL(), val)
	z.SetHL(z.HL() + 1)
	z.B--
	
	// Enhanced: Accurate flag calculation for INI
	k := int(val) + int((z.C + 1) & 0xFF)
	
	z.setFlag(FlagZ, z.B == 0)
	z.setFlag(FlagS, z.B&0x80 != 0)
	z.setFlag(FlagN, (val & 0x80) != 0)
	z.setFlag(FlagH, k > 0xFF)
	z.setFlag(FlagC, k > 0xFF)
	// P/V flag is parity of ((k & 0x07) XOR B)
	z.setFlag(FlagPV, parity(uint8(k & 0x07) ^ z.B))
	// X and Y flags from B register
	z.F = (z.F & 0xD7) | (z.B & (FlagX | FlagY))
	
	z.WZ = z.BC() + 1
	return 16
}

func (z *Z80) ind() int {
	val := z.IO.In(z.BC())
	z.Memory.Write(z.HL(), val)
	z.SetHL(z.HL() - 1)
	z.B--
	
	// Enhanced: Accurate flag calculation for IND
	// Note: Based on Z80 documentation, k = val + C (not C-1)
	k := int(val) + int(z.C)
	
	z.setFlag(FlagZ, z.B == 0)
	z.setFlag(FlagS, z.B&0x80 != 0)
	z.setFlag(FlagN, (val & 0x80) != 0)
	z.setFlag(FlagH, k > 0xFF)
	z.setFlag(FlagC, k > 0xFF)
	// P/V flag is parity of ((k & 0x07) XOR B)
	pvVal := uint8(k&0x07) ^ z.B
	z.setFlag(FlagPV, parity(pvVal))
	// X and Y flags from B register
	z.F = (z.F & 0xD7) | (z.B & (FlagX | FlagY))
	
	z.WZ = z.BC() - 1
	return 16
}

func (z *Z80) inir() int {
	z.ini()
	if z.B != 0 {
		z.PC -= 2 // Repeat instruction
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

func (z *Z80) indr() int {
	z.ind()
	if z.B != 0 {
		z.PC -= 2 // Repeat instruction
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

func (z *Z80) outi() int {
	val := z.Memory.Read(z.HL())
	z.B--
	z.IO.Out(z.BC(), val)
	z.SetHL(z.HL() + 1)
	
	// Enhanced: Accurate flag calculation for OUTI
	// Note: Use L after HL increment
	k := int(val) + int(z.L)
	
	z.setFlag(FlagZ, z.B == 0)
	z.setFlag(FlagS, z.B&0x80 != 0)
	z.setFlag(FlagN, (val & 0x80) != 0)
	z.setFlag(FlagH, k > 0xFF)
	z.setFlag(FlagC, k > 0xFF)
	// P/V flag is parity of ((k & 0x07) XOR B)
	pvVal := uint8(k&0x07) ^ z.B
	z.setFlag(FlagPV, parity(pvVal))
	// X and Y flags from B register
	z.F = (z.F & 0xD7) | (z.B & (FlagX | FlagY))
	
	z.WZ = z.BC() + 1
	return 16
}

func (z *Z80) outd() int {
	val := z.Memory.Read(z.HL())
	z.B--
	z.IO.Out(z.BC(), val)
	z.SetHL(z.HL() - 1)
	
	// Enhanced: Accurate flag calculation for OUTD
	// Note: Use L after HL decrement  
	k := int(val) + int(z.L)
	
	z.setFlag(FlagZ, z.B == 0)
	z.setFlag(FlagS, z.B&0x80 != 0)
	z.setFlag(FlagN, (val & 0x80) != 0)
	z.setFlag(FlagH, k > 0xFF)
	z.setFlag(FlagC, k > 0xFF)
	// P/V flag is parity of ((k & 0x07) XOR B)
	pvVal := uint8(k&0x07) ^ z.B
	z.setFlag(FlagPV, parity(pvVal))
	// X and Y flags from B register
	z.F = (z.F & 0xD7) | (z.B & (FlagX | FlagY))
	
	z.WZ = z.BC() - 1
	return 16
}

func (z *Z80) otir() int {
	z.outi()
	if z.B != 0 {
		z.PC -= 2 // Repeat instruction
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}

func (z *Z80) otdr() int {
	z.outd()
	if z.B != 0 {
		z.PC -= 2 // Repeat instruction
		// Increment R for the extra M1 cycle when repeating
		z.R = (z.R & 0x80) | ((z.R + 1) & 0x7F)
		return 21
	}
	return 16
}