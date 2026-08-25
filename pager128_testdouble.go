//go:build test

package z80

// Test-only memory wrapper that also reconstructs CB / DDCB / FDCB opcodes
// which are NOT fetched on an M1 cycle. It works in concert with cpu.M1Hook:
//   - M1Hook updates the prefix state after each opcode-fetch M1.
//   - Read() observes the *non-M1* memory reads immediately after CB (and DDCB/FDCB)
//     to classify the CB operation byte into the right coverage bucket.
//
// This file is intentionally small and self-contained so it won't affect production builds.

type covSinks struct {
	base map[byte]bool
	cb   map[byte]bool
	ed   map[byte]bool
	dd   map[byte]bool
	fd   map[byte]bool
	ddcb map[byte]bool
	fdcb map[byte]bool
}

func newCovSinks() *covSinks {
	return &covSinks{
		base: map[byte]bool{},
		cb:   map[byte]bool{},
		ed:   map[byte]bool{},
		dd:   map[byte]bool{},
		fd:   map[byte]bool{},
		ddcb: map[byte]bool{},
		fdcb: map[byte]bool{},
	}
}

// --- Sniffing memory ---------------------------------------------------------

type sniffMode uint8

const (
	modeNone sniffMode = iota
	modeCB           // next non-M1 read is CB opcode byte
	modeDDCB         // next two non-M1 reads: disp, then opcode byte
	modeFDCB         // next two non-M1 reads: disp, then opcode byte
)

type sniffMem struct {
	mem [65536]byte
	cv  *covSinks

	// prefix state gathered from M1Hook
	hasED bool
	hasDD bool
	hasFD bool

	// CB tracking carried from M1 to the following non-M1 reads
	mode  sniffMode
	phase uint8 // 0=next read, 1=2nd read (for DDCB/FDCB)
}

// newMem128 returns a 64K flat memory for tests that also sniffs CB/xx opcodes.
func newMem128(cv *covSinks) *sniffMem {
	return &sniffMem{cv: cv}
}

func (m *sniffMem) Read(addr uint16) uint8 {
	b := m.mem[addr]

	// Observe non-M1 reads right after a CB prefix
	switch m.mode {
	case modeCB:
		// This read is the CB opcode byte
		m.cv.cb[b] = true
		m.mode = modeNone
	case modeDDCB:
		if m.phase == 0 {
			// displacement byte
			m.phase = 1
		} else {
			// opcode byte
			m.cv.ddcb[b] = true
			m.mode = modeNone
			m.phase = 0
		}
	case modeFDCB:
		if m.phase == 0 {
			// displacement byte
			m.phase = 1
		} else {
			// opcode byte
			m.cv.fdcb[b] = true
			m.mode = modeNone
			m.phase = 0
		}
	}

	return b
}

func (m *sniffMem) Write(addr uint16, v uint8) {
	m.mem[addr] = v
}

// Helpers for tests
func (m *sniffMem) loadROM(data []byte, base uint16) {
	for i, b := range data {
		m.mem[base+uint16(i)] = b
	}
}

// Called from cpu.M1Hook configured by the test. Updates prefix state
// and fills Base/DD/FD/ED buckets directly (since those opcodes are fetched on M1).
func (m *sniffMem) onM1(op byte) {
	switch op {
	case 0xED:
		m.hasED, m.hasDD, m.hasFD = true, false, false
		return
	case 0xDD:
		m.hasDD, m.hasED, m.hasFD = true, false, false
		return
	case 0xFD:
		m.hasFD, m.hasED, m.hasDD = true, false, false
		return
	case 0xCB:
		// CB opcode byte will be read on a following non-M1 memory read
		if m.hasDD {
			m.mode, m.phase = modeDDCB, 0
			// keep DD "sticky" only for this sequence
			m.hasDD = false
			return
		}
		if m.hasFD {
			m.mode, m.phase = modeFDCB, 0
			m.hasFD = false
			return
		}
		m.mode, m.phase = modeCB, 0
		return
	}

	// Not a prefix byte: classify the opcode for the active prefix if any,
	// then clear the prefix latch. Otherwise, it's a base opcode.
	if m.hasED {
		m.cv.ed[op] = true
		m.hasED = false
		return
	}
	if m.hasDD {
		m.cv.dd[op] = true
		m.hasDD = false
		return
	}
	if m.hasFD {
		m.cv.fd[op] = true
		m.hasFD = false
		return
	}
	m.cv.base[op] = true
}
