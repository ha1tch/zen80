package z80

import "testing"

// TestDebugMemReadHookFires verifies DebugMemReadHook receives the address
// and value of every memory read, including opcode fetches -- the hook
// does not distinguish fetch reads from operand reads (see its doc
// comment on the var itself).
func TestDebugMemReadHookFires(t *testing.T) {
	cpu, mem, _ := testCPU()
	// 3E 42 = LD A,0x42: one opcode-fetch read (0x3E) + one operand read (0x42).
	loadProgram(cpu, mem, 0x1000, 0x3E, 0x42)

	type read struct {
		addr uint16
		val  uint8
	}
	var reads []read
	DebugMemReadHook = func(addr uint16, val uint8) {
		reads = append(reads, read{addr, val})
	}
	defer func() { DebugMemReadHook = nil }()

	mustStep(t, cpu)

	if len(reads) != 2 {
		t.Fatalf("got %d reads, want 2 (opcode fetch + operand): %+v", len(reads), reads)
	}
	assertEq(t, reads[0].addr, uint16(0x1000), "first read address (opcode fetch)")
	assertEq(t, reads[0].val, uint8(0x3E), "first read value (opcode)")
	assertEq(t, reads[1].addr, uint16(0x1001), "second read address (operand fetch)")
	assertEq(t, reads[1].val, uint8(0x42), "second read value (operand)")
}

// TestDebugMemReadHookSilentWhenNil confirms a Step behaves identically
// with the hook unset -- the nil-default costs nothing observable.
func TestDebugMemReadHookSilentWhenNil(t *testing.T) {
	cpu, mem, _ := testCPU()
	loadProgram(cpu, mem, 0x0000, 0x00) // NOP
	DebugMemReadHook = nil
	c := mustStep(t, cpu)
	assertEq(t, c, 4, "NOP cycles unaffected by nil hook")
}

// TestDebugIOOutHookFires verifies DebugIOOutHook receives the exact port
// and value the CPU is writing, before the write reaches the device --
// and that the device still receives it (the hook observes, not
// intercepts).
func TestDebugIOOutHookFires(t *testing.T) {
	cpu, mem, io := testCPU()
	// 3E 55 = LD A,0x55; D3 FE = OUT (0xFE),A -- port = A<<8|n = 0x55FE
	// (decode.go: addr := uint16(port) | (uint16(cpu.A) << 8)).
	loadProgram(cpu, mem, 0x0000, 0x3E, 0x55, 0xD3, 0xFE)

	var fired bool
	var gotPort uint16
	var gotVal uint8
	DebugIOOutHook = func(port uint16, val uint8) {
		fired = true
		gotPort = port
		gotVal = val
	}
	defer func() { DebugIOOutHook = nil }()

	mustStep(t, cpu) // LD A,55
	mustStep(t, cpu) // OUT (FE),A

	if !fired {
		t.Fatal("DebugIOOutHook did not fire for OUT (n),A")
	}
	assertEq(t, gotPort, uint16(0x55FE), "port seen by hook")
	assertEq(t, gotVal, uint8(0x55), "value seen by hook")
	assertEq(t, io.lastOut[0x55FE], uint8(0x55), "value actually dispatched to the device")
}

// TestDebugIOOutHookSilentWhenNil confirms OUT still reaches the device
// with the hook unset.
func TestDebugIOOutHookSilentWhenNil(t *testing.T) {
	cpu, mem, io := testCPU()
	loadProgram(cpu, mem, 0x0000, 0x3E, 0x01, 0xD3, 0x10)
	DebugIOOutHook = nil
	mustStep(t, cpu)
	mustStep(t, cpu)
	assertEq(t, io.lastOut[0x0110], uint8(0x01), "OUT still reaches the device with hook nil")
}
