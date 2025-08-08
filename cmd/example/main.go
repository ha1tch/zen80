// Example program demonstrating the Z80 emulator
package main

import (
	"fmt"
	"log"

	"github.com/ha1tch/zen80/io"
	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

func main() {
	// Example 1: Simple addition program
	fmt.Println("=== Example 1: Simple Addition ===")
	runSimpleAddition()

	// Example 2: Loop example
	fmt.Println("\n=== Example 2: Loop Counter ===")
	runLoopExample()

	// Example 3: Stack operations
	fmt.Println("\n=== Example 3: Stack Operations ===")
	runStackExample()

	// Example 4: I/O example
	fmt.Println("\n=== Example 4: I/O Operations ===")
	runIOExample()
}

func runSimpleAddition() {
	// Create memory and I/O
	mem := memory.NewRAM()
	io := io.NewNullIO()

	// Load a simple program: Load 5 into A, add 3
	program := []uint8{
		0x3E, 0x05, // LD A, 5
		0x06, 0x03, // LD B, 3
		0x80,       // ADD A, B
		0x76,       // HALT
	}
	mem.Load(0x0000, program)

	// Create and run CPU
	cpu := z80.New(mem, io)
	
	cycles := 0
	for !cpu.Halted && cycles < 1000 {
		c := cpu.Step()
		cycles += c
	}

	fmt.Printf("Result: A = %d (expected 8)\n", cpu.A)
	fmt.Printf("Cycles executed: %d\n", cycles)
}

func runLoopExample() {
	// Create memory and I/O
	mem := memory.NewRAM()
	io := io.NewNullIO()

	// Program: Count from 10 down to 0
	program := []uint8{
		0x06, 0x0A,       // LD B, 10
		0x3E, 0x00,       // LD A, 0
		// Loop:
		0x3C,             // INC A       (addr 0x04)
		0x10, 0xFD,       // DJNZ -3     (loop back to 0x04)
		0x76,             // HALT
	}
	mem.Load(0x0000, program)

	// Create and run CPU
	cpu := z80.New(mem, io)
	
	cycles := 0
	for !cpu.Halted && cycles < 10000 {
		c := cpu.Step()
		cycles += c
	}

	fmt.Printf("Final A = %d (should have counted 10 times)\n", cpu.A)
	fmt.Printf("Final B = %d (should be 0)\n", cpu.B)
	fmt.Printf("Cycles executed: %d\n", cycles)
}

func runStackExample() {
	// Create memory and I/O
	mem := memory.NewRAM()
	io := io.NewNullIO()

	// Program: Push values onto stack and pop them back
	program := []uint8{
		0x31, 0x00, 0x80, // LD SP, 0x8000
		0x01, 0x34, 0x12, // LD BC, 0x1234
		0x11, 0x78, 0x56, // LD DE, 0x5678
		0xC5,             // PUSH BC
		0xD5,             // PUSH DE
		0x01, 0x00, 0x00, // LD BC, 0x0000
		0x11, 0x00, 0x00, // LD DE, 0x0000
		0xD1,             // POP DE
		0xC1,             // POP BC
		0x76,             // HALT
	}
	mem.Load(0x0000, program)

	// Create and run CPU
	cpu := z80.New(mem, io)
	
	cycles := 0
	for !cpu.Halted && cycles < 1000 {
		c := cpu.Step()
		cycles += c
	}

	fmt.Printf("BC = 0x%04X (should be 0x1234)\n", cpu.BC())
	fmt.Printf("DE = 0x%04X (should be 0x5678)\n", cpu.DE())
	fmt.Printf("SP = 0x%04X (should be back to 0x8000)\n", cpu.SP)
	fmt.Printf("Cycles executed: %d\n", cycles)
}

func runIOExample() {
	// Create memory and mapped I/O
	mem := memory.NewRAM()
	mappedIO := io.NewMappedIO()

	// Set up I/O handlers
	outputBuffer := []uint8{}
	
	// Port 0x00: Output port (console)
	mappedIO.RegisterWriteHandler(0x00, func(port uint16, value uint8) {
		outputBuffer = append(outputBuffer, value)
		fmt.Printf("Output to port 0x%02X: 0x%02X ('%c')\n", port, value, value)
	})

	// Port 0x01: Input port (return sequential values)
	inputCounter := uint8(0x41) // Start with 'A'
	mappedIO.RegisterReadHandler(0x01, func(port uint16) uint8 {
		val := inputCounter
		inputCounter++
		fmt.Printf("Input from port 0x%02X: 0x%02X ('%c')\n", port, val, val)
		return val
	})

	// Program: Read from port and write to another port
	program := []uint8{
		0xDB, 0x01,       // IN A, (0x01)
		0xD3, 0x00,       // OUT (0x00), A
		0xDB, 0x01,       // IN A, (0x01)
		0xD3, 0x00,       // OUT (0x00), A
		0xDB, 0x01,       // IN A, (0x01)
		0xD3, 0x00,       // OUT (0x00), A
		0x76,             // HALT
	}
	mem.Load(0x0000, program)

	// Create and run CPU
	cpu := z80.New(mem, mappedIO)
	
	cycles := 0
	for !cpu.Halted && cycles < 1000 {
		c := cpu.Step()
		cycles += c
	}

	fmt.Printf("Output buffer: %v\n", outputBuffer)
	fmt.Printf("Cycles executed: %d\n", cycles)
}

// Helper function to disassemble a simple instruction (for debugging)
func disassemble(mem *memory.RAM, addr uint16) string {
	opcode := mem.Read(addr)
	switch opcode {
	case 0x00:
		return "NOP"
	case 0x76:
		return "HALT"
	case 0x3E:
		return fmt.Sprintf("LD A, 0x%02X", mem.Read(addr+1))
	case 0x06:
		return fmt.Sprintf("LD B, 0x%02X", mem.Read(addr+1))
	case 0x80:
		return "ADD A, B"
	case 0x3C:
		return "INC A"
	case 0x10:
		return fmt.Sprintf("DJNZ %d", int8(mem.Read(addr+1)))
	default:
		return fmt.Sprintf("DB 0x%02X", opcode)
	}
}