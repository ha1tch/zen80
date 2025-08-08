# zen80 Test Suite

Test coverage for the zen80 Z80 emulator and basic Spectrum system emulation.

## Project Scope

zen80 is an instruction-stepped Z80 emulator written in Go. It executes Z80 instructions accurately but does not provide cycle-accurate timing within instructions.

## Current Implementation Status

### Working Features

#### Z80 CPU Core
- All documented Z80 instructions
- All undocumented instructions
- Correct flag calculations including undocumented X/Y flags
- All register operations including shadow registers
- Stack operations (PUSH/POP, CALL/RET)
- Interrupt handling (IM 0/1/2 and NMI)
- Block operations (LDIR, CPIR, etc.)
- IX/IY indexed addressing with DD/FD prefixes
- CB/ED/DDCB/FDCB prefix instructions
- Accurate instruction cycle counting

#### Basic System Features
- Memory: 16K ROM + 48K RAM layout
- CPU speed: ~3.5 MHz (±5% accuracy)
- Frame synchronization: 50 FPS
- Speed control: 0.5x, 1x, 2x multipliers
- Interrupt generation: 50Hz at VBlank
- Basic I/O port framework

### Partial Implementations

#### Limited Spectrum Features
- Keyboard matrix (simplified port decoding)
- Border color tracking (no display output)
- Screen memory area (treated as regular RAM)
- ULA port (partial implementation)

### Not Implemented

#### Beyond Current Scope
- Video generation/rendering
- Sound (beeper or AY chip)
- Tape loading/saving
- Memory contention
- 128K memory banking
- Peripheral devices

## Test Categories

### Z80 Core Tests (`z80_test.go`)

Tests that verify working features:
- `TestUndocumentedFlags` - X/Y flag behavior
- `TestBlockInstructions` - LDIR, CPIR, LDDR operations
- `TestInterrupts` - Interrupt modes and NMI
- `TestIndexedInstructions` - IX/IY operations
- `TestDAA` - BCD arithmetic
- `TestStackOperations` - Stack manipulation
- `TestEdgeCases` - Boundary conditions

### System Tests (`spectrum_test.go`)

Tests with varying validity:
- `TestSpectrumMemoryBanking` - ✓ Works (basic ROM/RAM layout)
- `TestSpectrumIO` - ✓ Works (basic I/O)
- `TestSpectrumInterruptTiming` - ✓ Works (50Hz interrupts)
- `TestSpectrumTiming` - ✓ Works (3.5MHz speed)
- `TestSpectrumContention` - ✗ Skipped (not implemented)
- `TestScreenMemoryMapping` - ~ Passes (but just tests RAM)
- `TestKeyboardMatrix` - ~ Works (simplified implementation)

### Integration Tests (`integration_test.go`)

Complex program execution tests:
- `TestComplexProgram` - Game loop simulation
- `TestSelfModifyingCode` - Code modification
- `TestRecursion` - Stack-based algorithms
- `TestIOEcho` - Port I/O programs
- `TestConditionalExecution` - Branch logic
- `TestStringOperations` - String manipulation

## Running Tests

```bash
# All tests
go test ./test -v

# CPU core tests only
go test ./test -run "Test.*Block|Test.*DAA|Test.*Stack" -v

# System timing tests
go test ./test -run "TestSpectrumTiming" -v

# Integration tests
go test ./test -run "TestComplex|TestSelf" -v

# Benchmarks
go test ./test -bench=. -benchtime=10s

# With coverage
go test ./test -cover
```

## What You Can Do With zen80

### Supported Use Cases
- Execute Z80 assembly programs
- Test instruction behavior
- Debug Z80 code
- Run computational programs
- Benchmark instruction performance
- Learn Z80 architecture

### Unsupported Use Cases
- Run Spectrum games (no display)
- Load software from tape files
- Generate video output
- Produce sound
- Emulate timing-critical demos

## Test Coverage

| Component | Coverage | Status |
|-----------|----------|--------|
| Z80 Instructions | 95% | Working |
| Z80 Flags | 90% | Working |
| Z80 Interrupts | 80% | Working |
| Instruction Timing | 100% | Working |
| Spectrum Memory | 60% | Basic |
| Spectrum I/O | 20% | Limited |
| Spectrum Video | 0% | Not implemented |
| Spectrum Sound | 0% | Not implemented |

## Performance

Measured on modern hardware:
- Instruction throughput: ~150M instructions/second
- Effective CPU speed: 3.3-3.5 MHz
- Frame execution: >1000 frames/second (no rendering)

## Adding Tests

New tests should:
1. Test actual implemented features
2. Use table-driven test patterns
3. Include clear failure messages
4. Skip unimplemented features with `t.Skip()`

Example:
```go
func TestNewFeature(t *testing.T) {
    if !featureImplemented {
        t.Skip("Feature not yet implemented")
    }
    // Test implementation
}
```

