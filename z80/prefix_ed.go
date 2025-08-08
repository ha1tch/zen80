package z80

// executeED handles ED-prefixed instructions
func (z *Z80) executeED() int {
	opcode := z.fetchByte()
	
	// Decode ED opcode structure
	x := opcode >> 6        // Bits 7-6
	y := (opcode >> 3) & 7   // Bits 5-3
	z_val := opcode & 7      // Bits 2-0
	p := y >> 1              // Bits 5-4
	q := y & 1               // Bit 3
	
	// Many undefined ED opcodes act as NOP
	cycles := 8 // Default for undefined instructions
	
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
			z.F = (z.F & FlagC) | (val & (FlagX | FlagY))
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
				z.R = z.A
				return 9
			case 2: // LD A,I
				z.A = z.I
				z.setFlag(FlagS, z.A&0x80 != 0)
				z.setFlag(FlagZ, z.A == 0)
				z.setFlag(FlagH, false)
				z.setFlag(FlagPV, z.IFF2)
				z.setFlag(FlagN, false)
				z.F = (z.F & FlagC) | (z.A & (FlagX | FlagY))
				return 9
			case 3: // LD A,R
				z.A = z.R
				z.setFlag(FlagS, z.A&0x80 != 0)
				z.setFlag(FlagZ, z.A == 0)
				z.setFlag(FlagH, false)
				z.setFlag(FlagPV, z.IFF2)
				z.setFlag(FlagN, false)
				z.F = (z.F & FlagC) | (z.A & (FlagX | FlagY))
				return 9
			case 4: // RRD
				z.rrd()
				return 18
			case 5: // RLD
				z.rld()
				return 18
			case 6, 7: // NOP
				return 8
			}
		}
		
	case 2:
		// Block instructions
		if z_val <= 3 && y >= 4 {
			return z.executeBlockInstruction(y, z_val)
		}
	}
	
	return cycles
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
	z.F = (z.F & FlagC) | (z.A & (FlagX | FlagY))
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
	z.F = (z.F & FlagC) | (z.A & (FlagX | FlagY))
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