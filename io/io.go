// Package io provides I/O port implementations for the Z80 emulator.
package io

// NullIO implements a null I/O interface that returns 0xFF for all reads
type NullIO struct{}

// NewNullIO creates a new null I/O instance
func NewNullIO() *NullIO {
	return &NullIO{}
}

// In reads from an I/O port (always returns 0xFF)
func (n *NullIO) In(port uint16) uint8 {
	return 0xFF
}

// Out writes to an I/O port (no-op)
func (n *NullIO) Out(port uint16, value uint8) {
	// No-op
}

// MappedIO implements I/O with callbacks for specific ports
type MappedIO struct {
	readHandlers  map[uint16]func(port uint16) uint8
	writeHandlers map[uint16]func(port uint16, value uint8)
	defaultValue  uint8
}

// NewMappedIO creates a new mapped I/O instance
func NewMappedIO() *MappedIO {
	return &MappedIO{
		readHandlers:  make(map[uint16]func(port uint16) uint8),
		writeHandlers: make(map[uint16]func(port uint16, value uint8)),
		defaultValue:  0xFF,
	}
}

// RegisterReadHandler registers a read handler for a specific port
func (m *MappedIO) RegisterReadHandler(port uint16, handler func(port uint16) uint8) {
	m.readHandlers[port] = handler
}

// RegisterWriteHandler registers a write handler for a specific port
func (m *MappedIO) RegisterWriteHandler(port uint16, handler func(port uint16, value uint8)) {
	m.writeHandlers[port] = handler
}

// RegisterReadRange registers a read handler for a range of ports
func (m *MappedIO) RegisterReadRange(start, end uint16, handler func(port uint16) uint8) {
	for port := start; port <= end; port++ {
		m.readHandlers[port] = handler
	}
}

// RegisterWriteRange registers a write handler for a range of ports
func (m *MappedIO) RegisterWriteRange(start, end uint16, handler func(port uint16, value uint8)) {
	for port := start; port <= end; port++ {
		m.writeHandlers[port] = handler
	}
}

// In reads from an I/O port
func (m *MappedIO) In(port uint16) uint8 {
	if handler, exists := m.readHandlers[port]; exists {
		return handler(port)
	}
	// Also check for 8-bit port address (some systems only decode lower 8 bits)
	if handler, exists := m.readHandlers[port&0xFF]; exists {
		return handler(port)
	}
	return m.defaultValue
}

// Out writes to an I/O port
func (m *MappedIO) Out(port uint16, value uint8) {
	if handler, exists := m.writeHandlers[port]; exists {
		handler(port, value)
		return
	}
	// Also check for 8-bit port address
	if handler, exists := m.writeHandlers[port&0xFF]; exists {
		handler(port, value)
	}
}

// SetDefaultValue sets the default value returned for unmapped port reads
func (m *MappedIO) SetDefaultValue(value uint8) {
	m.defaultValue = value
}

// SimpleIO implements basic I/O with a simple port array
type SimpleIO struct {
	ports [256]uint8
}

// NewSimpleIO creates a new simple I/O instance
func NewSimpleIO() *SimpleIO {
	io := &SimpleIO{}
	// Initialize all ports to 0xFF
	for i := range io.ports {
		io.ports[i] = 0xFF
	}
	return io
}

// In reads from an I/O port (only uses lower 8 bits of address)
func (s *SimpleIO) In(port uint16) uint8 {
	return s.ports[port&0xFF]
}

// Out writes to an I/O port (only uses lower 8 bits of address)
func (s *SimpleIO) Out(port uint16, value uint8) {
	s.ports[port&0xFF] = value
}

// GetPort returns the current value of a port (for debugging)
func (s *SimpleIO) GetPort(port uint8) uint8 {
	return s.ports[port]
}

// SetPort sets the value of a port (for debugging/testing)
func (s *SimpleIO) SetPort(port uint8, value uint8) {
	s.ports[port] = value
}