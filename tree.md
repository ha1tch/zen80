z80emu/
├── go.mod                      # Go module definition
├── README.md                   # Project documentation
│
├── z80/                        # Core Z80 CPU emulation
│   ├── z80.go                  # CPU state and main loop
│   ├── decode.go               # Instruction decoder
│   ├── alu.go                  # ALU operations
│   ├── prefix_cb.go            # CB-prefixed instructions
│   ├── prefix_ed.go            # ED-prefixed instructions
│   ├── prefix_ddfd.go          # DD/FD-prefixed instructions
│   └── timing_fixes.go         # Cycle timing verification
│
├── memory/                     # Memory subsystem
│   └── memory.go               # RAM, ROM, and mapped memory
│
├── io/                         # I/O subsystem
│   └── io.go                   # I/O port implementations
│
├── system/                     # System-level emulation
│   ├── timing.go               # Speed control and synchronization
│   └── spectrum.go             # ZX Spectrum system emulation
│
└── cmd/                        # Executable programs
    ├── example/
    │   └── main.go             # Basic emulator examples
    └── spectrum/
        └── main.go             # Spectrum timing tests