package z80

// execute decodes and executes a single opcode, returning the number of cycles taken.
func (cpu *Z80) execute(opcode uint8) int {
	// Decode using the pattern from the Z80 opcode structure
	x := opcode >> 6        // Bits 7-6
	y := (opcode >> 3) & 7   // Bits 5-3
	z := opcode & 7          // Bits 2-0
	p := y >> 1              // Bits 5-4
	q := y & 1               // Bit 3

	switch x {
	case 0:
		return cpu.executeBlock0(opcode, y, z, p, q)
	case 1:
		return cpu.executeBlock1(opcode, y, z)
	case 2:
		return cpu.executeBlock2(opcode, y, z)
	case 3:
		return cpu.executeBlock3(opcode, y, z, p, q)
	}
	return 4 // Default NOP cycles
}

// executeBlock0 handles opcodes 0x00-0x3F
func (cpu *Z80) executeBlock0(opcode uint8, y, z, p, q uint8) int {
	switch z {
	case 0:
		switch y {
		case 0: // NOP
			return 4
		case 1: // EX AF,AF'
			cpu.A, cpu.A_ = cpu.A_, cpu.A
			cpu.F, cpu.F_ = cpu.F_, cpu.F
			return 4
		case 2: // DJNZ d
			cpu.B--
			if cpu.B != 0 {
				d := int8(cpu.fetchByte())
				cpu.PC = uint16(int32(cpu.PC) + int32(d))
				cpu.WZ = cpu.PC
				return 13
			}
			cpu.fetchByte() // Skip displacement
			return 8
		case 3: // JR d
			d := int8(cpu.fetchByte())
			cpu.PC = uint16(int32(cpu.PC) + int32(d))
			cpu.WZ = cpu.PC
			return 12
		default: // JR cc,d (y=4..7)
			if cpu.testCondition(y - 4) {
				d := int8(cpu.fetchByte())
				cpu.PC = uint16(int32(cpu.PC) + int32(d))
				cpu.WZ = cpu.PC
				return 12
			}
			cpu.fetchByte() // Skip displacement
			return 7
		}

	case 1:
		if q == 0 { // LD rp,nn
			val := cpu.fetchWord()
			switch p {
			case 0: cpu.SetBC(val)
			case 1: cpu.SetDE(val)
			case 2: cpu.SetHL(val)
			case 3: cpu.SP = val
			}
			return 10
		} else { // ADD HL,rp
			var val uint16
			switch p {
			case 0: val = cpu.BC()
			case 1: val = cpu.DE()
			case 2: val = cpu.HL()
			case 3: val = cpu.SP
			}
			cpu.SetHL(cpu.add16(cpu.HL(), val))
			return 11
		}

	case 2:
		// Indirect loading
		switch y {
		case 0: // LD (BC),A
			cpu.Memory.Write(cpu.BC(), cpu.A)
			cpu.WZ = (uint16(cpu.A) << 8) | ((cpu.BC() + 1) & 0xFF)
			return 7
		case 1: // LD A,(BC)
			cpu.A = cpu.Memory.Read(cpu.BC())
			cpu.WZ = cpu.BC() + 1
			return 7
		case 2: // LD (DE),A
			cpu.Memory.Write(cpu.DE(), cpu.A)
			cpu.WZ = (uint16(cpu.A) << 8) | ((cpu.DE() + 1) & 0xFF)
			return 7
		case 3: // LD A,(DE)
			cpu.A = cpu.Memory.Read(cpu.DE())
			cpu.WZ = cpu.DE() + 1
			return 7
		case 4: // LD (nn),HL
			addr := cpu.fetchWord()
			cpu.writeWord(addr, cpu.HL())
			cpu.WZ = addr + 1
			return 16
		case 5: // LD HL,(nn)
			addr := cpu.fetchWord()
			cpu.SetHL(cpu.readWord(addr))
			cpu.WZ = addr + 1
			return 16
		case 6: // LD (nn),A
			addr := cpu.fetchWord()
			cpu.Memory.Write(addr, cpu.A)
			cpu.WZ = (uint16(cpu.A) << 8) | ((addr + 1) & 0xFF)
			return 13
		case 7: // LD A,(nn)
			addr := cpu.fetchWord()
			cpu.A = cpu.Memory.Read(addr)
			cpu.WZ = addr + 1
			return 13
		}

	case 3:
		if q == 0 { // INC rp
			switch p {
			case 0: cpu.SetBC(cpu.BC() + 1)
			case 1: cpu.SetDE(cpu.DE() + 1)
			case 2: cpu.SetHL(cpu.HL() + 1)
			case 3: cpu.SP++
			}
			return 6
		} else { // DEC rp
			switch p {
			case 0: cpu.SetBC(cpu.BC() - 1)
			case 1: cpu.SetDE(cpu.DE() - 1)
			case 2: cpu.SetHL(cpu.HL() - 1)
			case 3: cpu.SP--
			}
			return 6
		}

	case 4: // INC r
		reg := cpu.getRegister8(y)
		if y == 6 { // (HL)
			addr := cpu.HL()
			val := cpu.Memory.Read(addr)
			cpu.Memory.Write(addr, cpu.inc8(val))
			return 11
		}
		*reg = cpu.inc8(*reg)
		return 4

	case 5: // DEC r
		reg := cpu.getRegister8(y)
		if y == 6 { // (HL)
			addr := cpu.HL()
			val := cpu.Memory.Read(addr)
			cpu.Memory.Write(addr, cpu.dec8(val))
			return 11
		}
		*reg = cpu.dec8(*reg)
		return 4

	case 6: // LD r,n
		val := cpu.fetchByte()
		if y == 6 { // LD (HL),n
			cpu.Memory.Write(cpu.HL(), val)
			return 10
		}
		*cpu.getRegister8(y) = val
		return 7

	case 7:
		// Assorted operations on accumulator/flags
		switch y {
		case 0: // RLCA
			cpu.A = cpu.rlc8(cpu.A)
			cpu.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 1: // RRCA
			cpu.A = cpu.rrc8(cpu.A)
			cpu.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 2: // RLA
			cpu.A = cpu.rl8(cpu.A)
			cpu.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 3: // RRA
			cpu.A = cpu.rr8(cpu.A)
			cpu.F &^= (FlagZ | FlagPV | FlagS)
			return 4
		case 4: // DAA
			cpu.daa()
			return 4
		case 5: // CPL
			cpu.A = ^cpu.A
			cpu.setFlag(FlagN, true)
			cpu.setFlag(FlagH, true)
			cpu.F = (cpu.F & (FlagS | FlagZ | FlagPV | FlagC)) | FlagN | FlagH | (cpu.A & (FlagX | FlagY))
			return 4
		case 6: // SCF
			cpu.setFlag(FlagC, true)
			cpu.setFlag(FlagN, false)
			cpu.setFlag(FlagH, false)
			cpu.F = (cpu.F & (FlagS | FlagZ | FlagPV)) | FlagC | (cpu.A & (FlagX | FlagY))
			return 4
		case 7: // CCF
			cpu.setFlag(FlagH, cpu.getFlag(FlagC))
			cpu.setFlag(FlagC, !cpu.getFlag(FlagC))
			cpu.setFlag(FlagN, false)
			cpu.F = (cpu.F & (FlagS | FlagZ | FlagPV | FlagC)) | (cpu.A & (FlagX | FlagY))
			return 4
		}
	}
	return 4
}

// executeBlock1 handles opcodes 0x40-0x7F (8-bit loads and HALT)
func (cpu *Z80) executeBlock1(opcode uint8, y, z uint8) int {
	if opcode == 0x76 { // HALT
		cpu.Halted = true
		return 4
	}
	
	// LD r,r'
	if z == 6 { // Source is (HL)
		val := cpu.Memory.Read(cpu.HL())
		*cpu.getRegister8(y) = val
		return 7
	} else if y == 6 { // Dest is (HL)
		val := *cpu.getRegister8(z)
		cpu.Memory.Write(cpu.HL(), val)
		return 7
	} else { // Register to register
		*cpu.getRegister8(y) = *cpu.getRegister8(z)
		return 4
	}
}

// executeBlock2 handles opcodes 0x80-0xBF (ALU operations)
func (cpu *Z80) executeBlock2(opcode uint8, y, z uint8) int {
	var val uint8
	
	if z == 6 { // (HL)
		val = cpu.Memory.Read(cpu.HL())
	} else {
		val = *cpu.getRegister8(z)
	}
	
	switch y {
	case 0: cpu.add8(val)     // ADD A,r
	case 1: cpu.adc8(val)     // ADC A,r
	case 2: cpu.sub8(val)     // SUB r
	case 3: cpu.sbc8(val)     // SBC A,r
	case 4: cpu.and8(val)     // AND r
	case 5: cpu.xor8(val)     // XOR r
	case 6: cpu.or8(val)      // OR r
	case 7: cpu.cp8(val)      // CP r
	}
	
	if z == 6 { // (HL)
		return 7
	}
	return 4
}

// executeBlock3 handles opcodes 0xC0-0xFF
func (cpu *Z80) executeBlock3(opcode uint8, y, z, p, q uint8) int {
	switch z {
	case 0: // RET cc
		if cpu.testCondition(y) {
			cpu.PC = cpu.pop()
			cpu.WZ = cpu.PC
			return 11
		}
		return 5

	case 1:
		if q == 0 { // POP rp2
			val := cpu.pop()
			switch p {
			case 0: cpu.SetBC(val)
			case 1: cpu.SetDE(val)
			case 2: cpu.SetHL(val)
			case 3: cpu.SetAF(val)
			}
			return 10
		} else {
			switch p {
			case 0: // RET
				cpu.PC = cpu.pop()
				cpu.WZ = cpu.PC
				return 10
			case 1: // EXX
				cpu.B, cpu.B_ = cpu.B_, cpu.B
				cpu.C, cpu.C_ = cpu.C_, cpu.C
				cpu.D, cpu.D_ = cpu.D_, cpu.D
				cpu.E, cpu.E_ = cpu.E_, cpu.E
				cpu.H, cpu.H_ = cpu.H_, cpu.H
				cpu.L, cpu.L_ = cpu.L_, cpu.L
				return 4
			case 2: // JP HL
				cpu.PC = cpu.HL()
				return 4
			case 3: // LD SP,HL
				cpu.SP = cpu.HL()
				return 6
			}
		}

	case 2: // JP cc,nn
		addr := cpu.fetchWord()
		if cpu.testCondition(y) {
			cpu.PC = addr
			cpu.WZ = addr
			return 10
		}
		return 10

	case 3:
		switch y {
		case 0: // JP nn
			cpu.PC = cpu.fetchWord()
			cpu.WZ = cpu.PC
			return 10
		case 1: // CB prefix
			return cpu.executeCB()
		case 2: // OUT (n),A
			port := cpu.fetchByte()
			addr := uint16(port) | (uint16(cpu.A) << 8)
			cpu.IO.Out(addr, cpu.A)
			cpu.WZ = addr
			return 11
		case 3: // IN A,(n)
			port := cpu.fetchByte()
			addr := uint16(port) | (uint16(cpu.A) << 8)
			cpu.A = cpu.IO.In(addr)
			cpu.WZ = addr + 1
			return 11
		case 4: // EX (SP),HL
			val := cpu.readWord(cpu.SP)
			cpu.writeWord(cpu.SP, cpu.HL())
			cpu.SetHL(val)
			cpu.WZ = val
			return 19
		case 5: // EX DE,HL
			de := cpu.DE()
			cpu.SetDE(cpu.HL())
			cpu.SetHL(de)
			return 4
		case 6: // DI
			cpu.pendingDI = true
			return 4
		case 7: // EI
			cpu.pendingEI = true
			return 4
		}

	case 4: // CALL cc,nn
		addr := cpu.fetchWord()
		if cpu.testCondition(y) {
			cpu.push(cpu.PC)
			cpu.PC = addr
			cpu.WZ = addr
			return 17
		}
		return 10

	case 5:
		if q == 0 { // PUSH rp2
			var val uint16
			switch p {
			case 0: val = cpu.BC()
			case 1: val = cpu.DE()
			case 2: val = cpu.HL()
			case 3: val = cpu.AF()
			}
			cpu.push(val)
			return 11
		} else {
			switch p {
			case 0: // CALL nn
				addr := cpu.fetchWord()
				cpu.push(cpu.PC)
				cpu.PC = addr
				cpu.WZ = addr
				return 17
			case 1: // DD prefix
				return cpu.executeDD()
			case 2: // ED prefix
				return cpu.executeED()
			case 3: // FD prefix
				return cpu.executeFD()
			}
		}

	case 6: // ALU n
		val := cpu.fetchByte()
		switch y {
		case 0: cpu.add8(val)
		case 1: cpu.adc8(val)
		case 2: cpu.sub8(val)
		case 3: cpu.sbc8(val)
		case 4: cpu.and8(val)
		case 5: cpu.xor8(val)
		case 6: cpu.or8(val)
		case 7: cpu.cp8(val)
		}
		return 7

	case 7: // RST p*8
		cpu.push(cpu.PC)
		cpu.PC = uint16(y) * 8
		cpu.WZ = cpu.PC
		return 11
	}
	
	return 4
}

// getRegister8 returns a pointer to the 8-bit register specified by the index
func (cpu *Z80) getRegister8(index uint8) *uint8 {
	switch index {
	case 0: return &cpu.B
	case 1: return &cpu.C
	case 2: return &cpu.D
	case 3: return &cpu.E
	case 4: return &cpu.H
	case 5: return &cpu.L
	case 6: 
		// (HL) is a special case - this shouldn't be called for it
		// The caller should handle (HL) specially
		panic("getRegister8 called with index 6 (HL)")
	case 7: return &cpu.A
	default: return &cpu.A
	}
}