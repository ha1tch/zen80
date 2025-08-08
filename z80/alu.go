package z80

// ALU operations for the Z80

// add8 performs 8-bit addition
func (z *Z80) add8(val uint8) {
	result := uint16(z.A) + uint16(val)
	halfCarry := (z.A&0x0F + val&0x0F) > 0x0F
	overflow := ((z.A^val)&0x80 == 0) && ((z.A^uint8(result))&0x80 != 0)
	
	z.A = uint8(result)
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, result > 0xFF)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// adc8 performs 8-bit addition with carry
func (z *Z80) adc8(val uint8) {
	carry := uint16(0)
	if z.getFlag(FlagC) {
		carry = 1
	}
	result := uint16(z.A) + uint16(val) + carry
	halfCarry := (z.A&0x0F + val&0x0F + uint8(carry)) > 0x0F
	overflow := ((z.A^val)&0x80 == 0) && ((z.A^uint8(result))&0x80 != 0)
	
	z.A = uint8(result)
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, result > 0xFF)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// sub8 performs 8-bit subtraction
func (z *Z80) sub8(val uint8) {
	result := int16(z.A) - int16(val)
	halfCarry := (int8(z.A&0x0F) - int8(val&0x0F)) < 0
	overflow := ((z.A^val)&0x80 != 0) && ((z.A^uint8(result))&0x80 != 0)
	
	z.A = uint8(result)
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, true)
	z.setFlag(FlagC, result < 0)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// sbc8 performs 8-bit subtraction with carry
func (z *Z80) sbc8(val uint8) {
	carry := int16(0)
	if z.getFlag(FlagC) {
		carry = 1
	}
	result := int16(z.A) - int16(val) - carry
	halfCarry := (int8(z.A&0x0F) - int8(val&0x0F) - int8(carry)) < 0
	overflow := ((z.A^val)&0x80 != 0) && ((z.A^uint8(result))&0x80 != 0)
	
	z.A = uint8(result)
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, true)
	z.setFlag(FlagC, result < 0)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// and8 performs bitwise AND
func (z *Z80) and8(val uint8) {
	z.A &= val
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, true)
	z.setFlag(FlagPV, parity(z.A))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, false)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// xor8 performs bitwise XOR
func (z *Z80) xor8(val uint8) {
	z.A ^= val
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(z.A))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, false)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// or8 performs bitwise OR
func (z *Z80) or8(val uint8) {
	z.A |= val
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(z.A))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, false)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// cp8 performs comparison (A - val without storing result)
func (z *Z80) cp8(val uint8) {
	result := int16(z.A) - int16(val)
	halfCarry := (int8(z.A&0x0F) - int8(val&0x0F)) < 0
	overflow := ((z.A^val)&0x80 != 0) && ((z.A^uint8(result))&0x80 != 0)
	
	z.setFlag(FlagS, uint8(result)&0x80 != 0)
	z.setFlag(FlagZ, uint8(result) == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, true)
	z.setFlag(FlagC, result < 0)
	z.setFlag(FlagX, val&FlagX != 0)
	z.setFlag(FlagY, val&FlagY != 0)
}

// inc8 performs 8-bit increment
func (z *Z80) inc8(val uint8) uint8 {
	result := val + 1
	halfCarry := (val&0x0F + 1) > 0x0F
	overflow := val == 0x7F
	
	// Preserve carry flag, update all others
	oldCarry := z.getFlag(FlagC)
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, oldCarry)  // Restore carry
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// dec8 performs 8-bit decrement
func (z *Z80) dec8(val uint8) uint8 {
	result := val - 1
	halfCarry := (int8(val&0x0F) - 1) < 0
	overflow := val == 0x80
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, true)
	// C flag is not affected
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// add16 performs 16-bit addition (ADD HL,rp)
func (z *Z80) add16(val1, val2 uint16) uint16 {
	result := uint32(val1) + uint32(val2)
	halfCarry := (val1&0x0FFF + val2&0x0FFF) > 0x0FFF
	
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, result > 0xFFFF)
	
	res16 := uint16(result)
	z.setFlag(FlagX, uint8(res16>>8)&FlagX != 0)
	z.setFlag(FlagY, uint8(res16>>8)&FlagY != 0)
	z.WZ = val1 + 1
	
	return res16
}

// adc16 performs 16-bit addition with carry (ADC HL,rp)
func (z *Z80) adc16(val1, val2 uint16) uint16 {
	carry := uint32(0)
	if z.getFlag(FlagC) {
		carry = 1
	}
	result := uint32(val1) + uint32(val2) + carry
	halfCarry := (val1&0x0FFF + val2&0x0FFF + uint16(carry)) > 0x0FFF
	overflow := ((val1^val2)&0x8000 == 0) && ((val1^uint16(result))&0x8000 != 0)
	
	res16 := uint16(result)
	
	z.setFlag(FlagS, res16&0x8000 != 0)
	z.setFlag(FlagZ, res16 == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, result > 0xFFFF)
	z.setFlag(FlagX, uint8(res16>>8)&FlagX != 0)
	z.setFlag(FlagY, uint8(res16>>8)&FlagY != 0)
	z.WZ = val1 + 1
	
	return res16
}

// sbc16 performs 16-bit subtraction with carry (SBC HL,rp)
func (z *Z80) sbc16(val1, val2 uint16) uint16 {
	carry := int32(0)
	if z.getFlag(FlagC) {
		carry = 1
	}
	result := int32(val1) - int32(val2) - carry
	halfCarry := (int16(val1&0x0FFF) - int16(val2&0x0FFF) - int16(carry)) < 0
	overflow := ((val1^val2)&0x8000 != 0) && ((val1^uint16(result))&0x8000 != 0)
	
	res16 := uint16(result)
	
	z.setFlag(FlagS, res16&0x8000 != 0)
	z.setFlag(FlagZ, res16 == 0)
	z.setFlag(FlagH, halfCarry)
	z.setFlag(FlagPV, overflow)
	z.setFlag(FlagN, true)
	z.setFlag(FlagC, result < 0)
	z.setFlag(FlagX, uint8(res16>>8)&FlagX != 0)
	z.setFlag(FlagY, uint8(res16>>8)&FlagY != 0)
	z.WZ = val1 + 1
	
	return res16
}

// Rotation and shift operations

// rlc8 rotates left circular
func (z *Z80) rlc8(val uint8) uint8 {
	result := (val << 1) | (val >> 7)
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x80 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// rrc8 rotates right circular
func (z *Z80) rrc8(val uint8) uint8 {
	result := (val >> 1) | (val << 7)
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x01 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// rl8 rotates left through carry
func (z *Z80) rl8(val uint8) uint8 {
	carry := uint8(0)
	if z.getFlag(FlagC) {
		carry = 1
	}
	result := (val << 1) | carry
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x80 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// rr8 rotates right through carry
func (z *Z80) rr8(val uint8) uint8 {
	carry := uint8(0)
	if z.getFlag(FlagC) {
		carry = 0x80
	}
	result := (val >> 1) | carry
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x01 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// sla8 shifts left arithmetic
func (z *Z80) sla8(val uint8) uint8 {
	result := val << 1
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x80 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// sra8 shifts right arithmetic
func (z *Z80) sra8(val uint8) uint8 {
	result := (val >> 1) | (val & 0x80)
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x01 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// sll8 shifts left logical (undocumented)
func (z *Z80) sll8(val uint8) uint8 {
	result := (val << 1) | 1
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x80 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// srl8 shifts right logical
func (z *Z80) srl8(val uint8) uint8 {
	result := val >> 1
	
	z.setFlag(FlagS, result&0x80 != 0)
	z.setFlag(FlagZ, result == 0)
	z.setFlag(FlagH, false)
	z.setFlag(FlagPV, parity(result))
	z.setFlag(FlagN, false)
	z.setFlag(FlagC, val&0x01 != 0)
	z.setFlag(FlagX, result&FlagX != 0)
	z.setFlag(FlagY, result&FlagY != 0)
	
	return result
}

// daa performs decimal adjust after addition
func (z *Z80) daa() {
	temp := z.A
	diff := uint8(0)
	carryOut := z.getFlag(FlagC)
	
	if z.getFlag(FlagN) {
		// After subtraction
		if z.getFlag(FlagH) {
			diff = 0x06
		}
		if z.getFlag(FlagC) {
			diff |= 0x60
		}
		z.A -= diff
	} else {
		// After addition
		if z.getFlag(FlagH) || (temp&0x0F) > 9 {
			diff = 0x06
		}
		if z.getFlag(FlagC) || temp > 0x99 {
			diff |= 0x60
			carryOut = true
		}
		z.A += diff
	}
	
	z.setFlag(FlagS, z.A&0x80 != 0)
	z.setFlag(FlagZ, z.A == 0)
	z.setFlag(FlagH, false) // H is always cleared after DAA
	z.setFlag(FlagPV, parity(z.A))
	z.setFlag(FlagC, carryOut)
	z.setFlag(FlagX, z.A&FlagX != 0)
	z.setFlag(FlagY, z.A&FlagY != 0)
}

// Helper function to calculate parity
func parity(val uint8) bool {
	count := 0
	for i := 0; i < 8; i++ {
		if val&(1<<i) != 0 {
			count++
		}
	}
	return count%2 == 0
}