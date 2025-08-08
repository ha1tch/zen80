package system

import (
	"fmt"
	"time"

	"github.com/ha1tch/zen80/io"
	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

// Spectrum represents a ZX Spectrum system
type Spectrum struct {
	cpu         *z80.Z80
	memory      *SpectrumMemory
	io          *SpectrumIO
	timing      *TimingController
	frameTimer  *FrameTimer
	
	// Video state
	screen      [192][256]uint8  // Screen pixels (attribute-less for simplicity)
	border      uint8            // Border color
	
	// System state
	running     bool
	paused      bool
}

// SpectrumMemory implements ZX Spectrum memory layout
type SpectrumMemory struct {
	rom  [16384]uint8  // 16K ROM
	ram  [49152]uint8  // 48K RAM
}

func NewSpectrumMemory() *SpectrumMemory {
	return &SpectrumMemory{}
}

func (m *SpectrumMemory) Read(address uint16) uint8 {
	if address < 0x4000 {
		return m.rom[address]
	}
	return m.ram[address-0x4000]
}

func (m *SpectrumMemory) Write(address uint16, value uint8) {
	if address >= 0x4000 {
		m.ram[address-0x4000] = value
	}
	// Writes to ROM are ignored
}

func (m *SpectrumMemory) LoadROM(data []uint8) {
	copy(m.rom[:], data)
}

// SpectrumIO implements ZX Spectrum I/O ports
type SpectrumIO struct {
	border      *uint8
	keyboard    [8]uint8  // Keyboard matrix
	tapeIn      bool
	speaker     bool
}

func NewSpectrumIO(border *uint8) *SpectrumIO {
	io := &SpectrumIO{
		border: border,
	}
	// Initialize keyboard matrix (no keys pressed)
	for i := range io.keyboard {
		io.keyboard[i] = 0x1F
	}
	return io
}

func (io *SpectrumIO) In(port uint16) uint8 {
	// ULA port (keyboard and tape)
	if port&0x01 == 0 {
		// Keyboard read
		result := uint8(0x1F) // No keys pressed
		for i := uint8(0); i < 8; i++ {
			if port&(1<<(i+8)) == 0 {
				result &= io.keyboard[i]
			}
		}
		// Bit 6 = tape input
		if io.tapeIn {
			result |= 0x40
		}
		return result
	}
	
	// Kempston joystick
	if port&0xFF == 0x1F {
		return 0x00 // No joystick input
	}
	
	return 0xFF
}

func (io *SpectrumIO) Out(port uint16, value uint8) {
	// ULA port (border and speaker)
	if port&0x01 == 0 {
		*io.border = value & 0x07     // Border color (bits 0-2)
		io.speaker = (value & 0x10) != 0  // Speaker (bit 4)
	}
}

// NewSpectrum creates a new ZX Spectrum emulator
func NewSpectrum() *Spectrum {
	spec := &Spectrum{
		memory:     NewSpectrumMemory(),
		timing:     NewSpectrumTiming(),
		frameTimer: NewSpectrumFrameTimer(),
	}
	
	spec.io = NewSpectrumIO(&spec.border)
	spec.cpu = z80.New(spec.memory, spec.io)
	
	return spec
}

// LoadROM loads the Spectrum ROM
func (s *Spectrum) LoadROM(data []uint8) error {
	if len(data) != 16384 {
		return fmt.Errorf("ROM must be exactly 16384 bytes, got %d", len(data))
	}
	s.memory.LoadROM(data)
	return nil
}

// LoadSnapshot loads a program into memory
func (s *Spectrum) LoadSnapshot(address uint16, data []uint8) {
	for i, b := range data {
		s.memory.Write(address+uint16(i), b)
	}
}

// Reset resets the system
func (s *Spectrum) Reset() {
	s.cpu.Reset()
	s.border = 0
	s.frameTimer = NewSpectrumFrameTimer()
}

// RunFrame executes one frame worth of CPU cycles
func (s *Spectrum) RunFrame() {
	frameDone := false
	
	for !frameDone && s.running {
		// Execute one instruction
		cycles := s.cpu.Step()
		
		// Update frame timing
		frameEvent := s.frameTimer.AddCycles(cycles)
		
		// Handle video events
		if frameEvent.VisibleLine {
			// In a real emulator, we'd render the scanline here
			// For now, just track that we're in visible area
		}
		
		if frameEvent.VBlankStart {
			// Generate interrupt at start of vertical blank
			s.cpu.INT = true
		}
		
		if frameEvent.FrameComplete {
			s.cpu.INT = false // Clear interrupt
			frameDone = true
		}
		
		// Update master timing
		if s.timing.AddCycles(cycles) {
			// Frame boundary according to cycle count
			// (should align with frameEvent.FrameComplete)
		}
	}
	
	// Synchronize to real time
	s.timing.SyncFrame()
}

// Run starts the emulation
func (s *Spectrum) Run() {
	s.running = true
	
	for s.running {
		if !s.paused {
			s.RunFrame()
		} else {
			time.Sleep(time.Millisecond * 16) // ~60 FPS when paused
		}
	}
}

// Stop stops the emulation
func (s *Spectrum) Stop() {
	s.running = false
}

// Pause pauses/unpauses the emulation
func (s *Spectrum) Pause(pause bool) {
	s.paused = pause
}

// GetStatistics returns timing statistics
func (s *Spectrum) GetStatistics() TimingStats {
	return s.timing.GetStatistics()
}

// SetSpeed sets emulation speed multiplier
func (s *Spectrum) SetSpeed(multiplier float64) {
	s.timing.SetSpeedMultiplier(multiplier)
}

// PressKey simulates a key press
func (s *Spectrum) PressKey(row, col uint8) {
	if row < 8 && col < 5 {
		s.io.keyboard[row] &^= (1 << col)
	}
}

// ReleaseKey simulates a key release
func (s *Spectrum) ReleaseKey(row, col uint8) {
	if row < 8 && col < 5 {
		s.io.keyboard[row] |= (1 << col)
	}
}