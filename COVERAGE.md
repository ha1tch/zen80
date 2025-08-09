## Implementation Summary Notes

### Key Gaps and Bugs Identified:
1. **IXH/IXL/IYH/IYL Register Access**: Undocumented opcodes (DD/FD with specific values) that allow treating IX/IY as separate 8-bit registers are not implemented
2. **ED Undefined Opcodes**: Many undefined ED opcodes should duplicate other ED instructions, but currently return NOP
3. **Block I/O Flags**: INI/IND/OUTI/OUTD and their repeat variants have simplified flag handling, missing complex undocumented behaviour
4. **Interrupt Mode 0**: Simplified to always execute RST 38H instead of reading instruction from data bus
5. **WZ Register Bug**: CB instructions with (HL) don't set WZ register, causing incorrect X/Y flags for BIT n,(HL)
6. **Block Transfer Y Flag Bug**: LDI/LDD/CPI/CPD and repeat variants calculate Y flag incorrectly - using `((n << 4) & FlagY)` instead of `((n & 0x02) << 4)`
7. **Cycle Verification**: `VerifyInstructionTiming()` function exists but is never called

### Correctly Implemented Features:
1. **RLCA/RRCA/RLA/RRA**: Flag handling is correct - these instructions preserve S, Z, and P/V flags
2. **JP (IX/IY)**: Works correctly despite confusing comment in `involvesHL()`
3. **Most Documented Instructions**: All standard Z80 instructions are implemented
4. **Undocumented Flags**: X and Y flag behaviour is correctly implemented
5. **DDCB/FDCB Register Copy**: Undocumented behaviour where results are copied to registers is implemented# zen80 Z80 Implementation Function Mapping

## Implementation Status Legend
- **FULL**: Complete implementation with all documented behaviour
- **NOT IMPL**: Not implemented
- **NOP**: Defaults to NOP behaviour
- **HANDLED**: Handled through special logic
- **UNDOC**: Undocumented instruction implemented
- **PARTIAL**: Partially implemented
- **SIMPLE**: Simplified implementation

## Main Instructions (Unprefixed)

| Opcode | Instruction | Coverage | Implementation | Function Location |
|--------|-------------|----------|----------------|-------------------|
| 00 | NOP | ✅ | FULL | `executeBlock0()` |
| 01 | LD BC,nn | ✅ | FULL | `executeBlock0()` case 1, q=0, p=0 |
| 02 | LD (BC),A | ✅ | FULL | `executeBlock0()` case 2, y=0 |
| 03 | INC BC | ✅ | FULL | `executeBlock0()` case 3, q=0, p=0 |
| 04 | INC B | ✅ | FULL | `executeBlock0()` case 4 + `inc8()` |
| 05 | DEC B | ✅ | FULL | `executeBlock0()` case 5 + `dec8()` |
| 06 | LD B,n | ✅ | FULL | `executeBlock0()` case 6 |
| 07 | RLCA | ✅ | FULL | `executeBlock0()` case 7, y=0 + `rlc8()` |
| 08 | EX AF,AF' | ✅ | FULL | `executeBlock0()` case 0, y=1 |
| 09 | ADD HL,BC | ✅ | FULL | `executeBlock0()` case 1, q=1 + `add16()` |
| 0A | LD A,(BC) | ✅ | FULL | `executeBlock0()` case 2, y=1 |
| 0B | DEC BC | ✅ | FULL | `executeBlock0()` case 3, q=1, p=0 |
| 0C | INC C | ✅ | FULL | `executeBlock0()` case 4 + `inc8()` |
| 0D | DEC C | ✅ | FULL | `executeBlock0()` case 5 + `dec8()` |
| 0E | LD C,n | ✅ | FULL | `executeBlock0()` case 6 |
| 0F | RRCA | ✅ | FULL | `executeBlock0()` case 7, y=1 + `rrc8()` |
| 10 | DJNZ d | ✅ | FULL | `executeBlock0()` case 0, y=2 |
| 11 | LD DE,nn | ✅ | FULL | `executeBlock0()` case 1, q=0, p=1 |
| 12 | LD (DE),A | ✅ | FULL | `executeBlock0()` case 2, y=2 |
| 13 | INC DE | ✅ | FULL | `executeBlock0()` case 3, q=0, p=1 |
| 14 | INC D | ✅ | FULL | `executeBlock0()` case 4 + `inc8()` |
| 15 | DEC D | ✅ | FULL | `executeBlock0()` case 5 + `dec8()` |
| 16 | LD D,n | ✅ | FULL | `executeBlock0()` case 6 |
| 17 | RLA | ✅ | FULL | `executeBlock0()` case 7, y=2 + `rl8()` |
| 18 | JR d | ✅ | FULL | `executeBlock0()` case 0, y=3 |
| 19 | ADD HL,DE | ✅ | FULL | `executeBlock0()` case 1, q=1 + `add16()` |
| 1A | LD A,(DE) | ✅ | FULL | `executeBlock0()` case 2, y=3 |
| 1B | DEC DE | ✅ | FULL | `executeBlock0()` case 3, q=1, p=1 |
| 1C | INC E | ✅ | FULL | `executeBlock0()` case 4 + `inc8()` |
| 1D | DEC E | ✅ | FULL | `executeBlock0()` case 5 + `dec8()` |
| 1E | LD E,n | ✅ | FULL | `executeBlock0()` case 6 |
| 1F | RRA | ✅ | FULL | `executeBlock0()` case 7, y=3 + `rr8()` |
| 20 | JR NZ,d | ✅ | FULL | `executeBlock0()` case 0, y=4 + `testCondition()` |
| 21 | LD HL,nn | ✅ | FULL | `executeBlock0()` case 1, q=0, p=2 |
| 22 | LD (nn),HL | ✅ | FULL | `executeBlock0()` case 2, y=4 |
| 23 | INC HL | ✅ | FULL | `executeBlock0()` case 3, q=0, p=2 |
| 24 | INC H | ✅ | FULL | `executeBlock0()` case 4 + `inc8()` |
| 25 | DEC H | ✅ | FULL | `executeBlock0()` case 5 + `dec8()` |
| 26 | LD H,n | ✅ | FULL | `executeBlock0()` case 6 |
| 27 | DAA | ✅ | FULL | `executeBlock0()` case 7, y=4 + `daa()` |
| 28 | JR Z,d | ✅ | FULL | `executeBlock0()` case 0, y=5 + `testCondition()` |
| 29 | ADD HL,HL | ✅ | FULL | `executeBlock0()` case 1, q=1 + `add16()` |
| 2A | LD HL,(nn) | ✅ | FULL | `executeBlock0()` case 2, y=5 |
| 2B | DEC HL | ✅ | FULL | `executeBlock0()` case 3, q=1, p=2 |
| 2C | INC L | ✅ | FULL | `executeBlock0()` case 4 + `inc8()` |
| 2D | DEC L | ✅ | FULL | `executeBlock0()` case 5 + `dec8()` |
| 2E | LD L,n | ✅ | FULL | `executeBlock0()` case 6 |
| 2F | CPL | ✅ | FULL | `executeBlock0()` case 7, y=5 |
| 30 | JR NC,d | ✅ | FULL | `executeBlock0()` case 0, y=6 + `testCondition()` |
| 31 | LD SP,nn | ✅ | FULL | `executeBlock0()` case 1, q=0, p=3 |
| 32 | LD (nn),A | ✅ | FULL | `executeBlock0()` case 2, y=6 |
| 33 | INC SP | ✅ | FULL | `executeBlock0()` case 3, q=0, p=3 |
| 34 | INC (HL) | ✅ | FULL | `executeBlock0()` case 4, y=6 + `inc8()` |
| 35 | DEC (HL) | ✅ | FULL | `executeBlock0()` case 5, y=6 + `dec8()` |
| 36 | LD (HL),n | ✅ | FULL | `executeBlock0()` case 6, y=6 |
| 37 | SCF | ✅ | FULL | `executeBlock0()` case 7, y=6 |
| 38 | JR C,d | ✅ | FULL | `executeBlock0()` case 0, y=7 + `testCondition()` |
| 39 | ADD HL,SP | ✅ | FULL | `executeBlock0()` case 1, q=1 + `add16()` |
| 3A | LD A,(nn) | ✅ | FULL | `executeBlock0()` case 2, y=7 |
| 3B | DEC SP | ✅ | FULL | `executeBlock0()` case 3, q=1, p=3 |
| 3C | INC A | ✅ | FULL | `executeBlock0()` case 4 + `inc8()` |
| 3D | DEC A | ✅ | FULL | `executeBlock0()` case 5 + `dec8()` |
| 3E | LD A,n | ✅ | FULL | `executeBlock0()` case 6 |
| 3F | CCF | ✅ | FULL | `executeBlock0()` case 7, y=7 |
| 40-75,77-7F | LD r,r' | ✅ | FULL | `executeBlock1()` |
| 76 | HALT | ✅ | FULL | `executeBlock1()` opcode=0x76 |
| 80-87 | ADD A,r | ✅ | FULL | `executeBlock2()` y=0 + `add8()` |
| 88-8F | ADC A,r | ✅ | FULL | `executeBlock2()` y=1 + `adc8()` |
| 90-97 | SUB r | ✅ | FULL | `executeBlock2()` y=2 + `sub8()` |
| 98-9F | SBC A,r | ✅ | FULL | `executeBlock2()` y=3 + `sbc8()` |
| A0-A7 | AND r | ✅ | FULL | `executeBlock2()` y=4 + `and8()` |
| A8-AF | XOR r | ✅ | FULL | `executeBlock2()` y=5 + `xor8()` |
| B0-B7 | OR r | ✅ | FULL | `executeBlock2()` y=6 + `or8()` |
| B8-BF | CP r | ✅ | FULL | `executeBlock2()` y=7 + `cp8()` |
| C0 | RET NZ | ✅ | FULL | `executeBlock3()` z=0 + `testCondition()` |
| C1 | POP BC | ✅ | FULL | `executeBlock3()` z=1, q=0, p=0 + `pop()` |
| C2 | JP NZ,nn | ✅ | FULL | `executeBlock3()` z=2 + `testCondition()` |
| C3 | JP nn | ✅ | FULL | `executeBlock3()` z=3, y=0 |
| C4 | CALL NZ,nn | ✅ | FULL | `executeBlock3()` z=4 + `testCondition()` |
| C5 | PUSH BC | ✅ | FULL | `executeBlock3()` z=5, q=0, p=0 + `push()` |
| C6 | ADD A,n | ✅ | FULL | `executeBlock3()` z=6, y=0 + `add8()` |
| C7 | RST 00H | ✅ | FULL | `executeBlock3()` z=7 + `push()` |
| C8 | RET Z | ✅ | FULL | `executeBlock3()` z=0 + `testCondition()` |
| C9 | RET | ✅ | FULL | `executeBlock3()` z=1, q=1, p=0 + `pop()` |
| CA | JP Z,nn | ✅ | FULL | `executeBlock3()` z=2 + `testCondition()` |
| CB | CB prefix | ✅ | HANDLED | `executeBlock3()` z=3, y=1 → `executeCB()` |
| CC | CALL Z,nn | ✅ | FULL | `executeBlock3()` z=4 + `testCondition()` |
| CD | CALL nn | ✅ | FULL | `executeBlock3()` z=5, q=1, p=0 + `push()` |
| CE | ADC A,n | ✅ | FULL | `executeBlock3()` z=6, y=1 + `adc8()` |
| CF | RST 08H | ✅ | FULL | `executeBlock3()` z=7 + `push()` |
| D0-D7 | RET/POP/JP/CALL nc | ✅ | FULL | `executeBlock3()` various cases |
| D8-DF | RET/EXX/JP/CALL c | ✅ | FULL | `executeBlock3()` various cases |
| D3 | OUT (n),A | ✅ | FULL | `executeBlock3()` z=3, y=2 |
| D9 | EXX | ✅ | FULL | `executeBlock3()` z=1, q=1, p=1 |
| DB | IN A,(n) | ✅ | FULL | `executeBlock3()` z=3, y=3 |
| DD | DD prefix | ✅ | HANDLED | `executeBlock3()` z=5, q=1, p=1 → `executeDD()` |
| E0-E7 | RET/POP/JP/CALL po | ✅ | FULL | `executeBlock3()` various cases |
| E8-EF | RET/JP/CALL pe | ✅ | FULL | `executeBlock3()` various cases |
| E3 | EX (SP),HL | ✅ | FULL | `executeBlock3()` z=3, y=4 |
| E9 | JP (HL) | ✅ | FULL | `executeBlock3()` z=1, q=1, p=2 |
| EB | EX DE,HL | ✅ | FULL | `executeBlock3()` z=3, y=5 |
| ED | ED prefix | ✅ | HANDLED | `executeBlock3()` z=5, q=1, p=2 → `executeED()` |
| F0-F7 | RET/POP/JP/CALL p | ✅ | FULL | `executeBlock3()` various cases |
| F8-FF | RET/LD/JP/CALL m | ✅ | FULL | `executeBlock3()` various cases |
| F3 | DI | ✅ | FULL | `executeBlock3()` z=3, y=6 |
| F9 | LD SP,HL | ✅ | FULL | `executeBlock3()` z=1, q=1, p=3 |
| FB | EI | ✅ | FULL | `executeBlock3()` z=3, y=7 |
| FD | FD prefix | ✅ | HANDLED | `executeBlock3()` z=5, q=1, p=3 → `executeFD()` |

## CB-Prefixed Instructions

| Opcode Range | Instruction | Coverage | Implementation | Function Location |
|--------------|-------------|----------|----------------|-------------------|
| CB 00-07 | RLC r | ✅ | FULL | `executeCB()` x=0, y=0 + `rlc8()` |
| CB 08-0F | RRC r | ✅ | FULL | `executeCB()` x=0, y=1 + `rrc8()` |
| CB 10-17 | RL r | ✅ | FULL | `executeCB()` x=0, y=2 + `rl8()` |
| CB 18-1F | RR r | ✅ | FULL | `executeCB()` x=0, y=3 + `rr8()` |
| CB 20-27 | SLA r | ✅ | FULL | `executeCB()` x=0, y=4 + `sla8()` |
| CB 28-2F | SRA r | ✅ | FULL | `executeCB()` x=0, y=5 + `sra8()` |
| CB 30-37 | SLL r | ✅ | UNDOC | `executeCB()` x=0, y=6 + `sll8()` |
| CB 38-3F | SRL r | ✅ | FULL | `executeCB()` x=0, y=7 + `srl8()` |
| CB 40-7F | BIT n,r | ⚠️ | PARTIAL | `executeCB()` x=1 + `bit()` (BUG: WZ not set for (HL) variant, affects flags) |
| CB 80-BF | RES n,r | ✅ | FULL | `executeCB()` x=2 |
| CB C0-FF | SET n,r | ✅ | FULL | `executeCB()` x=3 |

## ED-Prefixed Instructions

| Opcode | Instruction | Coverage | Implementation | Function Location |
|--------|-------------|----------|----------------|-------------------|
| ED 40 | IN B,(C) | ✅ | FULL | `executeED()` x=1, z=0, y=0 |
| ED 41 | OUT (C),B | ✅ | FULL | `executeED()` x=1, z=1, y=0 |
| ED 42 | SBC HL,BC | ✅ | FULL | `executeED()` x=1, z=2, q=0 + `sbc16()` |
| ED 43 | LD (nn),BC | ✅ | FULL | `executeED()` x=1, z=3, q=0 |
| ED 44 | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 45 | RETN | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 46 | IM 0 | ✅ | FULL | `executeED()` x=1, z=6, y=0 |
| ED 47 | LD I,A | ✅ | FULL | `executeED()` x=1, z=7, y=0 |
| ED 48 | IN C,(C) | ✅ | FULL | `executeED()` x=1, z=0, y=1 |
| ED 49 | OUT (C),C | ✅ | FULL | `executeED()` x=1, z=1, y=1 |
| ED 4A | ADC HL,BC | ✅ | FULL | `executeED()` x=1, z=2, q=1 + `adc16()` |
| ED 4B | LD BC,(nn) | ✅ | FULL | `executeED()` x=1, z=3, q=1 |
| ED 4C | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 4D | RETI | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 4E | IM 0/1 | ✅ | FULL | `executeED()` x=1, z=6, y=1 |
| ED 4F | LD R,A | ✅ | FULL | `executeED()` x=1, z=7, y=1 |
| ED 50 | IN D,(C) | ✅ | FULL | `executeED()` x=1, z=0, y=2 |
| ED 51 | OUT (C),D | ✅ | FULL | `executeED()` x=1, z=1, y=2 |
| ED 52 | SBC HL,DE | ✅ | FULL | `executeED()` x=1, z=2, q=0 + `sbc16()` |
| ED 53 | LD (nn),DE | ✅ | FULL | `executeED()` x=1, z=3, q=0 |
| ED 54 | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 55 | RETN | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 56 | IM 1 | ✅ | FULL | `executeED()` x=1, z=6, y=2 |
| ED 57 | LD A,I | ✅ | FULL | `executeED()` x=1, z=7, y=2 |
| ED 58 | IN E,(C) | ✅ | FULL | `executeED()` x=1, z=0, y=3 |
| ED 59 | OUT (C),E | ✅ | FULL | `executeED()` x=1, z=1, y=3 |
| ED 5A | ADC HL,DE | ✅ | FULL | `executeED()` x=1, z=2, q=1 + `adc16()` |
| ED 5B | LD DE,(nn) | ✅ | FULL | `executeED()` x=1, z=3, q=1 |
| ED 5C | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 5D | RETN | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 5E | IM 2 | ✅ | FULL | `executeED()` x=1, z=6, y=3 |
| ED 5F | LD A,R | ✅ | FULL | `executeED()` x=1, z=7, y=3 |
| ED 60 | IN H,(C) | ✅ | FULL | `executeED()` x=1, z=0, y=4 |
| ED 61 | OUT (C),H | ✅ | FULL | `executeED()` x=1, z=1, y=4 |
| ED 62 | SBC HL,HL | ✅ | FULL | `executeED()` x=1, z=2, q=0 + `sbc16()` |
| ED 63 | LD (nn),HL | ✅ | FULL | `executeED()` x=1, z=3, q=0 |
| ED 64 | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 65 | RETN | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 66 | IM 0 | ✅ | FULL | `executeED()` x=1, z=6, y=4 |
| ED 67 | RRD | ✅ | FULL | `executeED()` x=1, z=7, y=4 + `rrd()` |
| ED 68 | IN L,(C) | ✅ | FULL | `executeED()` x=1, z=0, y=5 |
| ED 69 | OUT (C),L | ✅ | FULL | `executeED()` x=1, z=1, y=5 |
| ED 6A | ADC HL,HL | ✅ | FULL | `executeED()` x=1, z=2, q=1 + `adc16()` |
| ED 6B | LD HL,(nn) | ✅ | FULL | `executeED()` x=1, z=3, q=1 |
| ED 6C | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 6D | RETN | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 6E | IM 0/1 | ✅ | FULL | `executeED()` x=1, z=6, y=5 |
| ED 6F | RLD | ✅ | FULL | `executeED()` x=1, z=7, y=5 + `rld()` |
| ED 70 | IN (C) | ✅ | FULL | `executeED()` x=1, z=0, y=6 |
| ED 71 | OUT (C),0 | ✅ | FULL | `executeED()` x=1, z=1, y=6 |
| ED 72 | SBC HL,SP | ✅ | FULL | `executeED()` x=1, z=2, q=0 + `sbc16()` |
| ED 73 | LD (nn),SP | ✅ | FULL | `executeED()` x=1, z=3, q=0 |
| ED 74 | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 75 | RETN | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 76 | IM 1 | ✅ | FULL | `executeED()` x=1, z=6, y=6 |
| ED 77 | NOP | ✅ | NOP | `executeED()` x=1, z=7, y=6 |
| ED 78 | IN A,(C) | ✅ | FULL | `executeED()` x=1, z=0, y=7 |
| ED 79 | OUT (C),A | ✅ | FULL | `executeED()` x=1, z=1, y=7 |
| ED 7A | ADC HL,SP | ✅ | FULL | `executeED()` x=1, z=2, q=1 + `adc16()` |
| ED 7B | LD SP,(nn) | ✅ | FULL | `executeED()` x=1, z=3, q=1 |
| ED 7C | NEG | ✅ | FULL | `executeED()` x=1, z=4 + `neg()` |
| ED 7D | RETN | ✅ | FULL | `executeED()` x=1, z=5 |
| ED 7E | IM 2 | ✅ | FULL | `executeED()` x=1, z=6, y=7 |
| ED 7F | NOP | ✅ | NOP | `executeED()` x=1, z=7, y=7 |
| ED A0 | LDI | ⚠️ | PARTIAL | `executeBlockInstruction()` → `ldi()` (BUG: Y flag calculation incorrect) |
| ED A1 | CPI | ⚠️ | PARTIAL | `executeBlockInstruction()` → `cpi()` (BUG: Y flag calculation incorrect) |
| ED A2 | INI | ⚠️ | SIMPLE | `executeBlockInstruction()` → `ini()` |
| ED A3 | OUTI | ⚠️ | SIMPLE | `executeBlockInstruction()` → `outi()` |
| ED A8 | LDD | ⚠️ | PARTIAL | `executeBlockInstruction()` → `ldd()` (BUG: Y flag calculation incorrect) |
| ED A9 | CPD | ⚠️ | PARTIAL | `executeBlockInstruction()` → `cpd()` (BUG: Y flag calculation incorrect) |
| ED AA | IND | ⚠️ | SIMPLE | `executeBlockInstruction()` → `ind()` |
| ED AB | OUTD | ⚠️ | SIMPLE | `executeBlockInstruction()` → `outd()` |
| ED B0 | LDIR | ⚠️ | PARTIAL | `executeBlockInstruction()` → `ldir()` (BUG: Y flag calculation incorrect) |
| ED B1 | CPIR | ⚠️ | PARTIAL | `executeBlockInstruction()` → `cpir()` (BUG: Y flag calculation incorrect) |
| ED B2 | INIR | ⚠️ | SIMPLE | `executeBlockInstruction()` → `inir()` |
| ED B3 | OTIR | ⚠️ | SIMPLE | `executeBlockInstruction()` → `otir()` |
| ED B8 | LDDR | ⚠️ | PARTIAL | `executeBlockInstruction()` → `lddr()` (BUG: Y flag calculation incorrect) |
| ED B9 | CPDR | ⚠️ | PARTIAL | `executeBlockInstruction()` → `cpdr()` (BUG: Y flag calculation incorrect) |
| ED BA | INDR | ⚠️ | SIMPLE | `executeBlockInstruction()` → `indr()` |
| ED BB | OTDR | ⚠️ | SIMPLE | `executeBlockInstruction()` → `otdr()` |
| ED others | Undefined | ❌ | NOP | `executeED()` default return 8 (should be duplicates of other ED opcodes) |

## DD-Prefixed (IX) Instructions

| Opcode | Instruction | Coverage | Implementation | Function Location |
|--------|-------------|----------|----------------|-------------------|
| DD 09 | ADD IX,BC | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 19 | ADD IX,DE | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 21 | LD IX,nn | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 22 | LD (nn),IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 23 | INC IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 29 | ADD IX,IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 2A | LD IX,(nn) | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 2B | DEC IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 34 | INC (IX+d) | ✅ | FULL | `executeDD()` special case handling |
| DD 35 | DEC (IX+d) | ✅ | FULL | `executeDD()` special case handling |
| DD 36 | LD (IX+d),n | ✅ | FULL | `executeDD()` special case handling |
| DD 39 | ADD IX,SP | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD 44-45,4C-4D,54-55,5C-5D,60-65,67-6D | LD with IXH/IXL | ❌ | NOT IMPL | Should allow IXH/IXL as registers (undocumented) |
| DD 46-7E | LD r,(IX+d) | ✅ | FULL | `executeDD()` special case handling |
| DD 70-77 | LD (IX+d),r | ✅ | FULL | `executeDD()` special case handling |
| DD 84-85,8C-8D,94-95,9C-9D,A4-A5,AC-AD,B4-B5,BC-BD | ALU with IXH/IXL | ❌ | NOT IMPL | Should allow IXH/IXL as registers (undocumented) |
| DD 86-BE | ALU ops (IX+d) | ✅ | FULL | `executeDD()` special case handling |
| DD CB | DDCB prefix | ✅ | HANDLED | `executeDD()` → `executeDDCB()` |
| DD DD | Double DD | ✅ | NOP | `executeDD()` returns 4 |
| DD E1 | POP IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD E3 | EX (SP),IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD E5 | PUSH IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD E9 | JP (IX) | ✅ | FULL | `executeDDFDInstruction()` with HL substitution (despite confusing comment) |
| DD ED | ED after DD | ✅ | HANDLED | `executeDD()` → `executeED()` + 4 |
| DD F9 | LD SP,IX | ✅ | FULL | `executeDDFDInstruction()` with HL substitution |
| DD FD | FD after DD | ✅ | HANDLED | `executeDD()` → `executeFD()` + 4 |

## FD-Prefixed (IY) Instructions

| Opcode | Instruction | Coverage | Implementation | Function Location |
|--------|-------------|----------|----------------|-------------------|
| FD 44-45,4C-4D,54-55,5C-5D,60-65,67-6D | LD with IYH/IYL | ❌ | NOT IMPL | Should allow IYH/IYL as registers (undocumented) |
| FD 84-85,8C-8D,94-95,9C-9D,A4-A5,AC-AD,B4-B5,BC-BD | ALU with IYH/IYL | ❌ | NOT IMPL | Should allow IYH/IYL as registers (undocumented) |
| FD 09-F9 | All IY ops | ✅ | FULL | `executeFD()` mirrors `executeDD()` logic |
| FD CB | FDCB prefix | ✅ | HANDLED | `executeFD()` → `executeFDCB()` |
| FD DD | DD after FD | ✅ | HANDLED | `executeFD()` → `executeDD()` + 4 |
| FD ED | ED after FD | ✅ | HANDLED | `executeFD()` → `executeED()` + 4 |
| FD FD | Double FD | ✅ | NOP | `executeFD()` returns 4 |

## DDCB-Prefixed Instructions

| Opcode Range | Instruction | Coverage | Implementation | Function Location |
|--------------|-------------|----------|----------------|-------------------|
| DDCB d 00-3F | Shift/Rotate (IX+d) | ✅ | FULL | `executeDDFDCBOperation()` x=0 |
| DDCB d 40-7F | BIT n,(IX+d) | ✅ | FULL | `executeDDFDCBOperation()` x=1 |
| DDCB d 80-BF | RES n,(IX+d) | ✅ | FULL | `executeDDFDCBOperation()` x=2 |
| DDCB d C0-FF | SET n,(IX+d) | ✅ | FULL | `executeDDFDCBOperation()` x=3 |
| DDCB undoc | Register copy | ✅ | UNDOC | `executeDDFDCBOperation()` copies to register |

## FDCB-Prefixed Instructions

| Opcode Range | Instruction | Coverage | Implementation | Function Location |
|--------------|-------------|----------|----------------|-------------------|
| FDCB d 00-FF | All (IY+d) bit ops | ✅ | FULL | `executeDDFDCBOperation()` |
| FDCB undoc | Register copy | ✅ | UNDOC | `executeDDFDCBOperation()` copies to register |

## Support Functions

| Function | Purpose | File | Notes |
|----------|---------|------|-------|
| `execute()` | Main instruction decoder | decode.go | |
| `executeBlock0()` | Opcodes 0x00-0x3F decoder | decode.go | |
| `executeBlock1()` | Opcodes 0x40-0x7F decoder | decode.go | |
| `executeBlock2()` | Opcodes 0x80-0xBF decoder | decode.go | |
| `executeBlock3()` | Opcodes 0xC0-0xFF decoder | decode.go | |
| `executeCB()` | CB prefix handler | prefix_cb.go | BUG: WZ not set for (HL) operations |
| `executeED()` | ED prefix handler | prefix_ed.go | |
| `executeDD()` | DD prefix handler | prefix_ddfd.go | Missing IXH/IXL register access |
| `executeFD()` | FD prefix handler | prefix_ddfd.go | Missing IYH/IYL register access |
| `executeDDCB()` | DDCB prefix handler | prefix_ddfd.go |
| `executeFDCB()` | FDCB prefix handler | prefix_ddfd.go |
| `executeDDFDInstruction()` | IX/IY instruction executor | prefix_ddfd.go |
| `executeDDFDCBOperation()` | DDCB/FDCB operation executor | prefix_ddfd.go |
| `executeBlockInstruction()` | ED block instruction dispatcher | prefix_ed.go |
| `add8()`, `adc8()`, `sub8()`, `sbc8()` | 8-bit arithmetic | alu.go |
| `and8()`, `or8()`, `xor8()`, `cp8()` | 8-bit logic | alu.go |
| `inc8()`, `dec8()` | 8-bit increment/decrement | alu.go |
| `add16()`, `adc16()`, `sbc16()` | 16-bit arithmetic | alu.go |
| `rlc8()`, `rrc8()`, `rl8()`, `rr8()` | Rotation operations | alu.go |
| `sla8()`, `sra8()`, `sll8()`, `srl8()` | Shift operations | alu.go |
| `daa()` | Decimal adjust | alu.go |
| `neg()` | Negate accumulator | prefix_ed.go |
| `rrd()`, `rld()` | Rotate decimal | prefix_ed.go |
| `bit()` | Bit test | prefix_cb.go |
| `ldi()`, `ldd()`, `ldir()`, `lddr()` | Block transfer | prefix_ed.go | BUG: Y flag calculation `((n << 4) & FlagY)` should be `((n & 0x02) << 4)` |
| `cpi()`, `cpd()`, `cpir()`, `cpdr()` | Block search | prefix_ed.go | BUG: Y flag calculation `((n << 4) & FlagY)` should be `((n & 0x02) << 4)` |
| `ini()`, `ind()`, `inir()`, `indr()` | Block input | prefix_ed.go | Simplified flag handling |
| `outi()`, `outd()`, `otir()`, `otdr()` | Block output | prefix_ed.go | Simplified flag handling |
| `testCondition()` | Conditional test | z80.go |
| `getRegister8()` | Register access | decode.go |
| `involvesHL()` | HL instruction check | prefix_ddfd.go | Has confusing comment about JP (HL) |
| `parity()` | Parity calculation | alu.go | |
| `push()`, `pop()` | Stack operations | z80.go | |
| `fetchByte()`, `fetchWord()` | Memory fetch | z80.go | |
| `readWord()`, `writeWord()` | Memory access | z80.go | |
| `handleInterrupts()` | Interrupt handler | z80.go | Mode 0 simplified |
| `VerifyInstructionTiming()` | Cycle verification | timing_fixes.go | Never called in code |

## Special Registers and Features

| Feature | Coverage | Implementation | Function Location |
|---------|----------|----------------|-------------------|
| A,F,B,C,D,E,H,L | ✅ | FULL | Direct fields in Z80 struct |
| A',F',B',C',D',E',H',L' | ✅ | FULL | Direct fields in Z80 struct |
| IXH,IXL,IYH,IYL | ❌ | NOT IMPL | Direct fields exist but not opcode accessible (DD/FD 44,45,4C,4D,54,55,5C,5D,etc. missing) |
| I,R | ✅ | FULL | Direct fields in Z80 struct |
| SP,PC | ✅ | FULL | Direct fields in Z80 struct |
| WZ (MEMPTR) | ✅ | FULL | Direct field in Z80 struct |
| IFF1,IFF2 | ✅ | FULL | Direct fields in Z80 struct |
| IM | ✅ | FULL | Direct field in Z80 struct |
| Halted | ✅ | FULL | Direct field in Z80 struct |
| pendingEI,pendingDI | ✅ | FULL | Direct fields in Z80 struct |
| NMI,INT | ✅ | FULL | Direct fields in Z80 struct |
| nmiEdge | ✅ | FULL | Direct field in Z80 struct |

## Interrupt Implementation

| Feature | Coverage | Implementation | Function Location |
|---------|----------|----------------|-------------------|
| NMI handling | ✅ | FULL | `handleInterrupts()` in z80.go |
| INT Mode 0 | ⚠️ | SIMPLE | `handleInterrupts()` defaults to RST 38H (should execute data bus instruction) |
| INT Mode 1 | ✅ | FULL | `handleInterrupts()` jumps to 0x0038 |
| INT Mode 2 | ✅ | FULL | `handleInterrupts()` vectored via I register |
| EI delay | ✅ | FULL | `Step()` with pendingEI flag |
| DI delay | ✅ | FULL | `Step()` with pendingDI flag |

## Flag Implementation

| Flag | Bit | Coverage | Implementation | Functions |
|------|-----|----------|----------------|-----------|
| Carry (C) | 0 | ✅ | FULL | All ALU functions |
| Add/Subtract (N) | 1 | ✅ | FULL | All ALU functions |
| Parity/Overflow (P/V) | 2 | ✅ | FULL | Context-dependent in ALU |
| X (undocumented) | 3 | ✅ | UNDOC | All ALU functions |
| Half-carry (H) | 4 | ✅ | FULL | All ALU functions |
| Y (undocumented) | 5 | ✅ | UNDOC | All ALU functions |
| Zero (Z) | 6 | ✅ | FULL | All ALU functions |
| Sign (S) | 7 | ✅ | FULL | All ALU functions |
