// scenerun.go - headless render + keyboard harness for the Oakhollow engine.
//
// Throwaway-by-design: rewrite main() per task (render a frame, dump a pixel
// region, drive keys, assert no corruption). Built on github.com/ha1tch/zen80.
// Place as cmd/scenerun/main.go inside a zen80 checkout and `go run ./cmd/scenerun`.
//
// Reusable pieces (keep these): pxset, render, KbIO, rowIndex, sym, frame stepping.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

// ---- adjust these paths to your build outputs ----
const binPath = "/home/claude/oakhollow/build/oakhollow.bin"
const symPath = "/home/claude/oakhollow/build/oakhollow.sym"

// ---- settable keyboard matrix IO (inject key presses) ----
type KbIO struct{ kb [8]uint8 }

func NewKbIO() *KbIO {
	k := &KbIO{}
	for i := range k.kb {
		k.kb[i] = 0x1F // 0x1F = no keys pressed in this row
	}
	return k
}
func (io *KbIO) In(port uint16) uint8 {
	if port&0x01 == 0 { // ULA keyboard read
		r := uint8(0x1F)
		for i := uint8(0); i < 8; i++ {
			if port&(1<<(i+8)) == 0 {
				r &= io.kb[i]
			}
		}
		return r
	}
	return 0xFF
}
func (io *KbIO) Out(port uint16, value uint8) {}
func (io *KbIO) press(row, bit int)           { io.kb[row] &^= (1 << uint(bit)) }
func (io *KbIO) release(row, bit int)         { io.kb[row] |= (1 << uint(bit)) }

// rowIndex: a Belfield row byte ($FB,$FD,$DF,$7F,...) is ~(1<<i); return i.
// Verified controls: Q=$FB/0  A=$FD/0  O=$DF/1  P=$DF/0  M=$7F/2
func rowIndex(rb uint8) int {
	for i := 0; i < 8; i++ {
		if rb&(1<<uint(i)) == 0 {
			return i
		}
	}
	return -1
}

// sym: read a label's address from the .sym at runtime (NEVER hardcode - they
// shift when generated data changes size).
func sym(name string) uint16 {
	b, _ := os.ReadFile(symPath)
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(strings.ReplaceAll(line, "\t", " "))
		if len(f) >= 3 && f[0] == name && strings.ToUpper(f[1]) == "EQU" {
			h := strings.TrimSuffix(f[2], "H")
			v, _ := strconv.ParseUint(h, 16, 32)
			return uint16(v)
		}
	}
	return 0
}

// pxset: ZX display-file pixel test (non-linear screen layout).
func pxset(m *memory.RAM, x, y int) bool {
	a := uint16(0x4000 + ((y & 0xC0) << 5) + ((y & 0x07) << 8) + ((y & 0x38) << 2) + (x >> 3))
	return m.Read(a)&(0x80>>uint(x&7)) != 0
}

// render: whole screen to a PNG at scale SC, GB-green palette.
func render(mem *memory.RAM, path string) {
	const W, H, SC = 256, 192, 3
	img := image.NewRGBA(image.Rect(0, 0, W*SC, H*SC))
	ink := color.RGBA{20, 30, 20, 255}
	paper := color.RGBA{200, 220, 170, 255}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := paper
			if pxset(mem, x, y) {
				c = ink
			}
			for dy := 0; dy < SC; dy++ {
				for dx := 0; dx < SC; dx++ {
					img.Set(x*SC+dx, y*SC+dy, c)
				}
			}
		}
	}
	f, _ := os.Create(path)
	png.Encode(f, img)
	f.Close()
}

func main() {
	z80.DEBUG_M1 = false
	bin, _ := os.ReadFile(binPath)
	mem := memory.NewRAM()
	mem.Load(0x8000, bin)
	io := NewKbIO()
	cpu := z80.New(mem, io)
	cpu.PC = 0x8000

	// step one 50Hz frame: run until HALT, then clear it.
	frames, steps := 0, 0
	step1 := func() {
		for {
			cpu.Step()
			steps++
			if cpu.Halted {
				cpu.Halted = false
				frames++
				return
			}
			if steps > 300000000 {
				return
			}
		}
	}

	// --- example: settle, then render the opening screen ---
	for i := 0; i < 25; i++ {
		step1()
	}
	render(mem, "/home/claude/oakhollow/docs/screenshot_oakhollow.png")
	fmt.Println("rendered screenshot. kn_x =", mem.Read(sym("kn_x")), " map =", mem.Read(sym("cur_map")))

	// --- example: drive the knight east into the next location ---
	// io.press(rowIndex(0xDF), 0)            // hold P (right)
	// for mem.Read(sym("cur_map")) == 0 { step1() }
	// io.release(rowIndex(0xDF), 0)
	// fmt.Println("transitioned to map", mem.Read(sym("cur_map")))
}
