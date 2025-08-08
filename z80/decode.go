package z80

// execute decodes and executes a single opcode, returning the number of cycles taken.
func (z *Z80) execute(opcode uint8) int {
	// Decode using the pattern from the Z80 opcode structure
	x := opcode >> 6        // Bits 7-6
	y := (opcode >> 3) & 7   // Bits 5-3
	z := opcode & 7          // Bits 2-0
	p := y >> 1              // Bits 5-4
	q := y & 1               // Bit 3

	switch x {
	case 0:
		return z.executeBlock0(opcode, y, z, p, q)
	case 1:
		return z.executeBlock1(opcode, y, z)
	case 2:
		return z.executeBlock2(opcode, y, z)
	case 3:
		return z.executeBlock3(opcode, y, z, p, q)
	}
	return 4 // Default NOP cycles
}

// executeBlock0 handles opcodes 0x00-0x3F
func (z *Z80) executeBlock0(opcode uint8, y, z, p, q uint8) int {
	switch z {
	case 0:
		switch y {
		case 0: // NOP
			return 4
		case 1: // EX AF,AF'
			z.A, z.A_ = z.A_, z.A
			z.F, z.F_ = z.F_, z.F
			return 4
		case 2: // DJNZ d
			z.B--
			if z.B != 0 {
				d := int8(z.fetchByte())
				z.PC = uint16(int32(z.PC) + int32(d))
				z.WZ = z.PC
				return 13
			}
			z.fetchByte() // Skip displacement
			return 8
		case 3: // JR d
			d := int8(z.fetchByte())
			z.PC = uint16(int32(z.PC) + int32(d))
			z.WZ = z.PC
			return 12
		default: // JR cc,d (y=4..7)
			if z.testCondition(y - 4) {
				d := int8(z.fetchByte())
				z.PC = uint16(int32(z.PC) + int32(d))
				z.WZ = z.PC
				return 12
			}
			z.fetchByte() // Skip displacement
			return 7
		}

	case 1:
		if q == 0 { // LD rp,nn
			val := z.fetchWord()
			switch p {
			case 0: z.SetBC(val)
			case 1: z.SetDE(val)
			case 2: z.SetHL(val)
			case 3: z.SP = val
			}
			return 10
		} else { // ADD HL,rp
			var val uint16
			switch p {
			case 0: val = z.BC()
			case 1: val = z.DE()
			case 2: val = z.HL()
			case 3: val = z.SP
			}
			z.SetHL(z.add16(z.HL(), val))
			return 11
		}

	case 2:
		// Indirect loading
		switch y {
		case 0: // LD (BC),A
			z.Memory.Write(z.BC(), z.A)
			z.WZ = (uint16(z.A) << 8) | ((z.BC() + 1) & 0xFF)
			return 7
		case 1: // LD A,(BC)
			z.A = z.Memory.Read(z.BC())
			z.WZ = z.BC() + 1
			return 7
		case 2: // LD (DE),A
			z.Memory.Write(z.DE(), z.A)
			z.WZ = (uint16(z.A) << 8) | ((z.DE() + 1) & 0xFF)
			return 7
		case 3: // LD A,(DE)
			z.A = z.Memory.Read(z.DE())
			z.WZ = z.DE() + 1
			return 7
		case 4: // LD (nn),HL
			addr := z.fetchWord()
			z.writeWord(addr, z.HL())
			z.WZ = addr + 1
			return 16
		case 5: // LD HL,(nn)
			addr := z.fetchWord()
			z.SetHL(z.readWord(addr))
			z.WZ = addr + 1
			return 16
		case 6: // LD (nn),A
			addr := z.fetchWord()
			z.Memory.Write(addr, z.A)
			z.WZ = (uint16(z.A) << 8) | ((addr + 1) & 0xFF)
			return 13
		case 7: // LD A,(nn)
			addr := z.fetchWord()
			z.A = z.Memory.Read(addr)
			z.WZ = addr + 1
			return 13
		}

	case 3:
		if q == 0 { // INC rp
			switch p {
			case 0: z.SetBC(z.BC() + 1)
			case 1: z.SetDE(z.DE() + 1)
			case 2: z.SetHL(z.HL() + 1)
			case 3: z.SP++
			}
			return 6
		} else { // DEC rp
			switch p {
			case 0: z.SetBC(z.BC() - 1)
			case 1: z.SetDE(z.DE() - 1)
			case 2: z.SetHL(z.HL() - 1)
			case 3: z.SP--
			}
			return 6
		}

	case 4: // INC r
		*z.getRegister8(y) = z.inc8(*z.getRegister8(y))
		if y == 6 { // (HL)
			return 11
		}
		return 4

	case 5: // DEC r
		*z.getRegister8(y) = z.dec8(*z.getRegister8(y))
		if y == 6 { // (HL)
			return 11
		}
		return 4

	case 6: // LD r,n
		val := z.fetchByte()
		*z.getRegister8(y) = val
		if y == 6 { // (HL)
			return 10
		}
		return 7

	case 7:
		// Assorted operations on accumulator/flags
		switch y {
		case 0: // RLCA
			z.A = z.rlc8(z.A)
			z.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 1: // RRCA
			z.A = z.rrc8(z.A)
			z.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 2: // RLA
			z.A = z.rl8(z.A)
			z.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 3: // RRA
			z.A = z.rr8(z.A)
			z.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 4: // DAA
			z.daa()
			return 4
		case 5: // CPL
			z.A = ^z.A
			z.setFlag(FlagN, true)
			z.setFlag(FlagH, true)
			z.F = (z.F & (FlagS | FlagZ | FlagPV | FlagC)) | FlagN | FlagH | (z.A & (FlagX | FlagY))
			return 4
		case 6: // SCF
			z.setFlag(FlagC, true)
			z.setFlag(FlagN, false)
			z.setFlag(FlagH, false)
			z.F = (z.F & (FlagS | FlagZ | FlagPV)) | FlagC | (z.A & (FlagX | FlagY))
			return 4
		case 7: // CCF
			z.setFlag(FlagH, z.getFlag(FlagC))
			z.setFlag(FlagC, !z.getFlag(FlagC))
			z.setFlag(FlagN, false)
			z.F = (z.F & (FlagS | FlagZ | FlagPV | FlagC)) | (z.A & (FlagX | FlagY))
			return 4
		}
	}
	return 4
}

// executeBlock1 handles opcodes 0x40-0x7F (8-bit loads and HALT)
func (z *Z80) executeBlock1(opcode uint8, y, z uint8) int {
	if opcode == 0x76 { // HALT
		z.Halted = true
		return 4
	}
	
	// LD r,r'
	src := z.getRegister8(z)
	dst := z.getRegister8(y)
	*dst = *src
	
	if y == 6 || z == 6 { // (HL) involved
		return 7
	}
	return 4
}

// executeBlock2 handles opcodes 0x80-0xBF (ALU operations)
func (z *Z80) executeBlock2(opcode uint8, y, z uint8) int {
	val := *z.getRegister8(z)
	
	switch y {
	case 0: z.add8(val)     // ADD A,r
	case 1: z.adc8(val)     // ADC A,r
	case 2: z.sub8(val)     // SUB r
	case 3: z.sbc8(val)     // SBC A,r
	case 4: z.and8(val)     // AND r
	case 5: z.xor8(val)     // XOR r
	case 6: z.or8(val)      // OR r
	case 7: z.cp8(val)      // CP r
	}
	
	if z == 6 { // (HL)
		return 7
	}
	return 4
}

// executeBlock3 handles opcodes 0xC0-0xFF
func (z *Z80) executeBlock3(opcode uint8, y, z, p, q uint8) int {
	switch z {
	case 0: // RET cc
		if z.testCondition(y) {
			z.PC = z.pop()
			z.WZ = z.PC
			return 11
		}
		return 5

	case 1:
		if q == 0 { // POP rp2
			val := z.pop()
			switch p {
			case 0: z.SetBC(val)
			case 1: z.SetDE(val)
			case 2: z.SetHL(val)
			case 3: z.SetAF(val)
			}
			return 10
		} else {
			switch p {
			case 0: // RET
				z.PC = z.pop()
				z.WZ = z.PC
				return 10
			case 1: // EXX
				z.B, z.B_ = z.B_, z.B
				z.C, z.C_ = z.C_, z.C
				z.D, z.D_ = z.D_, z.D
				z.E, z.E_ = z.E_, z.E
				z.H, z.H_ = z.H_, z.H
				z.L, z.L_ = z.L_, z.L
				return 4
			case 2: // JP HL
				z.PC = z.HL()
				return 4
			case 3: // LD SP,HL
				z.SP = z.HL()
				return 6
			}
		}

	case 2: // JP cc,nn
		addr := z.fetchWord()
		if z.testCondition(y) {
			z.PC = addr
			z.WZ = addr
			return 10
		}
		return 10

	case 3:
		switch y {
		case 0: // JP nn
			z.PC = z.fetchWord()
			z.WZ = z.PC
			return 10
		case 1: // CB prefix
			return z.executeCB()
		case 2: // OUT (n),A
			port := z.fetchByte()
			addr := uint16(port) | (uint16(z.A) << 8)
			z.IO.Out(addr, z.A)
			z.WZ = addr
			return 11
		case 3: // IN A,(n)
			port := z.fetchByte()
			addr := uint16(port) | (uint16(z.A) << 8)
			z.A = z.IO.In(addr)
			z.WZ = addr + 1
			return 11
		case 4: // EX (SP),HL
			val := z.readWord(z.SP)
			z.writeWord(z.SP, z.HL())
			z.SetHL(val)
			z.WZ = val
			return 19
		case 5: // EX DE,HL
			de := z.DE()
			z.SetDE(z.HL())
			z.SetHL(de)
			return 4
		case 6: // DI
			z.pendingDI = true
			return 4
		case 7: // EI
			z.pendingEI = true
			return 4
		}

	case 4: // CALL cc,nn
		addr := z.fetchWord()
		if z.testCondition(y) {
			z.push(z.PC)
			z.PC = addr
			z.WZ = addr
			return 17
		}
		return 10

	case 5:
		if q == 0 { // PUSH rp2
			var val uint16
			switch p {
			case 0: val = z.BC()
			case 1: val = z.DE()
			case 2: val = z.HL()
			case 3: val = z.AF()
			}
			z.push(val)
			return 11
		} else {
			switch p {
			case 0: // CALL nn
				addr := z.fetchWord()
				z.push(z.PC)
				z.PC = addr
				z.WZ = addr
				return 17
			case 1: // DD prefix
				return z.executeDD()
			case 2: // ED prefix
				return z.executeED()
			case 3: // FD prefix
				return z.executeFD()
			}
		}

	case 6: // ALU n
		val := z.fetchByte()
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
		return 7

	case 7: // RST p*8
		z.push(z.PC)
		z.PC = uint16(y) * 8
		z.WZ = z.PC
		return 11
	}
	
	return 4
}

// getRegister8 returns a pointer to the 8-bit register specified by the index
func (z *Z80) getRegister8(index uint8) *uint8 {
	switch index {
	case 0: return &z.B
	case 1: return &z.C
	case 2: return &z.D
	case 3: return &z.E
	case 4: return &z.H
	case 5: return &z.L
	case 6: // (HL)
		addr := z.HL()
		val := z.Memory.Read(addr)
		// Return a pointer to a temporary - caller must handle (HL) specially
		temp := val
		return &temp
	case 7: return &z.A
	default: return &z.A
	}
}