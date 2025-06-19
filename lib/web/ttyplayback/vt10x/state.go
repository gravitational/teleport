package vt10x

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
)

type Segment struct {
	Text       string  `json:"text"`
	Pen        Pen     `json:"pen"`
	Offset     int     `json:"offset"`
	CellCount  int     `json:"cellCount"`
	CharWidth  int     `json:"charWidth"`
	ExtraClass *string `json:"extraClass,omitempty"`
}

type RGB8 struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

type PenColor struct {
	Type  string
	Value uint8
	RGB   *RGB8
}

type Pen struct {
	Foreground  *PenColor `json:"foreground,omitempty"`
	Background  *PenColor `json:"background,omitempty"`
	IsBold      bool      `json:"is_bold"`
	IsItalic    bool      `json:"is_italic"`
	IsUnderline bool      `json:"is_underline"`
	IsBlink     bool      `json:"is_blink"`
	IsInverse   bool      `json:"is_inverse"`
}

func penFromGlyph(g Glyph) Pen {
	return Pen{
		Background:  colorFromVT10X(g.BG),
		Foreground:  colorFromVT10X(g.FG),
		IsBold:      g.Mode&attrBold != 0,
		IsItalic:    g.Mode&attrItalic != 0,
		IsUnderline: g.Mode&attrUnderline != 0,
		IsBlink:     g.Mode&attrBlink != 0,
		IsInverse:   g.Mode&attrReverse != 0,
	}
}

func printColor(c *PenColor) string {
	if c == nil {
		return "default"
	}
	if c.Type == "Indexed" {
		return fmt.Sprintf("indexed(%d)", c.Value)
	}
	if c.RGB != nil {
		return fmt.Sprintf("rgb(%d, %d, %d)", c.RGB.R, c.RGB.G, c.RGB.B)
	}
	return "unknown"
}

func isSpecialChar(g Glyph) bool {
	return g.Char == 0 || g.Char == '\t' || g.Char == '\n'
}

func glyphWidth(g Glyph) int {
	if g.Char == 0 {
		return 0
	}
	return 1
}

func (t *terminal) Line(n int) []Segment {
	width, _ := t.Size()
	var segments []Segment
	offset := 0

	col := 0
	for col < width {
		startGlyph := t.Cell(col, n)
		startPen := penFromGlyph(startGlyph)

		var cells []Glyph
		cells = append(cells, startGlyph)
		col++

		for col < width {
			g := t.Cell(col, n)
			currentPen := penFromGlyph(g)

			if !pensEqual(startPen, currentPen) || isSpecialChar(startGlyph) || isSpecialChar(g) {
				break
			}

			cells = append(cells, g)
			col++
		}

		if len(cells) == 0 {
			continue
		}

		text := ""
		cellCount := 0
		for _, cell := range cells {
			if cell.Char != 0 {
				text += string(cell.Char)
			}
			cellCount += glyphWidth(cell)
		}

		if cellCount > 0 {
			segment := Segment{
				Text:      text,
				Pen:       startPen,
				Offset:    offset,
				CellCount: cellCount,
				CharWidth: glyphWidth(cells[0]),
			}
			segments = append(segments, segment)
			offset += cellCount
		}
	}

	return segments
}

func pensEqual(p1, p2 Pen) bool {
	return p1.IsBold == p2.IsBold &&
		p1.IsItalic == p2.IsItalic &&
		p1.IsUnderline == p2.IsUnderline &&
		p1.IsBlink == p2.IsBlink &&
		p1.IsInverse == p2.IsInverse &&
		colorsEqual(p1.Foreground, p2.Foreground) &&
		colorsEqual(p1.Background, p2.Background)
}

func colorFromVT10X(vt10xColor Color) *PenColor {
	if vt10xColor >= DefaultFG {
		return nil
	}

	if vt10xColor < 256 {
		return &PenColor{
			Type:  "Indexed",
			Value: uint8(vt10xColor),
		}
	}

	return nil
}

func colorsEqual(c1, c2 *PenColor) bool {
	if c1 == nil && c2 == nil {
		return true
	}
	if c1 == nil || c2 == nil {
		return false
	}
	if c1.Type != c2.Type {
		return false
	}
	if c1.Type == "Indexed" {
		return c1.Value == c2.Value
	}
	return c1.RGB.R == c2.RGB.R && c1.RGB.G == c2.RGB.G && c1.RGB.B == c2.RGB.B
}

//type Color struct {
//	R, G, B uint8
//}

// TerminalState represents the minimal state needed to recreate
// a terminal through ANSI escape sequences
type TerminalState struct {
	// Screen dimensions
	Cols, Rows int

	// Cursor position and visibility
	CursorX, CursorY int
	CursorVisible    bool

	// Both screen buffers
	PrimaryBuffer   [][]Glyph
	AlternateBuffer [][]Glyph

	// Active screen buffer (false = primary, true = alternate)
	AltScreen bool

	// Scroll region
	ScrollTop, ScrollBottom int

	// Tab stops
	TabStops []int

	// Terminal modes that can be set via ANSI
	Wrap         bool
	Insert       bool
	Origin       bool
	AutoWrap     bool
	ReverseVideo bool

	// Window title
	Title string

	// Saved cursor state (for DECSC/DECRC)
	SavedCursorX, SavedCursorY int
}

// Cell represents a single character cell with its attributes
type Cell struct {
	Char      rune
	FG        Color
	BG        Color
	Bold      bool
	Underline bool
	Reverse   bool
	Italic    bool
	Blink     bool
}

const (
	tabspaces = 8
)

const (
	attrReverse = 1 << iota
	attrUnderline
	attrBold
	attrGfx
	attrItalic
	attrBlink
	attrWrap
)

const (
	cursorDefault = 1 << iota
	cursorWrapNext
	cursorOrigin
)

// ModeFlag represents various terminal mode states.
type ModeFlag uint32

// Terminal modes
const (
	ModeWrap ModeFlag = 1 << iota
	ModeInsert
	ModeAppKeypad
	ModeAltScreen
	ModeCRLF
	ModeMouseButton
	ModeMouseMotion
	ModeReverse
	ModeKeyboardLock
	ModeHide
	ModeEcho
	ModeAppCursor
	ModeMouseSgr
	Mode8bit
	ModeBlink
	ModeFBlink
	ModeFocus
	ModeMouseX10
	ModeMouseMany
	ModeMouseMask = ModeMouseButton | ModeMouseMotion | ModeMouseX10 | ModeMouseMany
)

// ChangeFlag represents possible state changes of the terminal.
type ChangeFlag uint32

// Terminal changes to occur in VT.ReadState
const (
	ChangedScreen ChangeFlag = 1 << iota
	ChangedTitle
)

type Glyph struct {
	Char   rune
	Mode   int16
	FG, BG Color
}

type line []Glyph

type Cursor struct {
	Attr  Glyph
	X, Y  int
	State uint8
}

type parseState func(c rune)

// State represents the terminal emulation state. Use Lock/Unlock
// methods to synchronize data access with VT.
type State struct {
	DebugLogger *log.Logger

	w             io.Writer
	mu            sync.Mutex
	changed       ChangeFlag
	cols, rows    int
	lines         []line
	altLines      []line
	dirty         []bool // line dirtiness
	anydirty      bool
	cur, curSaved Cursor
	top, bottom   int // scroll limits
	mode          ModeFlag
	state         parseState
	str           strEscape
	csi           csiEscape
	numlock       bool
	tabs          []bool
	title         string
	colorOverride map[Color]Color
}

func newState(w io.Writer) *State {
	return &State{
		w:             w,
		colorOverride: make(map[Color]Color),
	}
}

func (t *State) logf(format string, args ...interface{}) {
	if t.DebugLogger != nil {
		t.DebugLogger.Printf(format, args...)
	}
}

func (t *State) logln(s string) {
	if t.DebugLogger != nil {
		t.DebugLogger.Println(s)
	}
}

func (t *State) lock() {
	t.mu.Lock()
}

func (t *State) unlock() {
	t.mu.Unlock()
}

// Lock locks the state object's mutex.
func (t *State) Lock() {
	t.mu.Lock()
}

// Unlock resets change flags and unlocks the state object's mutex.
func (t *State) Unlock() {
	t.resetChanges()
	t.mu.Unlock()
}

// Cell returns the glyph containing the character code, foreground color, and
// background color at position (x, y) relative to the top left of the terminal.
func (t *State) Cell(x, y int) Glyph {
	cell := t.lines[y][x]
	fg, ok := t.colorOverride[cell.FG]
	if ok {
		cell.FG = fg
	}
	bg, ok := t.colorOverride[cell.BG]
	if ok {
		cell.BG = bg
	}
	return cell
}

// Cursor returns the current position of the cursor.
func (t *State) Cursor() Cursor {
	return t.cur
}

// CursorVisible returns the visible state of the cursor.
func (t *State) CursorVisible() bool {
	return t.mode&ModeHide == 0
}

// Mode returns the current terminal mode.
func (t *State) Mode() ModeFlag {
	return t.mode
}

// Title returns the current title set via the tty.
func (t *State) Title() string {
	return t.title
}

/*
// ChangeMask returns a bitfield of changes that have occured by VT.
func (t *State) ChangeMask() ChangeFlag {
	return t.changed
}
*/

// Changed returns true if change has occured.
func (t *State) Changed(change ChangeFlag) bool {
	return t.changed&change != 0
}

// resetChanges resets the change mask and dirtiness.
func (t *State) resetChanges() {
	for i := range t.dirty {
		t.dirty[i] = false
	}
	t.anydirty = false
	t.changed = 0
}

func (t *State) saveCursor() {
	t.curSaved = t.cur
}

func (t *State) restoreCursor() {
	t.cur = t.curSaved
	t.moveTo(t.cur.X, t.cur.Y)
}

func (t *State) put(c rune) {
	t.state(c)
}

func (t *State) putTab(forward bool) {
	x := t.cur.X
	if forward {
		if x == t.cols {
			return
		}
		for x++; x < t.cols && !t.tabs[x]; x++ {
		}
	} else {
		if x == 0 {
			return
		}
		for x--; x > 0 && !t.tabs[x]; x-- {
		}
	}
	t.moveTo(x, t.cur.Y)
}

func (t *State) newline(firstCol bool) {
	y := t.cur.Y
	if y == t.bottom {
		cur := t.cur
		t.cur = t.defaultCursor()
		t.scrollUp(t.top, 1)
		t.cur = cur
	} else {
		y++
	}
	if firstCol {
		t.moveTo(0, y)
	} else {
		t.moveTo(t.cur.X, y)
	}
}

// table from st, which in turn is from rxvt :)
var gfxCharTable = [62]rune{
	'↑', '↓', '→', '←', '█', '▚', '☃', // A - G
	0, 0, 0, 0, 0, 0, 0, 0, // H - O
	0, 0, 0, 0, 0, 0, 0, 0, // P - W
	0, 0, 0, 0, 0, 0, 0, ' ', // X - _
	'◆', '▒', '␉', '␌', '␍', '␊', '°', '±', // ` - g
	'␤', '␋', '┘', '┐', '┌', '└', '┼', '⎺', // h - o
	'⎻', '─', '⎼', '⎽', '├', '┤', '┴', '┬', // p - w
	'│', '≤', '≥', 'π', '≠', '£', '·', // x - ~
}

func (t *State) setChar(c rune, attr *Glyph, x, y int) {
	if attr.Mode&attrGfx != 0 {
		if c >= 0x41 && c <= 0x7e && gfxCharTable[c-0x41] != 0 {
			c = gfxCharTable[c-0x41]
		}
	}
	t.changed |= ChangedScreen
	t.dirty[y] = true
	t.lines[y][x] = *attr
	t.lines[y][x].Char = c
	//if t.options.BrightBold && attr.Mode&attrBold != 0 && attr.FG < 8 {
	if attr.Mode&attrBold != 0 && attr.FG < 8 {
		t.lines[y][x].FG = attr.FG + 8
	}
	if attr.Mode&attrReverse != 0 {
		t.lines[y][x].FG = attr.BG
		t.lines[y][x].BG = attr.FG
	}
}

func (t *State) defaultCursor() Cursor {
	c := Cursor{}
	c.Attr.FG = DefaultFG
	c.Attr.BG = DefaultBG
	return c
}

func (t *State) reset() {
	t.cur = t.defaultCursor()
	t.saveCursor()
	for i := range t.tabs {
		t.tabs[i] = false
	}
	for i := tabspaces; i < len(t.tabs); i += tabspaces {
		t.tabs[i] = true
	}
	t.top = 0
	t.bottom = t.rows - 1
	t.mode = ModeWrap
	t.clear(0, 0, t.rows-1, t.cols-1)
	t.moveTo(0, 0)
}

// TODO: definitely can improve allocs
func (t *State) resize(cols, rows int) bool {
	if cols == t.cols && rows == t.rows {
		return false
	}
	if cols < 1 || rows < 1 {
		return false
	}
	slide := t.cur.Y - rows + 1
	if slide > 0 {
		copy(t.lines, t.lines[slide:slide+rows])
		copy(t.altLines, t.altLines[slide:slide+rows])
	}

	lines, altLines, tabs := t.lines, t.altLines, t.tabs
	t.lines = make([]line, rows)
	t.altLines = make([]line, rows)
	t.dirty = make([]bool, rows)
	t.tabs = make([]bool, cols)

	minrows := min(rows, t.rows)
	mincols := min(cols, t.cols)
	t.changed |= ChangedScreen
	for i := 0; i < rows; i++ {
		t.dirty[i] = true
		t.lines[i] = make(line, cols)
		t.altLines[i] = make(line, cols)
	}
	for i := 0; i < minrows; i++ {
		copy(t.lines[i], lines[i])
		copy(t.altLines[i], altLines[i])
	}
	copy(t.tabs, tabs)
	if cols > t.cols {
		i := t.cols - 1
		for i > 0 && !tabs[i] {
			i--
		}
		for i += tabspaces; i < len(tabs); i += tabspaces {
			tabs[i] = true
		}
	}

	t.cols = cols
	t.rows = rows
	t.setScroll(0, rows-1)
	t.moveTo(t.cur.X, t.cur.Y)
	for i := 0; i < 2; i++ {
		if mincols < cols && minrows > 0 {
			t.clear(mincols, 0, cols-1, minrows-1)
		}
		if cols > 0 && minrows < rows {
			t.clear(0, minrows, cols-1, rows-1)
		}
		t.swapScreen()
	}
	return slide > 0
}

func (t *State) clear(x0, y0, x1, y1 int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	x0 = clamp(x0, 0, t.cols-1)
	x1 = clamp(x1, 0, t.cols-1)
	y0 = clamp(y0, 0, t.rows-1)
	y1 = clamp(y1, 0, t.rows-1)
	t.changed |= ChangedScreen
	for y := y0; y <= y1; y++ {
		t.dirty[y] = true
		for x := x0; x <= x1; x++ {
			t.lines[y][x] = t.cur.Attr
			t.lines[y][x].Char = ' '
		}
	}
}

func (t *State) clearAll() {
	t.clear(0, 0, t.cols-1, t.rows-1)
}

func (t *State) moveAbsTo(x, y int) {
	if t.cur.State&cursorOrigin != 0 {
		y += t.top
	}
	t.moveTo(x, y)
}

func (t *State) moveTo(x, y int) {
	var miny, maxy int
	if t.cur.State&cursorOrigin != 0 {
		miny = t.top
		maxy = t.bottom
	} else {
		miny = 0
		maxy = t.rows - 1
	}
	x = clamp(x, 0, t.cols-1)
	y = clamp(y, miny, maxy)
	t.changed |= ChangedScreen
	t.cur.State &^= cursorWrapNext
	t.cur.X = x
	t.cur.Y = y
}

func (t *State) swapScreen() {
	t.lines, t.altLines = t.altLines, t.lines
	t.mode ^= ModeAltScreen
	t.dirtyAll()
}

func (t *State) dirtyAll() {
	t.changed |= ChangedScreen
	for y := 0; y < t.rows; y++ {
		t.dirty[y] = true
	}
}

func (t *State) setScroll(top, bottom int) {
	top = clamp(top, 0, t.rows-1)
	bottom = clamp(bottom, 0, t.rows-1)
	if top > bottom {
		top, bottom = bottom, top
	}
	t.top = top
	t.bottom = bottom
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	} else if val > max {
		return max
	}
	return val
}

func between(val, min, max int) bool {
	if val < min || val > max {
		return false
	}
	return true
}

func (t *State) scrollDown(orig, n int) {
	n = clamp(n, 0, t.bottom-orig+1)
	t.clear(0, t.bottom-n+1, t.cols-1, t.bottom)
	t.changed |= ChangedScreen
	for i := t.bottom; i >= orig+n; i-- {
		t.lines[i], t.lines[i-n] = t.lines[i-n], t.lines[i]
		t.dirty[i] = true
		t.dirty[i-n] = true
	}

	// TODO: selection scroll
}

func (t *State) scrollUp(orig, n int) {
	n = clamp(n, 0, t.bottom-orig+1)
	t.clear(0, orig, t.cols-1, orig+n-1)
	t.changed |= ChangedScreen
	for i := orig; i <= t.bottom-n; i++ {
		t.lines[i], t.lines[i+n] = t.lines[i+n], t.lines[i]
		t.dirty[i] = true
		t.dirty[i+n] = true
	}

	// TODO: selection scroll
}

func (t *State) modMode(set bool, bit ModeFlag) {
	if set {
		t.mode |= bit
	} else {
		t.mode &^= bit
	}
}

func (t *State) setMode(priv bool, set bool, args []int) {
	if priv {
		for _, a := range args {
			switch a {
			case 1: // DECCKM - cursor key
				t.modMode(set, ModeAppCursor)
			case 5: // DECSCNM - reverse video
				mode := t.mode
				t.modMode(set, ModeReverse)
				if mode != t.mode {
					// TODO: redraw
				}
			case 6: // DECOM - origin
				if set {
					t.cur.State |= cursorOrigin
				} else {
					t.cur.State &^= cursorOrigin
				}
				t.moveAbsTo(0, 0)
			case 7: // DECAWM - auto wrap
				t.modMode(set, ModeWrap)
			// IGNORED:
			case 0, // error
				2,  // DECANM - ANSI/VT52
				3,  // DECCOLM - column
				4,  // DECSCLM - scroll
				8,  // DECARM - auto repeat
				18, // DECPFF - printer feed
				19, // DECPEX - printer extent
				42, // DECNRCM - national characters
				12: // att610 - start blinking cursor
				break
			case 25: // DECTCEM - text cursor enable mode
				t.modMode(!set, ModeHide)
			case 9: // X10 mouse compatibility mode
				t.modMode(false, ModeMouseMask)
				t.modMode(set, ModeMouseX10)
			case 1000: // report button press
				t.modMode(false, ModeMouseMask)
				t.modMode(set, ModeMouseButton)
			case 1002: // report motion on button press
				t.modMode(false, ModeMouseMask)
				t.modMode(set, ModeMouseMotion)
			case 1003: // enable all mouse motions
				t.modMode(false, ModeMouseMask)
				t.modMode(set, ModeMouseMany)
			case 1004: // send focus events to tty
				t.modMode(set, ModeFocus)
			case 1006: // extended reporting mode
				t.modMode(set, ModeMouseSgr)
			case 1034:
				t.modMode(set, Mode8bit)
			case 1049, // = 1047 and 1048
				47, 1047:
				alt := t.mode&ModeAltScreen != 0
				if alt {
					t.clear(0, 0, t.cols-1, t.rows-1)
				}
				if !set || !alt {
					t.swapScreen()
				}
				if a != 1049 {
					break
				}
				fallthrough
			case 1048:
				if set {
					t.saveCursor()
				} else {
					t.restoreCursor()
				}
			case 1001:
				// mouse highlight mode; can hang the terminal by design when
				// implemented
			case 1005:
				// utf8 mouse mode; will confuse applications not supporting
				// utf8 and luit
			case 1015:
				// urxvt mangled mouse mode; incompatiblt and can be mistaken
				// for other control codes
			default:
				t.logf("unknown private set/reset mode %d\n", a)
			}
		}
	} else {
		for _, a := range args {
			switch a {
			case 0: // Error (ignored)
			case 2: // KAM - keyboard action
				t.modMode(set, ModeKeyboardLock)
			case 4: // IRM - insertion-replacement
				t.modMode(set, ModeInsert)
				t.logln("insert mode not implemented")
			case 12: // SRM - send/receive
				t.modMode(set, ModeEcho)
			case 20: // LNM - linefeed/newline
				t.modMode(set, ModeCRLF)
			case 34:
				t.logln("right-to-left mode not implemented")
			case 96:
				t.logln("right-to-left copy mode not implemented")
			default:
				t.logf("unknown set/reset mode %d\n", a)
			}
		}
	}
}

func (t *State) setAttr(attr []int) {
	if len(attr) == 0 {
		attr = []int{0}
	}
	for i := 0; i < len(attr); i++ {
		a := attr[i]
		switch a {
		case 0:
			t.cur.Attr.Mode &^= attrReverse | attrUnderline | attrBold | attrItalic | attrBlink
			t.cur.Attr.FG = DefaultFG
			t.cur.Attr.BG = DefaultBG
		case 1:
			t.cur.Attr.Mode |= attrBold
		case 3:
			t.cur.Attr.Mode |= attrItalic
		case 4:
			t.cur.Attr.Mode |= attrUnderline
		case 5, 6: // slow, rapid blink
			t.cur.Attr.Mode |= attrBlink
		case 7:
			t.cur.Attr.Mode |= attrReverse
		case 21, 22:
			t.cur.Attr.Mode &^= attrBold
		case 23:
			t.cur.Attr.Mode &^= attrItalic
		case 24:
			t.cur.Attr.Mode &^= attrUnderline
		case 25, 26:
			t.cur.Attr.Mode &^= attrBlink
		case 27:
			t.cur.Attr.Mode &^= attrReverse
		case 38:
			if i+2 < len(attr) && attr[i+1] == 5 {
				i += 2
				if between(attr[i], 0, 255) {
					t.cur.Attr.FG = Color(attr[i])
				} else {
					t.logf("bad fgcolor %d\n", attr[i])
				}
			} else if i+4 < len(attr) && attr[i+1] == 2 {
				i += 4
				r, g, b := attr[i-2], attr[i-1], attr[i]
				if !between(r, 0, 255) || !between(g, 0, 255) || !between(b, 0, 255) {
					t.logf("bad fg rgb color (%d,%d,%d)\n", r, g, b)
				} else {
					t.cur.Attr.FG = Color(r<<16 | g<<8 | b)
				}
			} else {
				t.logf("gfx attr %d unknown\n", a)
			}
		case 39:
			t.cur.Attr.FG = DefaultFG
		case 48:
			if i+2 < len(attr) && attr[i+1] == 5 {
				i += 2
				if between(attr[i], 0, 255) {
					t.cur.Attr.BG = Color(attr[i])
				} else {
					t.logf("bad bgcolor %d\n", attr[i])
				}
			} else if i+4 < len(attr) && attr[i+1] == 2 {
				i += 4
				r, g, b := attr[i-2], attr[i-1], attr[i]
				if !between(r, 0, 255) || !between(g, 0, 255) || !between(b, 0, 255) {
					t.logf("bad bg rgb color (%d,%d,%d)\n", r, g, b)
				} else {
					t.cur.Attr.BG = Color(r<<16 | g<<8 | b)
				}
			} else {
				t.logf("gfx attr %d unknown\n", a)
			}
		case 49:
			t.cur.Attr.BG = DefaultBG
		default:
			if between(a, 30, 37) {
				t.cur.Attr.FG = Color(a - 30)
			} else if between(a, 40, 47) {
				t.cur.Attr.BG = Color(a - 40)
			} else if between(a, 90, 97) {
				t.cur.Attr.FG = Color(a - 90 + 8)
			} else if between(a, 100, 107) {
				t.cur.Attr.BG = Color(a - 100 + 8)
			} else {
				t.logf("gfx attr %d unknown\n", a)
			}
		}
	}
}

func (t *State) insertBlanks(n int) {
	src := t.cur.X
	dst := src + n
	size := t.cols - dst
	t.changed |= ChangedScreen
	t.dirty[t.cur.Y] = true

	if dst >= t.cols {
		t.clear(t.cur.X, t.cur.Y, t.cols-1, t.cur.Y)
	} else {
		copy(t.lines[t.cur.Y][dst:dst+size], t.lines[t.cur.Y][src:src+size])
		t.clear(src, t.cur.Y, dst-1, t.cur.Y)
	}
}

func (t *State) insertBlankLines(n int) {
	if t.cur.Y < t.top || t.cur.Y > t.bottom {
		return
	}
	t.scrollDown(t.cur.Y, n)
}

func (t *State) deleteLines(n int) {
	if t.cur.Y < t.top || t.cur.Y > t.bottom {
		return
	}
	t.scrollUp(t.cur.Y, n)
}

func (t *State) deleteChars(n int) {
	src := t.cur.X + n
	dst := t.cur.X
	size := t.cols - src
	t.changed |= ChangedScreen
	t.dirty[t.cur.Y] = true

	if src >= t.cols {
		t.clear(t.cur.X, t.cur.Y, t.cols-1, t.cur.Y)
	} else {
		copy(t.lines[t.cur.Y][dst:dst+size], t.lines[t.cur.Y][src:src+size])
		t.clear(t.cols-n, t.cur.Y, t.cols-1, t.cur.Y)
	}
}

func (t *State) setTitle(title string) {
	t.changed |= ChangedTitle
	t.title = title
}

func (t *State) Size() (cols, rows int) {
	return t.cols, t.rows
}

func (t *State) String() string {
	t.Lock()
	defer t.Unlock()

	var view []rune
	for y := 0; y < t.rows; y++ {
		for x := 0; x < t.cols; x++ {
			attr := t.Cell(x, y)
			view = append(view, attr.Char)
		}
		view = append(view, '\n')
	}

	return string(view)
}

// DumpState returns the terminal state needed to recreate it via ANSI sequences
func (t *State) DumpState() TerminalState {
	t.mu.Lock()
	defer t.mu.Unlock()

	state := TerminalState{
		Cols:          t.cols,
		Rows:          t.rows,
		CursorX:       t.cur.X,
		CursorY:       t.cur.Y,
		CursorVisible: t.mode&ModeHide == 0,
		AltScreen:     t.mode&ModeAltScreen != 0,
		ScrollTop:     t.top,
		ScrollBottom:  t.bottom,
		Title:         t.title,
		SavedCursorX:  t.curSaved.X,
		SavedCursorY:  t.curSaved.Y,

		// Terminal modes
		Wrap:         t.mode&ModeWrap != 0,
		Insert:       t.mode&ModeInsert != 0,
		Origin:       t.cur.State&cursorOrigin != 0,
		AutoWrap:     t.mode&ModeWrap != 0, // Same as Wrap
		ReverseVideo: t.mode&ModeReverse != 0,
	}

	// Collect tab stops
	for i, isTab := range t.tabs {
		if isTab {
			state.TabStops = append(state.TabStops, i)
		}
	}

	// Copy primary buffer
	state.PrimaryBuffer = make([][]Glyph, t.rows)
	for y := 0; y < t.rows; y++ {
		state.PrimaryBuffer[y] = make([]Glyph, t.cols)
		for x := 0; x < t.cols; x++ {
			if y < len(t.lines) && x < len(t.lines[y]) {
				g := t.lines[y][x]
				state.PrimaryBuffer[y][x] = g
			}
		}
	}

	// Copy alternate buffer
	state.AlternateBuffer = make([][]Glyph, t.rows)
	for y := 0; y < t.rows; y++ {
		state.AlternateBuffer[y] = make([]Glyph, t.cols)
		for x := 0; x < t.cols; x++ {
			if y < len(t.altLines) && x < len(t.altLines[y]) {
				g := t.altLines[y][x]
				state.AlternateBuffer[y][x] = g
			}
		}
	}

	return state
}

func (t *State) ANSI() string {
	state := t.DumpState()

	var buf bytes.Buffer

	// Reset terminal to clean state
	buf.WriteString("\x1b[!p") // Soft reset (DECSTR)
	buf.WriteString("\x1b[2J") // Clear screen
	buf.WriteString("\x1b[H")  // Home cursor

	// Set terminal size if supported
	buf.WriteString(fmt.Sprintf("\x1b[8;%d;%dt", state.Rows, state.Cols))

	// Set window title
	if state.Title != "" {
		buf.WriteString(fmt.Sprintf("\x1b]0;%s\x07", state.Title))
	}

	// Set terminal modes
	if state.Wrap {
		buf.WriteString("\x1b[?7h")
	} else {
		buf.WriteString("\x1b[?7l")
	}

	if state.Insert {
		buf.WriteString("\x1b[4h")
	} else {
		buf.WriteString("\x1b[4l")
	}

	if state.Origin {
		buf.WriteString("\x1b[?6h")
	} else {
		buf.WriteString("\x1b[?6l")
	}

	if state.ReverseVideo {
		buf.WriteString("\x1b[?5h")
	} else {
		buf.WriteString("\x1b[?5l")
	}

	// Clear all tab stops and set custom ones
	buf.WriteString("\x1b[3g")
	for _, col := range state.TabStops {
		buf.WriteString(fmt.Sprintf("\x1b[%dG", col+1))
		buf.WriteString("\x1bH")
	}

	// Set scroll region
	if state.ScrollTop != 0 || state.ScrollBottom != state.Rows-1 {
		buf.WriteString(fmt.Sprintf("\x1b[%d;%dr", state.ScrollTop+1, state.ScrollBottom+1))
	}

	// Function to render a buffer to the ANSI output
	renderBuffer := func(buffer [][]Glyph) {
		// Reset all attributes before starting
		buf.WriteString("\x1b[0m")

		lastFG := Color(^uint32(0))
		lastBG := Color(^uint32(0))
		var lastMode int16 = -1

		for y := 0; y < len(buffer); y++ {
			buf.WriteString(fmt.Sprintf("\x1b[%d;1H", y+1))

			for x := 0; x < len(buffer[y]); x++ {
				glyph := buffer[y][x]
				var codes []string

				fg := glyph.FG
				bg := glyph.BG
				if glyph.Mode&attrReverse != 0 {
					fg, bg = bg, fg
				}

				if glyph.Mode != lastMode {
					if lastMode != -1 {
						codes = append(codes, "0")
						lastFG = Color(^uint32(0))
						lastBG = Color(^uint32(0))
					}

					if glyph.Mode&attrBold != 0 {
						codes = append(codes, "1")
					}
					if glyph.Mode&attrUnderline != 0 {
						codes = append(codes, "4")
					}
					if glyph.Mode&attrItalic != 0 {
						codes = append(codes, "3")
					}
					if glyph.Mode&attrBlink != 0 {
						codes = append(codes, "5")
					}
					lastMode = glyph.Mode
				}

				if fg != lastFG {
					if fg == DefaultFG {
						codes = append(codes, "39")
					} else if fg < 16 {
						if fg < 8 {
							codes = append(codes, fmt.Sprintf("%d", 30+fg))
						} else {
							codes = append(codes, fmt.Sprintf("%d", 90+fg-8))
						}
					} else if fg < 256 {
						codes = append(codes, "38", "5", fmt.Sprintf("%d", fg))
					} else {
						r := (fg >> 16) & 0xFF
						g := (fg >> 8) & 0xFF
						b := fg & 0xFF
						codes = append(codes, "38", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b))
					}
					lastFG = fg
				}

				if bg != lastBG {
					if bg == DefaultBG {
						codes = append(codes, "49")
					} else if bg < 16 {
						if bg < 8 {
							codes = append(codes, fmt.Sprintf("%d", 40+bg))
						} else {
							codes = append(codes, fmt.Sprintf("%d", 100+bg-8))
						}
					} else if bg < 256 {
						codes = append(codes, "48", "5", fmt.Sprintf("%d", bg))
					} else {
						r := (bg >> 16) & 0xFF
						g := (bg >> 8) & 0xFF
						b := bg & 0xFF
						codes = append(codes, "48", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b))
					}
					lastBG = bg
				}

				if len(codes) > 0 {
					buf.WriteString("\x1b[")
					buf.WriteString(strings.Join(codes, ";"))
					buf.WriteString("m")
				}

				if glyph.Char == 0 {
					buf.WriteRune(' ')
				} else {
					buf.WriteRune(glyph.Char)
				}
			}
		}

		// Reset at end
		buf.WriteString("\x1b[0m")
	}

	// Handle alternate screen mode
	if state.AltScreen {
		// 1. First render the alternate buffer (background/saved content)
		renderBuffer(state.AlternateBuffer)

		buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", state.SavedCursorY+1, state.SavedCursorX+1))

		// 3. Switch to alternate screen mode (this saves the current buffer and cursor)
		buf.WriteString("\x1b[?1049h")

		// 4. Clear and render the primary buffer (current active content)
		buf.WriteString("\x1b[2J") // Clear the alternate screen
		buf.WriteString("\x1b[H")  // Home cursor
		renderBuffer(state.PrimaryBuffer)
	} else {
		// Normal mode - just render primary buffer
		renderBuffer(state.PrimaryBuffer)
	}

	// Set final cursor position
	buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", state.CursorY+1, state.CursorX+1))

	// Set cursor visibility
	if state.CursorVisible {
		buf.WriteString("\x1b[?25h")
	} else {
		buf.WriteString("\x1b[?25l")
	}

	return buf.String()
}

// colorToANSI converts a Color to ANSI escape sequence parameters
func (t *State) colorToANSI(c Color, isForeground bool) string {
	base := 30
	if !isForeground {
		base = 40
	}

	switch {
	case c >= Black && c <= White:
		// Basic 8 colors
		return fmt.Sprintf("%d", base+int(c))
	case c >= 8 && c <= 15:
		// Bright colors
		return fmt.Sprintf("%d", base+60+int(c)-8)
	case c >= 16 && c <= 255:
		// 256 color palette
		if isForeground {
			return fmt.Sprintf("38;5;%d", c)
		}
		return fmt.Sprintf("48;5;%d", c)
	case c >= (1<<24) && c < (1<<25):
		// RGB color
		r := (c >> 16) & 0xFF
		g := (c >> 8) & 0xFF
		b := c & 0xFF
		if isForeground {
			return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
		}
		return fmt.Sprintf("48;2;%d;%d;%d", r, g, b)
	default:
		// Default or unknown
		if isForeground {
			return "39"
		}
		return "49"
	}
}
