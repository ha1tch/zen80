# Z80N: documentation vs. ZEsarUX cross-check

Findings from cross-checking this project's Z80N implementation (see
`z80n.go`) against a second, independent, real-world source once actual
uncertainty was flagged in the code -- not a routine audit of every
opcode, only the ones where the SpecNext wiki's own documentation left
genuine doubt.

**Source checked:** `github.com/chernandezba/zesarux`, the ED-prefix
opcode table in `src/cpus/z80_codpred.c`. GPLv3, mature (initial commit
2022-03-04), TBBlue/Next-aware, actively maintained. Function names are
decimal-indexed by opcode second byte (`instruccion_ed_39` = `ED $27`).

**Primary documentation source:** the SpecNext wiki's [Extended Z80
instruction set](https://wiki.specnext.dev/Extended_Z80_instruction_set)
page, including its dated "A note on some Z80-N specific observations"
section, which corrects its own main instruction table on two points
(2021-09-16 and 2025-01-25 entries).

**Resolution policy** (as directed): the newest documented behaviour
takes priority; older claims are considered superseded unless
independently corroborated by a source at least as current. Where two
current, independently-arrived-at sources disagree, both are recorded
here rather than one being silently discarded.

## Confirmed: implementation already matched ZEsarUX

### TEST $im8 (ED 27)
Implemented as: `C:=0, N:=0, H:=1, PV:=parity(A&n), Z/S standard`
(equivalent to a real `AND A,n`, without writing the result to `A`).

ZEsarUX's own comment: *"same as AND N but without affecting A"*
(`mismo que AND N pero sin afectar A`). Its code does a single
overwriting assignment, `Z80_FLAGS = FLAG_H | sz53p_table[temp_a]`,
which -- because it replaces the whole flags byte rather than OR-ing
into it -- implicitly zeroes C and N. Exact match.

This also resolves an internal inconsistency in the wiki's own page:
its detailed instruction table lists TEST's C and H columns as `S`
("standard"), which doesn't match a real AND's well-established
`C=0,H=1` pattern and contradicts the page's own summary line ("Change
flags as AND A"). ZEsarUX confirms the summary line was right and the
detailed table entry was the error.

### OUTINB (ED 90)
Implemented by reusing this codebase's own already-verified `outi()`
flag computation, with the `B--` step removed (per "OUTINB = OUTI but B
is not decremented"). The wiki marks every flag `?` (unconfirmed) for
this specific opcode.

ZEsarUX implements real, specific flags for OUTINB -- and they match,
flag for flag: H/C via an 8-bit-truncation overflow check (equivalent
to this codebase's full-width threshold check, just a different
idiom), Z/S/X/Y from `B`'s *current* (never-decremented) value via the
same `sz53` lookup pattern `outi()` already uses, PV from
`(sum&7)^B` parity, N from the copied byte's bit 7. Both implementations
also read the low byte (`L`) *after* HL's own increment, not before --
an easy detail to get backwards, and both agree on it.

### LDPIRX (ED B7)
Implemented as: flags unaffected. The wiki's 2025-01-25 correction note
names `LDIX`/`LDDX`/`LDIRX`/`LDDRX` explicitly as now affecting flags,
and conspicuously omits `LDPIRX` -- read at the time as meaning the
correction doesn't extend to it, not as an oversight.

ZEsarUX's own comment states outright: *"LDPIRX does not affect
flags."* Direct confirmation, not just an absence of contrary evidence.

### BSLA DE,B (ED 28) -- boundary behaviour, not a flag question
Not a prior doubt, but worth recording: this codebase's implementation
(`z.SetDE(z.DE() << amt)`, plain Go) and ZEsarUX's C implementation
handle amounts >= 16 differently *in source form* while agreeing in
*result*. ZEsarUX needs an explicit guard --
`if (shift_amount >= 16) DE = 0;` -- because C's behaviour for a shift
count at or beyond the operand's own width is undefined by the C
standard; different compilers could legally do different things.
Go's language spec defines this case exactly (result is 0), so the
single-line Go form gets the same answer for free, with no special
case needed. Confirmed empirically for the boundary case `B=31` in
`z80n_adversarial_test.go`.

## Genuine, recorded disagreement -- implementation unchanged

### ADD HL,A / ADD DE,A / ADD BC,A (ED 31/32/33) -- carry flag
**Wiki:** two dated findings. 2021-09-16 established these do NOT
preserve carry the way classic `ADD HL,rr` does (main table marks it
`?`). 2025-01-25, "Testing 3.02.x" (an explicit, versioned, real-
hardware-tested claim): refined to "most probably always reset."
Implemented here as `C:=0` on that later finding.

**ZEsarUX:** `HL += reg_a;` -- no flags touched at all, comment reads
*"(no flags set)"*. This is a genuine, direct contradiction of the
wiki's 2025 finding, not silence on the question.

**Resolution:** kept the wiki's 2025-01-25 behaviour (`C:=0`).
Git-blamed the exact ZEsarUX line: unchanged since the file's very
first commit, 2022-03-04 -- close to three years before the wiki's
dated hardware test. Read as ZEsarUX simply predating the correction
rather than a competing current claim. Recorded here because it *is* a
real disagreement between two real implementations, not because the
resolution was in doubt.

## Genuine correction made as a result of this cross-check

### LDIX / LDDX -- undocumented X (bit 3) / Y (bit 5) flags
**Original implementation:** reused this codebase's own `ldi()`
formula wholesale -- `n := val + z.A`, then `X := n&8`, `Y := (n&2)<<4`
-- on the assumption that the wiki's "affects flags similarly to LDI"
note meant *identically to LDI, formula included*.

**The gap:** the wiki's 2025-01-25 note only asserts LDIX/LDDX/LDIRX/
LDDRX now affect flags "similarly to LDI, LDD, LDIR and LDDR" -- it
does not commit to the exact X/Y derivation. The `val+A` formula was
this project's own extrapolation into that gap, not something either
documentation source actually claimed.

**ZEsarUX's actual implementation** (`instruccion_ed_164` for LDIX,
`instruccion_ed_172` for LDDX, both unchanged since the same first
commit): derives X/Y from the *raw byte read*, with no `+A` at all --
`if (byte_leido & 8) FLAG_3;` / `if (byte_leido & 2) FLAG_5;`. Same
pattern, consistently, in both instructions.

**Resolution:** corrected both `z80nLdix()` and `z80nLddx()` to match
ZEsarUX exactly (`val&0x08` -> X, `val&0x02` -> Y, no `+A`). This
wasn't a source-preference call the way the ADD-carry case was --
there was no competing dated claim to weigh against ZEsarUX, just this
project's own unfounded assumption, which ZEsarUX's real,
independently-arrived-at implementation corrected. `LDIRX`/`LDDRX`
inherit the fix automatically, since both call `z80nLdix()`/`z80nLddx()`
directly rather than duplicating their logic.

New test coverage added in `z80n_semantics_test.go`
(`TestZ80N_Ldix/flags_from_ZEsarUX_cross_check`) isolates X and Y
independently against two byte values chosen so neither flag's
correctness could hide behind the other (`0x0B`: both source bits set;
`0x04`: neither), plus a PV check on both sides of the BC==0 boundary.

## Not yet cross-checked

Everything else in `z80n.go` (`SWAPNIB`, `MIRROR A`, the remaining
barrel-shift/rotate opcodes, `MUL D,E`, the `ADD rr,$im16` forms,
`PIXELDN`, `PIXELAD`, `SETAE`) was implemented directly from the wiki
with no documented internal contradiction or unresolved gap at the
time, so no ZEsarUX cross-check was done for these specifically. Worth
revisiting the same way if a similar doubt surfaces for any of them.

Step 3's remaining opcodes (`PUSH $im16`, both `NEXTREG` forms,
`JP (C)`) are now implemented, also directly from the wiki with no
documented internal contradiction: `PUSH $im16`'s formula (SP-=2;
SP*:=nn, with the operand itself uniquely big-endian in the instruction
stream) and `NEXTREG`'s two-port-write formula are both stated plainly
with no dated hardware-testing note to reconcile against anything else.
`JP (C)`'s formula (`PC:=PC&$C000+IN(C)<<6`) is likewise stated plainly,
though its flag effects are marked "?" across the board with no
alternative source offering anything more specific -- left unchanged
rather than guessed, the same conservative choice made elsewhere in
this file for genuinely undocumented cases. No ZEsarUX cross-check was
done for any of the four, since none had the kind of two-source
disagreement that prompted one for `ADD HL/DE/BC,A`.

## A further, independent check available later

A real FPGA core's Verilog/VHDL source (if obtained) would be a
stronger source than either of the above for genuinely disputed points
like the `ADD HL/DE/BC,A` carry question -- both the wiki and ZEsarUX
are *someone's account* of hardware behaviour, however careful; the
core's own source is the specification made real. Flagged as a
reasonable next step if the ADD-carry disagreement, or anything else
found later, needs settling beyond "which account is newer."
