package z80

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ANSI colors
const (
	ansiReset = "\x1b[0m"
	ansiCyan  = "\x1b[36m"
	ansiYellow= "\x1b[33m"
	ansiGreen = "\x1b[32m"
)

// --- env helpers ---
func getenvInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil {
			return n
		}
	}
	return def
}

func getenvBool(name string, def bool) bool {
	v := strings.ToLower(os.Getenv(name))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return def
	}
}

// --- tiny comma formatter ---
func commify(n int) string {
	s := strconv.FormatInt(int64(n), 10)
	b := []byte(s)
	out := make([]byte, 0, len(b)+len(b)/3)
	for i := range b {
		if i != 0 && (len(b)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, b[i])
	}
	return string(out)
}

// ---- reflection helpers to access CPU regs without a concrete type ----
type cpuRegs struct{ v reflect.Value } // must hold a struct or pointer to struct

func asRegs(cpu interface{}) cpuRegs { return cpuRegs{v: reflect.Indirect(reflect.ValueOf(cpu))} }

func (r cpuRegs) getU16(field string) (uint16, bool) {
	f := r.v.FieldByName(field)
	if f.IsValid() && f.CanInterface() {
		switch f.Kind() {
		case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uint8:
			return uint16(f.Uint()), true
		}
	}
	return 0, false
}

func (r cpuRegs) setU16(field string, val uint16) bool {
	f := r.v.FieldByName(field)
	if f.IsValid() && f.CanSet() {
		switch f.Kind() {
		case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			f.SetUint(uint64(val))
			return true
		case reflect.Uint8:
			f.SetUint(uint64(byte(val)))
			return true
		}
	}
	return false
}

func (r cpuRegs) getU8(field string) (byte, bool) {
	f := r.v.FieldByName(field)
	if f.IsValid() && f.CanInterface() {
		switch f.Kind() {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			return byte(f.Uint()), true
		}
	}
	return 0, false
}

func (r cpuRegs) setU8(field string, val byte) bool {
	f := r.v.FieldByName(field)
	if f.IsValid() && f.CanSet() {
		switch f.Kind() {
		case reflect.Uint8:
			f.SetUint(uint64(val))
			return true
		case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			f.SetUint(uint64(val))
			return true
		}
	}
	return false
}

// Composite helpers (fallback to 16-bit pairs if 8-bit regs aren't present)
func (r cpuRegs) getC() (byte, bool) {
	if v, ok := r.getU8("C"); ok {
		return v, true
	}
	if bc, ok := r.getU16("BC"); ok {
		return byte(bc & 0x00FF), true
	}
	return 0, false
}
func (r cpuRegs) getE() (byte, bool) {
	if v, ok := r.getU8("E"); ok {
		return v, true
	}
	if de, ok := r.getU16("DE"); ok {
		return byte(de & 0x00FF), true
	}
	return 0, false
}
func (r cpuRegs) getD() (byte, bool) {
	if v, ok := r.getU8("D"); ok {
		return v, true
	}
	if de, ok := r.getU16("DE"); ok {
		return byte(de >> 8), true
	}
	return 0, false
}
func (r cpuRegs) setA(val byte) bool {
	if r.setU8("A", val) {
		return true
	}
	// fallback via AF high byte
	if af, ok := r.getU16("AF"); ok {
		af = (af & 0x00FF) | (uint16(val) << 8)
		return r.setU16("AF", af)
	}
	return false
}
func (r cpuRegs) getPC() (uint16, bool) { return r.getU16("PC") }
func (r cpuRegs) setPC(v uint16) bool   { return r.setU16("PC", v) }
func (r cpuRegs) getSP() (uint16, bool) { return r.getU16("SP") }
func (r cpuRegs) setSP(v uint16) bool   { return r.setU16("SP", v) }

// ---- BDOS host-handled trap at PC==0005 (floooh-style, reflection-based) ----
func handleBDOSTrap(t *testing.T, regs cpuRegs, mem *ram64, con *[]byte) {
	fn, ok := regs.getC()
	if !ok {
		t.Fatalf("BDOS trap: cannot read C register")
	}
	switch fn {
	case 0x02: // console out, E=char
		e, ok := regs.getE()
		if !ok {
			t.Fatalf("BDOS fn2: cannot read E")
		}
		*con = append(*con, e)
	case 0x06: // direct console I/O
		e, ok := regs.getE()
		if !ok {
			t.Fatalf("BDOS fn6: cannot read E")
		}
		if e == 0xFF {
			regs.setA(0x00) // no key ready
		} else if e == 0x00 {
			regs.setA(0x0D) // fake Enter
		} else {
			*con = append(*con, e)
		}
	case 0x09: // print string at DE
		d, ok1 := regs.getD()
		e, ok2 := regs.getE()
		if !ok1 || !ok2 {
			t.Fatalf("BDOS fn9: cannot read DE")
		}
		addr := (uint16(d) << 8) | uint16(e)
		for int(addr) < len(mem.data) {
			b := mem.data[addr]
			if b == '$' {
				break
			}
			*con = append(*con, b)
			addr++
		}
	default:
		// ignore other functions for now
	}
	// emulate RET via pop from memory[SP]
	sp, ok := regs.getSP()
	if !ok {
		t.Fatalf("BDOS trap: cannot read SP")
	}
	lo := uint16(mem.data[sp])
	hi := uint16(mem.data[sp+1])
	ret := (hi << 8) | lo
	regs.setSP(sp + 2)
	regs.setPC(ret)
}

// ---- CP/M ANSI colorizing writer (streaming) ----
type ansiState struct {
	atLineStart bool
	prefixBuf   []byte // for deciding cyan 'Z80' vs yellow
	pendingO    bool   // for OK detection across chunk boundaries
	lastEmitted byte
	lineColor   string // current default line color
}

func newAnsiState() *ansiState {
	return &ansiState{atLineStart: true, prefixBuf: make([]byte, 0, 4)}
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func writeWithColor(out *os.File, color string, data []byte) {
	if len(data) == 0 {
		return
	}
	_, _ = out.Write([]byte(color))
	_, _ = out.Write(data)
}

func ansiWrite(out *os.File, st *ansiState, data []byte) {
	i := 0
	for i < len(data) {
		b := data[i]
		// Handle new line starts
		if st.atLineStart {
			st.prefixBuf = append(st.prefixBuf, b)
			if len(st.prefixBuf) < 3 {
				i++
				continue
			}
			// Decide color once we have 3 bytes or a mismatch earlier
			prefix := st.prefixBuf
			if len(prefix) >= 3 && prefix[0] == 'Z' && prefix[1] == '8' && prefix[2] == '0' {
				st.lineColor = ansiCyan
			} else {
				st.lineColor = ansiYellow
			}
			// emit buffered prefix in chosen color
			_, _ = out.Write([]byte(st.lineColor))
			_, _ = out.Write(prefix)
			st.atLineStart = false
			st.prefixBuf = st.prefixBuf[:0]
			st.lastEmitted = prefix[len(prefix)-1]
			i++
			continue
		}

		// Handle CR/LF
		if b == '\r' || b == '\n' {
			_, _ = out.Write([]byte{b})
			st.atLineStart = true
			st.pendingO = false
			st.lastEmitted = b
			i++
			continue
		}

		// OK detection (token-boundary, preserve line color around it)
		if st.pendingO {
			// we previously saw 'O' but didn't emit yet
			if b == 'K' {
				// look ahead boundary
				next := byte(0)
				if i+1 < len(data) {
					next = data[i+1]
				}
				prev := st.lastEmitted
				if !isLetter(prev) && !isLetter(next) {
					// emit green 'OK'
					_, _ = out.Write([]byte(ansiGreen))
					_, _ = out.Write([]byte{'O', 'K'})
					_, _ = out.Write([]byte(st.lineColor)) // back to line color
					st.lastEmitted = 'K'
					st.pendingO = false
					i++
					continue
				}
				// not a standalone token, flush 'O' then continue with current b
				_, _ = out.Write([]byte(st.lineColor))
				_, _ = out.Write([]byte{'O'})
				st.lastEmitted = 'O'
				st.pendingO = false
				// fallthrough to handle current b normally
			} else {
				// flush 'O'
				_, _ = out.Write([]byte(st.lineColor))
				_, _ = out.Write([]byte{'O'})
				st.lastEmitted = 'O'
				st.pendingO = false
				// continue to handle b normally
			}
		}

		if b == 'O' {
			// hold to see if next is 'K'
			st.pendingO = true
			// do not advance lastEmitted yet
			i++
			continue
		}

		// normal streaming char in current line color
		_, _ = out.Write([]byte(st.lineColor))
		_, _ = out.Write([]byte{b})
		st.lastEmitted = b
		i++
	}
}

// ---- pretty-print last batch of lines ----
func lastLines(delta []byte, tailBytes, maxLines int) string {
	if len(delta) == 0 {
		return ""
	}
	if tailBytes > 0 && len(delta) > tailBytes {
		delta = delta[len(delta)-tailBytes:]
	}
	// normalize newlines: CRLF/CR -> LF, and map control chars (except tab/newline) to '.'
	norm := bytes.ReplaceAll(delta, []byte("\r\n"), []byte("\n"))
	norm = bytes.ReplaceAll(norm, []byte("\r"), []byte("\n"))
	buf := make([]byte, 0, len(norm))
	for i := 0; i < len(norm); i++ {
		b := norm[i]
		if b == '\n' || b == '\t' || (b >= 32 && b != 127) {
			buf = append(buf, b)
		} else {
			buf = append(buf, '.')
		}
	}
	s := string(buf)
	parts := strings.Split(s, "\n")
	if maxLines <= 0 || maxLines >= len(parts) {
		return s
	}
	return strings.Join(parts[len(parts)-maxLines:], "\n")
}

func TestZEX_CPMSim_BDOS_PCTrap(t *testing.T) {
	comPath := "../rom/zexdoc.com"
	com, err := os.ReadFile(comPath)
	if err != nil {
		t.Skipf("missing zexdoc.com in ./rom")
	}

	mem := &ram64{}
	io := &dummyIO{} // IO not used with PC-trap, but CPU needs one
	cpu := New(mem, io)
	regs := asRegs(cpu)

	// CP/M layout
	copy(mem.data[0x0100:], com)
	if !regs.setPC(0x0100) {
		t.Fatalf("cannot set PC")
	}
	if !regs.setSP(0xF000) {
		t.Fatalf("cannot set SP")
	}

	// steps: 0 means 'run forever' (until warm boot or silent bail)
	maxStepsEnv := getenvInt("Z80_ZEX_STEPS", 2_000_000_000)
	maxSteps := maxStepsEnv
	if maxStepsEnv == 0 {
		maxSteps = int(^uint(0) >> 1) // largest int for this arch
	}

	progressEvery := getenvInt("Z80_ZEX_PROGRESS_EVERY", 10_000_000) // default 10M ops
	progressMuted := getenvBool("Z80_ZEX_PROGRESS_MUTE", false)      // if true, silence heartbeat logs
	deltaMuted := getenvBool("Z80_ZEX_DELTA_MUTE", false)            // if true, hide 'delta output:' labeling and print raw CP/M text (with ANSI color)
	silentLimit := getenvInt("Z80_ZEX_SILENT_LIMIT", 50_000_000)     // ops with no new output before bailing
	tailLines := getenvInt("Z80_ZEX_TAIL_LINES", 8)
	tailBytes := getenvInt("Z80_ZEX_TAIL_MAX_BYTES", 2048)

	var con []byte
	lastOut := 0
	lastLogged := 0
	silentSince := 0
	bdosCalls := 0
	fnCounts := make(map[byte]int)

	// ANSI streaming state (only used in deltaMuted mode)
	ansi := newAnsiState()

	start := time.Now()
	for i := 1; i <= maxSteps; i++ {
		cpu.Step()

		pc, ok := regs.getPC()
		if !ok {
			t.Fatalf("cannot read PC")
		}

		if pc == 0x0005 {
			if fn, ok := regs.getC(); ok {
				fnCounts[fn]++
			}
			bdosCalls++
			handleBDOSTrap(t, regs, mem, &con)
			if len(con) != lastOut {
				delta := con[lastLogged:]
				if deltaMuted {
					ansiWrite(os.Stdout, ansi, delta) // raw stream with colors, preserve CR semantics
				} else {
					pretty := lastLines(delta, tailBytes, tailLines)
					if strings.TrimSpace(pretty) != "" {
						t.Logf("delta output:\n%s", pretty)
					}
				}
				lastOut = len(con)
				lastLogged = len(con)
				silentSince = 0
			}
			continue
		}

		if pc == 0x0000 {
			if deltaMuted {
				_, _ = os.Stdout.Write([]byte(ansiReset))
			}
			t.Log("Warm boot reached (PC=0000), stopping.")
			break
		}

		if !progressMuted && progressEvery > 0 && i%progressEvery == 0 {
			t.Logf("Ops processed: %s (PC=%04X, out=%d, BDOS=%d)", commify(i), pc, len(con), bdosCalls)
		}

		silentSince++
		if silentSince >= silentLimit {
			startWin := int(pc) - 8
			if startWin < 0 {
				startWin = 0
			}
			endWin := startWin + 32
			if endWin > len(mem.data) {
				endWin = len(mem.data)
			}
			if deltaMuted {
				_, _ = os.Stdout.Write([]byte(ansiReset))
			}
			t.Logf("No output for %s ops; hot PC=%04X; win=[%d:%d] bytes=% X", commify(silentSince), pc, startWin, endWin, mem.data[startWin:endWin])
			for fn, c := range fnCounts {
				t.Logf("BDOS fn %d calls: %d", fn, c)
			}
			pretty := lastLines(con, tailBytes, tailLines)
			if strings.TrimSpace(pretty) != "" {
				t.Logf("Last output lines:\n%s", pretty)
			}
			t.Fatalf("Bailing after prolonged silent loop")
		}
	}
	elapsed := time.Since(start)

	if deltaMuted {
		_, _ = os.Stdout.Write([]byte(ansiReset)) // always reset before exiting the test
	}

	pretty := lastLines(con, tailBytes, tailLines)
	if strings.TrimSpace(pretty) != "" {
		t.Logf("Final output tail:\n%s", pretty)
	} else {
		t.Log("Output: <none>")
	}
	t.Logf("Total captured: %d bytes in %s", len(con), elapsed)

	if path, ok := os.LookupEnv("Z80_ZEX_OUTPUT"); ok && path != "" {
		if dir := filepath.Dir(path); dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(path, con, 0o644); err != nil {
			t.Fatalf("failed to write ZEX log: %v", err)
		}
		t.Logf("Wrote ZEX log to %s (%d bytes).", path, len(con))
	}
}
