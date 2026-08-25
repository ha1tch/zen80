package z80

// ram64 is a minimal flat 64K memory used by tests (e.g., ZEX).
type ram64 struct{ data [65536]byte }

func (m *ram64) Read(a uint16) uint8     { return m.data[a] }
func (m *ram64) Write(a uint16, v uint8) { m.data[a] = v }

// dummyIO is a trivial I/O device. In ZEX tests we use OUT(0),C as "console".
type dummyIO struct{ out []byte }

func (io *dummyIO) In(port uint16) uint8 {
	// Nothing to read; return 0x00 for port 0 so "status" appears idle.
	if port == 0 {
		return 0x00
	}
	return 0xFF
}

func (io *dummyIO) Out(port uint16, value uint8) {
	if port == 0 {
		io.out = append(io.out, value)
	}
	// ignore all other ports
}
