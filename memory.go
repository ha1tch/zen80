// Package memory provides memory implementations for the Z80 emulator.
package memory

// RAM implements a simple 64KB RAM
type RAM struct {
	data [65536]uint8
}

// NewRAM creates a new 64KB RAM instance
func NewRAM() *RAM {
	return &RAM{}
}

// Read reads a byte from the specified address
func (r *RAM) Read(address uint16) uint8 {
	return r.data[address]
}

// Write writes a byte to the specified address
func (r *RAM) Write(address uint16, value uint8) {
	r.data[address] = value
}

// Load loads data into memory starting at the specified address
func (r *RAM) Load(address uint16, data []uint8) {
	copy(r.data[address:], data)
}

// Clear clears all memory to zero
func (r *RAM) Clear() {
	for i := range r.data {
		r.data[i] = 0
	}
}

// ROM implements read-only memory
type ROM struct {
	data []uint8
}

// NewROM creates a new ROM with the specified data
func NewROM(data []uint8) *ROM {
	rom := &ROM{
		data: make([]uint8, len(data)),
	}
	copy(rom.data, data)
	return rom
}

// Read reads a byte from the specified address
func (r *ROM) Read(address uint16) uint8 {
	if int(address) < len(r.data) {
		return r.data[address]
	}
	return 0xFF // Return 0xFF for out-of-bounds reads
}

// Write is a no-op for ROM
func (r *ROM) Write(address uint16, value uint8) {
	// No-op - can't write to ROM
}

// MappedMemory implements a memory mapper with ROM and RAM regions
type MappedMemory struct {
	rom      *ROM
	ram      *RAM
	romSize  uint16
	romMask  uint16 // For mirroring if ROM is smaller than region
}

// NewMappedMemory creates a memory mapper with ROM at the bottom and RAM above
func NewMappedMemory(rom []uint8, romSize uint16) *MappedMemory {
	// Calculate mask for ROM mirroring
	mask := romSize - 1
	if romSize&mask != 0 {
		// Not a power of 2, so no mirroring
		mask = 0xFFFF
	}
	
	return &MappedMemory{
		rom:     NewROM(rom),
		ram:     NewRAM(),
		romSize: romSize,
		romMask: mask,
	}
}

// Read reads a byte from the appropriate memory region
func (m *MappedMemory) Read(address uint16) uint8 {
	if address < m.romSize {
		return m.rom.Read(address & m.romMask)
	}
	return m.ram.Read(address)
}

// Write writes a byte to the appropriate memory region
func (m *MappedMemory) Write(address uint16, value uint8) {
	if address < m.romSize {
		// Writes to ROM region are ignored
		return
	}
	m.ram.Write(address, value)
}

// LoadRAM loads data into RAM at the specified address
func (m *MappedMemory) LoadRAM(address uint16, data []uint8) {
	m.ram.Load(address, data)
}

// ClearRAM clears all RAM
func (m *MappedMemory) ClearRAM() {
	m.ram.Clear()
}