# zen80
#### is a Z80 Emulator written in Go

A simple instruction-stepped Z80 CPU emulator written in Go, inspired by the cycle-accurate emulation techniques described in floooh's blog posts.

## Features

- **Instruction-stepped execution**: Simple and fast, suitable for most games and applications
- **Complete instruction set**: All documented Z80 instructions including:
  - Main instructions
  - CB-prefixed (bit operations)
  - ED-prefixed (extended instructions)
  - DD/FD-prefixed (IX/IY operations)
  - DDCB/FDCB-prefixed (indexed bit operations)
- **Accurate flag handling**: Including undocumented X and Y flags
- **Interrupt support**: NMI and maskable interrupts (modes 0, 1, 2)
- **Memory and I/O interfaces**: Flexible interfaces for custom implementations
- **Clean API**: Simple to integrate into larger projects

## Project Structure

```
zen80/
├── z80/
│   ├── z80.go           # Core CPU state and main loop
│   ├── decode.go        # Instruction decoder
│   ├── alu.go           # Arithmetic and logic operations
│   ├── prefix_cb.go     # CB-prefixed instructions
│   ├── prefix_ed.go     # ED-prefixed instructions
│   └── prefix_ddfd.go   # DD/FD-prefixed instructions
├── memory/
│   └── memory.go        # Memory implementations
├── io/
│   └── io.go           # I/O port implementations
├── cmd/
│   └── example/
│       └── main.go     # Example programs
├── go.mod
└── README.md
```

## Quick Start

```go
package main

import (
    "github.com/ha1tch/zen80/z80"
    "github.com/ha1tch/zen80/memory"
    "github.com/ha1tch/zen80/io"
)

func main() {
    // Create memory and I/O
    mem := memory.NewRAM()
    io := io.NewNullIO()
    
    // Load a program
    program := []uint8{
        0x3E, 0x05,  // LD A, 5
        0x06, 0x03,  // LD B, 3
        0x80,        // ADD A, B
        0x76,        // HALT
    }
    mem.Load(0x0000, program)
    
    // Create and run CPU
    cpu := z80.New(mem, io)
    for !cpu.Halted {
        cpu.Step()
    }
    
    // Result is in register A
    fmt.Printf("Result: %d\n", cpu.A)
}
```

## Usage

### Basic CPU Control

```go
// Create CPU
cpu := z80.New(memory, io)

// Reset CPU
cpu.Reset()

// Execute one instruction
cycles := cpu.Step()

// Run until condition
cpu.Run(func() bool {
    return !shouldStop
})
```

### Memory Implementation

Implement the `MemoryInterface`:

```go
type MemoryInterface interface {
    Read(address uint16) uint8
    Write(address uint16, value uint8)
}
```

Built-in implementations:
- `RAM`: Simple 64KB RAM
- `ROM`: Read-only memory
- `MappedMemory`: ROM + RAM regions

### I/O Implementation

Implement the `IOInterface`:

```go
type IOInterface interface {
    In(port uint16) uint8
    Out(port uint16, value uint8)
}
```

Built-in implementations:
- `NullIO`: Returns 0xFF for all reads
- `SimpleIO`: Basic 256-port array
- `MappedIO`: Port handlers with callbacks

### Interrupts

```go
// Trigger interrupts
cpu.INT = true  // Maskable interrupt
cpu.NMI = true  // Non-maskable interrupt

// Interrupt modes
cpu.IM = 0  // Mode 0: Execute instruction from data bus
cpu.IM = 1  // Mode 1: RST 38H
cpu.IM = 2  // Mode 2: Vectored interrupts
```

## Design Decisions

Based on the lessons from the cycle-accurate emulation articles:

1. **Instruction-Stepped Approach**: Chosen for simplicity and adequate performance for most use cases
2. **Clean Interfaces**: Memory and I/O as interfaces allow flexible implementations
3. **No Complex Callbacks**: Simple, synchronous execution model
4. **Direct Register Access**: Public register fields for easy inspection and debugging
5. **Accurate Flag Behavior**: Including undocumented flags for compatibility

## Performance Considerations

- **Optimized for clarity over speed**: The code prioritizes readability and correctness
- **No JIT compilation**: Pure interpretation for portability
- **Suitable for**: Games, business software, educational purposes
- **May struggle with**: Timing-critical demos, exact hardware simulation

## Testing

The test suite is organised in three tiers. The distinction matters: the
instruction exercisers (ZEXDOC/ZEXALL) run for billions of cycles and are kept
separate from the fast unit tests so day-to-day `go test` stays quick.

### 1. Fast unit tests

The bulk of the suite — per-instruction behaviour, flag derivations, timing,
prefixes, interrupt modes. These run in well under a second:

```bash
go test ./z80/... ./pkg/...
```

Note that a bare `go test ./z80` will also pick up the ZEXDOC/ZEXALL harnesses
(tier 3), which run to completion by default and take minutes. To run only the
fast tests, skip them:

```bash
go test ./z80/... ./pkg/... -skip 'ZEX|ZEXALL'
```

### 2. ROM-backed opcode-coverage test

`opcov_runtime_rom_test.go` executes a real 128K ROM to exercise opcode
coverage at runtime. It needs a ROM path and a step budget, which `runtest.sh`
sets up:

```bash
./runtest.sh
```

This points `Z80_ROM_PATH` at `rom/128-0.rom` and runs with a 50M-step budget.

### 3. ZEXDOC / ZEXALL conformance exercisers

These are the standard Z80 instruction exercisers (the same ones used to
validate real hardware emulators), run under a small CP/M BDOS shim. ZEXDOC
checks documented flag behaviour; ZEXALL additionally checks the undocumented
flag bits. Each runs to completion and **takes several minutes**:

```bash
./zexdoc.sh     # documented-flags exerciser
./zexall.sh     # documented + undocumented flags
```

A passing run prints each instruction group followed by `OK`. Console output
is captured to `zexdoc.out` / `zexall.out`.

These exercisers are gated behind environment variables so they do not run by
accident as part of the fast suite. The runner scripts set them for you; the
key ones are:

| Variable | Meaning |
|----------|---------|
| `Z80_ZEX_STEPS` / `Z80_ZEXALL_STEPS` | Maximum CPU steps. **`0` means unlimited** (run to completion). The in-code default is ~2 billion, which is why a bare `go test` triggers a long run. |
| `Z80_ZEX_OUTPUT` / `Z80_ZEXALL_OUTPUT` | File to capture console output to. |
| `Z80_ZEX_PROGRESS_EVERY` | How often to print a progress line (in steps). |
| `Z80_ZEX_SILENT_LIMIT` | Steps with no new output before the harness gives up. |

The scripts run the exercisers with `-timeout=0` so Go's default 10-minute test
timeout does not cut them off. Run them on a real machine; in a heavily
constrained or sandboxed environment they may appear to hang simply because
they need the cycles to finish.

### Everything

To run the complete suite the way the runner scripts do — fast tests, the
ROM-backed test, and both exercisers — run `./runtest.sh` followed by
`./zexdoc.sh` and `./zexall.sh`.

## Future Enhancements

Potential improvements while maintaining simplicity:

1. **Basic Debugger**: Breakpoints, step debugging, register inspection
2. **Cycle Counting**: More accurate cycle counting for each instruction
3. **State Serialization**: Save/load CPU state
4. **Performance Optimizations**: Table-driven decoder, caching
5. **Test Suite**: Comprehensive instruction testing

## References

- [Z80 CPU User Manual](http://www.z80.info/z80-documented.pdf)
- [The Undocumented Z80 Documented](http://www.z80.info/z80undoc.htm)
- [floooh's Z80 Emulation Blog Posts](https://floooh.github.io/2021/12/17/cycle-stepped-z80.html)
- [Decoding Z80 Opcodes](http://www.z80.info/decoding.htm)



## Contact

Email: h@ual.fi

https://oldbytes.space/@haitchfive

## License

Copyright 2025 h@ual.fi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
