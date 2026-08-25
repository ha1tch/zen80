// Package system provides system-level emulation components
package system

import (
	"time"
)

// TimingController manages emulation speed to match target hardware
type TimingController struct {
	targetHz        float64       // Target CPU frequency (e.g., 3.5 MHz)
	cyclesPerFrame  int          // Cycles per video frame
	frameRate       float64      // Target frame rate (e.g., 50 Hz)
	
	// Timing state
	frameCycles     int          // Cycles executed in current frame
	frameStartTime  time.Time    // Real time when frame started
	targetFrameTime time.Duration // Target duration per frame
	
	// Statistics
	actualHz        float64      // Measured CPU frequency
	frameCount      uint64       // Total frames executed
	totalCycles     uint64       // Total cycles executed
	startTime       time.Time    // Emulation start time
	
	// Speed control
	speedMultiplier float64      // 1.0 = normal, 2.0 = double speed, etc.
	unlimited       bool         // Run as fast as possible
}

// NewTimingController creates a timing controller for target system
func NewTimingController(cpuHz float64, frameRate float64) *TimingController {
	cyclesPerFrame := int(cpuHz / frameRate)
	targetFrameTime := time.Duration(float64(time.Second) / frameRate)
	
	return &TimingController{
		targetHz:        cpuHz,
		cyclesPerFrame:  cyclesPerFrame,
		frameRate:       frameRate,
		targetFrameTime: targetFrameTime,
		speedMultiplier: 1.0,
		frameStartTime:  time.Now(),
		startTime:       time.Now(),
	}
}

// NewSpectrumTiming creates a timing controller for ZX Spectrum
func NewSpectrumTiming() *TimingController {
	return NewTimingController(3500000, 50) // 3.5 MHz, 50 Hz PAL
}

// AddCycles adds executed cycles and checks if frame sync is needed
func (tc *TimingController) AddCycles(cycles int) bool {
	tc.frameCycles += cycles
	tc.totalCycles += uint64(cycles)
	
	// Check if we've completed a frame's worth of cycles
	if tc.frameCycles >= tc.cyclesPerFrame {
		tc.frameCycles -= tc.cyclesPerFrame
		return true // Signal frame complete
	}
	return false
}

// SyncFrame performs frame synchronization to maintain target speed
func (tc *TimingController) SyncFrame() {
	tc.frameCount++
	
	if tc.unlimited {
		tc.frameStartTime = time.Now()
		return
	}
	
	// Calculate how long this frame should take
	adjustedFrameTime := time.Duration(float64(tc.targetFrameTime) / tc.speedMultiplier)
	
	// Calculate when next frame should start
	targetTime := tc.frameStartTime.Add(adjustedFrameTime)
	
	// Wait if we're running too fast
	now := time.Now()
	if now.Before(targetTime) {
		time.Sleep(targetTime.Sub(now))
	}
	
	// Update statistics every second
	elapsed := now.Sub(tc.startTime)
	if elapsed > time.Second {
		tc.actualHz = float64(tc.totalCycles) / elapsed.Seconds()
	}
	
	tc.frameStartTime = time.Now()
}

// SetSpeedMultiplier sets emulation speed (1.0 = normal)
func (tc *TimingController) SetSpeedMultiplier(speed float64) {
	tc.speedMultiplier = speed
}

// SetUnlimited enables/disables unlimited speed mode
func (tc *TimingController) SetUnlimited(unlimited bool) {
	tc.unlimited = unlimited
}

// GetStatistics returns current timing statistics
func (tc *TimingController) GetStatistics() TimingStats {
	elapsed := time.Since(tc.startTime).Seconds()
	return TimingStats{
		TargetHz:    tc.targetHz,
		ActualHz:    tc.actualHz,
		FrameCount:  tc.frameCount,
		TotalCycles: tc.totalCycles,
		Uptime:      elapsed,
		FrameRate:   float64(tc.frameCount) / elapsed,
	}
}

// TimingStats holds emulation timing statistics
type TimingStats struct {
	TargetHz    float64
	ActualHz    float64
	FrameCount  uint64
	TotalCycles uint64
	Uptime      float64
	FrameRate   float64
}

// FrameTimer provides scanline-level timing for video emulation
type FrameTimer struct {
	cyclesPerLine   int
	linesPerFrame   int
	currentLine     int
	lineCycles      int
}

// NewSpectrumFrameTimer creates a frame timer for ZX Spectrum
func NewSpectrumFrameTimer() *FrameTimer {
	return &FrameTimer{
		cyclesPerLine: 224,  // 224 T-states per scanline
		linesPerFrame: 312,  // 312 lines (192 visible + 120 border/retrace)
		currentLine:   0,
		lineCycles:    0,
	}
}

// AddCycles updates timing and returns events
func (ft *FrameTimer) AddCycles(cycles int) FrameEvent {
	ft.lineCycles += cycles
	event := FrameEvent{}
	
	// Check for line completion
	for ft.lineCycles >= ft.cyclesPerLine {
		ft.lineCycles -= ft.cyclesPerLine
		ft.currentLine++
		
		// Check for frame completion
		if ft.currentLine >= ft.linesPerFrame {
			ft.currentLine = 0
			event.FrameComplete = true
		}
		
		// Determine scanline type
		if ft.currentLine < 192 {
			event.VisibleLine = true
			event.LineNumber = ft.currentLine
		} else if ft.currentLine == 192 {
			event.VBlankStart = true
		}
	}
	
	return event
}

// GetBeamPosition returns current beam position for effects
func (ft *FrameTimer) GetBeamPosition() (line, column int) {
	column = (ft.lineCycles * 256) / ft.cyclesPerLine // Approximate
	return ft.currentLine, column
}

// FrameEvent describes video timing events
type FrameEvent struct {
	FrameComplete bool
	VBlankStart   bool
	VisibleLine   bool
	LineNumber    int
}