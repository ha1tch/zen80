package z80

import "testing"

// mockMemory is a simple 64K RAM that satisfies MemoryInterface.
type mockMemory struct {
	data [65536]uint8
}

func (m *mockMemory) Read(address uint16) uint8 { return m.data[address] }
func (m *mockMemory) Write(address uint16, value uint8) { m.data[address] = value }

// mockIO is a trivial port device that echoes last written value.
type mockIO struct {
	lastOut map[uint16]uint8
	inVals  map[uint16]uint8
}

func newMockIO() *mockIO {
	return &mockIO{ lastOut: make(map[uint16]uint8), inVals: make(map[uint16]uint8) }
}

func (io *mockIO) In(port uint16) uint8  { return io.inVals[port] }
func (io *mockIO) Out(port uint16, value uint8) { io.lastOut[port] = value }

// testCPU creates a CPU with empty RAM/IO and PC=0, SP=0xFFFF.
func testCPU() (*Z80, *mockMemory, *mockIO) {
	mem := &mockMemory{}
	io := newMockIO()
	cpu := New(mem, io)
	cpu.Reset()
	cpu.SP = 0xFFFF
	cpu.PC = 0x0000
	return cpu, mem, io
}

// loadProgram writes bytes at address and sets PC to that address.
func loadProgram(cpu *Z80, mem *mockMemory, addr uint16, bytes ...uint8) {
	for i, b := range bytes {
		mem.Write(addr+uint16(i), b)
	}
	cpu.PC = addr
}

// mustStep runs one instruction and logs an error if cycles <= 0.
// Returns cycles consumed.
func mustStep(t *testing.T, cpu *Z80) int {
	c := cpu.Step()
	if c <= 0 {
		t.Errorf("Step returned invalid cycles: %d", c)
	}
	return c
}

// assert helper shortcuts (non-fatal).
func assertEq[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

func assertFlag(t *testing.T, cpu *Z80, flag uint8, want bool, msg string) {
	t.Helper()
	if cpu.getFlag(flag) != want {
		t.Errorf("%s: flag 0x%02X got %v, want %v (F=%02X)", msg, flag, cpu.getFlag(flag), want, cpu.F)
	}
}
