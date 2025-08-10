package z80

import (
	"os"
	"path/filepath"
	"testing"
)

// -------- Minimal Spectrum 128 memory + IO for ROM paging (test-only) --------

type mem128 struct {
	rom    [2][]byte // 16K each
	ram    [0x10000]byte
	romSel int // 0 or 1 (selected 16K ROM at 0000-3FFF)
}

func newMem128() *mem128 { return &mem128{} }

func (m *mem128) Read(a uint16) uint8 {
	if a < 0x4000 {
		if m.rom[m.romSel] != nil && int(a) < len(m.rom[m.romSel]) {
			return m.rom[m.romSel][a]
		}
		return 0xFF
	}
	return m.ram[a]
}

func (m *mem128) Write(a uint16, v uint8) {
	// ROM is read-only
	if a >= 0x4000 {
		m.ram[a] = v
	}
}

type io128 struct {
	mem        *mem128
	writes7ffd int
	lock       bool
}

func newIO128(mem *mem128) *io128 { return &io128{mem: mem} }

func (io *io128) In(port uint16) uint8 { return 0xFF }

func (io *io128) Out(port uint16, v uint8) {
	if port == 0x7FFD && !io.lock {
		io.mem.romSel = int((v >> 4) & 1) // bit 4 selects ROM
		if (v & 0x20) != 0 {
			io.lock = true // paging lock
		}
		io.writes7ffd++
	}
}

// -------- helpers --------

func loadFileMust(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

// -------- The test --------

// env: Z80_ROM_STEPS (default ~10M), Z80_ROM_DIR (default ../rom)
func TestOpcodeCoverage_WithSpectrumROM(t *testing.T) {
	steps := getenvInt("Z80_ROM_STEPS", 10_000_000)

	romdir := os.Getenv("Z80_ROM_DIR")
	if romdir == "" {
		romdir = filepath.Join("..", "rom")
	}
	rom0 := loadFileMust(t, filepath.Join(romdir, "128-0.rom"))
	rom1 := loadFileMust(t, filepath.Join(romdir, "128-1.rom"))

	mem := newMem128()
	mem.rom[0] = rom0
	mem.rom[1] = rom1
	io := newIO128(mem)

	cpu := New(mem, io)
	cpu.Reset()

	// Coverage buckets via M1 hook (requires DEBUG_M1=true in z80.go)
	var sinks = newCovSinks()
	attachM1Coverage(cpu, &sinks.base, &sinks.cb, &sinks.ed, &sinks.dd, &sinks.fd, &sinks.ddcb, &sinks.fdcb)

	// Run
	for i := 0; i < steps; i++ {
		cpu.Step()
	}

	// Report minimal stats (kept terse to avoid redecls – full printer lives in previous runs)
	t.Logf("Base: %d/256 (%.1f%%)", len(sinks.base.m), float64(len(sinks.base.m))/2.56)
	t.Logf("CB: %d/256 (%.1f%%)", len(sinks.cb.m), float64(len(sinks.cb.m))/2.56)
	t.Logf("ED: %d/256 (%.1f%%)", len(sinks.ed.m), float64(len(sinks.ed.m))/2.56)
	t.Logf("DD: %d/256 (%.1f%%)", len(sinks.dd.m), float64(len(sinks.dd.m))/2.56)
	t.Logf("FD: %d/256 (%.1f%%)", len(sinks.fd.m), float64(len(sinks.fd.m))/2.56)
	t.Logf("DDCB: %d/256 (%.1f%%)", len(sinks.ddcb.m), float64(len(sinks.ddcb.m))/2.56)
	t.Logf("FDCB: %d/256 (%.1f%%)", len(sinks.fdcb.m), float64(len(sinks.fdcb.m))/2.56)

	// Quick diag of current ROM head
	head := make([]byte, 32)
	for i := 0; i < 32; i++ {
		head[i] = mem.Read(uint16(i))
	}
	sum := 0
	for _, b := range head {
		sum += int(b)
	}
	t.Logf("Diag: first 32 bytes checksum=%d (ROM%d mapped)", sum, mem.romSel)
	t.Logf("Diag: first 32 bytes hexdump:\n%s", hexdump(head, 16))
}
