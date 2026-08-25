package z80

import (
	"fmt"
	"strings"
	"time"
)

// hexdump prints a compact hex + ascii line dump suitable for test logs.
func hexdump(p []byte, width int) string {
	if width <= 0 {
		width = 16
	}
	var b strings.Builder
	for i := 0; i < len(p); i += width {
		end := i + width
		if end > len(p) {
			end = len(p)
		}
		b.WriteString(fmt.Sprintf("%08X  ", i))
		for j := i; j < end; j++ {
			b.WriteString(fmt.Sprintf("%02x ", p[j]))
		}
		for j := end; j < i+width; j++ {
			b.WriteString("   ")
		}
		b.WriteString(" |")
		for j := i; j < end; j++ {
			c := p[j]
			if c < 32 || c > 126 {
				c = '.'
			}
			b.WriteByte(c)
		}
		b.WriteString("|\n")
	}
	return b.String()
}

// Coverage buckets for opcode prefixes.
type bucket struct{ m map[byte]bool }

func newBucket() bucket { return bucket{m: map[byte]bool{}} }

// covSinks groups all coverage buckets for convenience.
type covSinks struct {
	base, cb, ed, dd, fd, ddcb, fdcb bucket
}

func newCovSinks() covSinks {
	return covSinks{
		base: newBucket(), cb: newBucket(), ed: newBucket(),
		dd: newBucket(), fd: newBucket(), ddcb: newBucket(), fdcb: newBucket(),
	}
}

// prefix reconstruction state for M1 tracing.
type prefState struct {
	hasDD, hasFD, hasCB, hasED   bool
	ddDispPending, fdDispPending bool
	afterCB                      bool
}

// attachM1Coverage wires an M1 hook that fills the provided buckets.
// NOTE: Your z80.go must call z.M1Hook on M1 fetches (it does when DEBUG_M1 is true).
func attachM1Coverage(z *Z80, base, cb, ed, dd, fd, ddcb, fdcb *bucket) {
	st := &prefState{}
	z.M1Hook = func(pc uint16, op byte, context string) {
		// Reconstruct full opcode groups across M1s.
		switch {
		case st.afterCB:
			// after CB/DDCB/FDCB opcode fetch
			if st.ddDispPending {
				ddcb.m[op] = true
				st.ddDispPending = false
				st.hasDD = false
			} else if st.fdDispPending {
				fdcb.m[op] = true
				st.fdDispPending = false
				st.hasFD = false
			} else {
				cb.m[op] = true
			}
			st.afterCB = false
			st.hasCB = false
		default:
			switch op {
			case 0xDD:
				st.hasDD, st.hasFD, st.hasCB, st.hasED = true, false, false, false
			case 0xFD:
				st.hasFD, st.hasDD, st.hasCB, st.hasED = true, false, false, false
			case 0xED:
				st.hasED, st.hasCB = true, false
			case 0xCB:
				// Could be plain CB or DDCB/FDCB. Displacement byte happens between M1s, so we mark a pending state.
				if st.hasDD {
					st.ddDispPending = true
				} else if st.hasFD {
					st.fdDispPending = true
				}
				st.afterCB = true
			default:
				if st.hasED {
					ed.m[op] = true
					st.hasED = false
				} else if st.ddDispPending {
					// The displacement itself is not an M1 fetch; next M1 is the real opcode (handled above).
					// Leave ddDispPending set until afterCB path.
				} else if st.fdDispPending {
					// Same for FD-prefixed CB group.
				} else if st.hasDD {
					dd.m[op] = true
					st.hasDD = false
				} else if st.hasFD {
					fd.m[op] = true
					st.hasFD = false
				} else {
					base.m[op] = true
				}
			}
		}
	}
}

// small helper to timestamp messages consistently
func ts() string { return time.Now().Format("15:04:05.000") }
