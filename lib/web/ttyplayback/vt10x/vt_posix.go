// +build linux darwin dragonfly solaris openbsd netbsd freebsd

package vt10x

import (
	"bufio"
	"bytes"
	"io"
	"unicode"
	"unicode/utf8"
)

type terminal struct {
	*State
}

func newTerminal(info TerminalInfo) *terminal {
	t := &terminal{newState(info.w)}
	t.init(info.cols, info.rows)
	return t
}

func (t *terminal) init(cols, rows int) {
	t.numlock = true
	t.state = t.parse
	t.cur.Attr.FG = DefaultFG
	t.cur.Attr.BG = DefaultBG
	t.Resize(cols, rows)
	t.reset()
}

// Write parses input and writes terminal changes to state.
func (t *terminal) Write(p []byte) (int, error) {
	var written int
	r := bytes.NewReader(p)
	t.lock()
	defer t.unlock()
	for {
		c, sz, err := r.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return written, err
		}
		written += sz
		if c == unicode.ReplacementChar && sz == 1 {
			if r.Len() == 0 {
				// not enough bytes for a full rune
				return written - 1, nil
			}
			t.logln("invalid utf8 sequence")
			continue
		}
		t.put(c)
	}
	return written, nil
}

// WriteWithChanges writes data to the terminal and returns the number of bytes written,
// along with the line numbers that were changed.
func (t *terminal) WriteWithChanges(p []byte) (int, []int, error) {
	var written int
	r := bytes.NewReader(p)
	t.lock()
	defer t.unlock()
	
	// Reset dirty flags before processing
	for i := range t.dirty {
		t.dirty[i] = false
	}
	t.anydirty = false
	
	for {
		c, sz, err := r.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return written, nil, err
		}
		written += sz
		if c == unicode.ReplacementChar && sz == 1 {
			if r.Len() == 0 {
				// not enough bytes for a full rune
				return written - 1, nil, nil
			}
			t.logln("invalid utf8 sequence")
			continue
		}
		t.put(c)
	}
	
	// Collect the changed line numbers
	var changedLines []int
	for i, isDirty := range t.dirty {
		if isDirty {
			changedLines = append(changedLines, i)
		}
	}
	
	return written, changedLines, nil
}

// TODO: add tests for expected blocking behavior
func (t *terminal) Parse(br *bufio.Reader) error {
	var locked bool
	defer func() {
		if locked {
			t.unlock()
		}
	}()
	for {
		c, sz, err := br.ReadRune()
		if err != nil {
			return err
		}
		if c == unicode.ReplacementChar && sz == 1 {
			t.logln("invalid utf8 sequence")
			break
		}
		if !locked {
			t.lock()
			locked = true
		}

		// put rune for parsing and update state
		t.put(c)

		// break if our buffer is empty, or if buffer contains an
		// incomplete rune.
		n := br.Buffered()
		if n == 0 || (n < 4 && !fullRuneBuffered(br)) {
			break
		}
	}
	return nil
}

func fullRuneBuffered(br *bufio.Reader) bool {
	n := br.Buffered()
	buf, err := br.Peek(n)
	if err != nil {
		return false
	}
	return utf8.FullRune(buf)
}

func (t *terminal) Resize(cols, rows int) {
	t.lock()
	defer t.unlock()
	_ = t.resize(cols, rows)
}

func (t *terminal) ResizeWithChanges(cols, rows int) []int {
	t.lock()
	defer t.unlock()
	
	// Get the current dimensions
	oldRows := t.rows
	
	// Perform the resize
	if !t.resize(cols, rows) {
		// No actual resize happened
		return nil
	}
	
	// Determine which lines changed
	minRows := min(rows, oldRows)
	changedLines := make([]int, 0)
	
	// All existing lines that fit in the new size are considered changed
	for i := 0; i < minRows; i++ {
		changedLines = append(changedLines, i)
	}
	
	// If we grew, all new lines are also changed
	if rows > oldRows {
		for i := oldRows; i < rows; i++ {
			changedLines = append(changedLines, i)
		}
	}
	
	return changedLines
}
