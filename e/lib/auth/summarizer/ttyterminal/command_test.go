package ttyterminal

import (
	"strings"
	"testing"
	"time"

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/set"
)

func TestCommandWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		writes           []writeOp
		expectedLines    []line
		expectedActive   set.Set[int]
		expectedComplete map[int][]rune
	}{
		{
			name: "single line write",
			writes: []writeOp{
				{data: []byte("Hello world"), timestamp: 1 * time.Second},
			},
			expectedLines:    nil, // Lines not flushed yet
			expectedActive:   set.New(0),
			expectedComplete: map[int][]rune{},
		},
		{
			name: "write on new line",
			writes: []writeOp{
				{data: []byte("Line 1\r\n"), timestamp: 1 * time.Second},
				{data: []byte("Line 2"), timestamp: 2 * time.Second},
			},
			expectedLines: []line{
				{lineNumber: 0, content: "Line 1", timestamp: 2 * time.Second, tokenCount: 2},
			},
			expectedActive:   set.New(1),
			expectedComplete: map[int][]rune{0: []rune("Line 1")},
		},
		{
			name: "overwrite same line",
			writes: []writeOp{
				{data: []byte("Progress: 10%"), timestamp: 1 * time.Second},
				{data: []byte("\rProgress: 20%"), timestamp: 2 * time.Second},
				{data: []byte("\rProgress: 30%\r\n"), timestamp: 3 * time.Second},
				{data: []byte("Done"), timestamp: 4 * time.Second},
			},
			expectedLines: []line{
				{lineNumber: 0, content: "Progress: 30%", timestamp: 4 * time.Second, tokenCount: 4},
			},
			expectedActive:   set.New(1),
			expectedComplete: map[int][]rune{0: []rune("Progress: 30%")},
		},
		{
			name: "clear line",
			writes: []writeOp{
				{data: []byte("Text to clear"), timestamp: 1 * time.Second},
				{data: []byte("\r\x1b[K"), timestamp: 2 * time.Second}, // Clear line
				{data: []byte("New text"), timestamp: 3 * time.Second},
			},
			expectedLines: []line{
				{lineNumber: 0, content: "Text to clear", timestamp: 2 * time.Second, tokenCount: 4},
			},
			expectedActive: set.New(0),
			expectedComplete: map[int][]rune{
				0: []rune("Text to clear"),
			},
		},
		{
			name: "multiple active lines",
			writes: []writeOp{
				{data: []byte("Line 1\r\nLine 2\r\nLine 3"), timestamp: 1 * time.Second},
			},
			expectedActive:   set.New(0, 1, 2),
			expectedComplete: map[int][]rune{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &commandRecreator{
				activeLines:    set.New[int](),
				completedLines: make(map[int][]rune),
				vt:             vt10x.New(vt10x.WithSize(80, 24)),
				counter:        fakeTokenCounter{},
			}

			for _, w := range tt.writes {
				err := c.write(w.data, w.timestamp)
				require.NoError(t, err)
			}

			require.Equal(t, tt.expectedLines, c.lines, "lines mismatch")
			require.Equal(t, tt.expectedActive, c.activeLines, "active lines mismatch")
			require.Equal(t, tt.expectedComplete, c.completedLines, "completed lines mismatch")
		})
	}
}

func TestCommandFlushActiveLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		writes        []writeOp
		expectedLines []line
	}{
		{
			name: "simple multi line write",
			writes: []writeOp{
				{data: []byte("Line 1\r\nLine 2\r\nLine 3"), timestamp: 1 * time.Second},
			},
			expectedLines: []line{
				{lineNumber: 0, content: "Line 1", timestamp: 1 * time.Second},
				{lineNumber: 1, content: "Line 2", timestamp: 1 * time.Second},
				{lineNumber: 2, content: "Line 3", timestamp: 1 * time.Second},
			},
		},
		{
			name: "line clear and overwrite",
			writes: []writeOp{
				{data: []byte("Initial text"), timestamp: 1 * time.Second},
				{data: []byte("\r\x1b[K"), timestamp: 2 * time.Second}, // Clear line
				{data: []byte("Updated text"), timestamp: 3 * time.Second},
			},
			expectedLines: []line{
				{lineNumber: 0, content: "Initial text", timestamp: 2 * time.Second},
				{lineNumber: 0, content: "Updated text", timestamp: 3 * time.Second},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &commandRecreator{
				activeLines:    set.New[int](),
				completedLines: make(map[int][]rune),
				vt:             vt10x.New(vt10x.WithSize(80, 24)),
				counter:        fakeTokenCounter{},
			}

			for _, w := range tt.writes {
				err := c.write(w.data, w.timestamp)
				require.NoError(t, err)
			}

			c.flushActiveLines(tt.writes[len(tt.writes)-1].timestamp)

			require.Len(t, c.lines, len(tt.expectedLines), "unexpected number of lines")

			for i, expectedLine := range tt.expectedLines {
				if i < len(c.lines) {
					actualLine := c.lines[i]
					require.Equal(t, expectedLine.lineNumber, actualLine.lineNumber, "line number mismatch at index %d", i)
					require.Equal(t, expectedLine.content, actualLine.content, "content mismatch at index %d", i)
					require.Equal(t, expectedLine.timestamp, actualLine.timestamp, "timestamp mismatch at index %d", i)
				}
			}
		})
	}
}

func TestCommandTruncatedChunks(t *testing.T) {
	t.Parallel()

	c := &commandRecreator{
		activeLines:    set.New[int](),
		completedLines: make(map[int][]rune),
		vt:             vt10x.New(vt10x.WithSize(80, 24)),
		tokenLimit:     50,
		chunkLimit:     3,
		counter:        fakeTokenCounter{},
	}

	for i := 0; i < 10; i++ {
		line := strings.Repeat("abc", 30)
		err := c.write([]byte(line+"\r\n"), time.Duration(i)*time.Second)
		require.NoError(t, err)
	}

	c.flushActiveLines(10 * time.Second)

	chunks, truncated := c.chunkLines(c.lines)

	require.True(t, truncated, "Expected chunks to be truncated due to limits")
	require.Len(t, chunks, c.chunkLimit, "Expected number of chunks to match chunk limit")
}

func TestCommandChunkLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		lines           []line
		alternateScreen bool
		expectedChunks  int
	}{
		{
			name:           "empty lines",
			lines:          []line{},
			expectedChunks: 0,
		},
		{
			name: "single chunk within limit",
			lines: []line{
				{lineNumber: 0, content: "Line 1", timestamp: 1 * time.Second, tokenCount: 10},
				{lineNumber: 1, content: "Line 2", timestamp: 2 * time.Second, tokenCount: 10},
			},
			expectedChunks: 1,
		},
		{
			name: "multiple chunks exceeding limit",
			lines: []line{
				{lineNumber: 0, content: strings.Repeat("a", 3000), timestamp: 1 * time.Second, tokenCount: 9000},
				{lineNumber: 1, content: strings.Repeat("b", 3000), timestamp: 2 * time.Second, tokenCount: 9000},
				{lineNumber: 2, content: strings.Repeat("c", 3000), timestamp: 3 * time.Second, tokenCount: 9000},
			},
			expectedChunks: 3,
		},
		{
			name: "alternate screen mode snapshots",
			lines: []line{
				{lineNumber: -1, content: "Snapshot 1", timestamp: 1 * time.Second, tokenCount: 100},
				{lineNumber: -1, content: "Snapshot 2", timestamp: 2 * time.Second, tokenCount: 100},
			},
			alternateScreen: true,
			expectedChunks:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &commandRecreator{
				activeLines:     set.New[int](),
				completedLines:  make(map[int][]rune),
				vt:              vt10x.New(vt10x.WithSize(80, 24)),
				lines:           tt.lines,
				alternateScreen: tt.alternateScreen,
				tokenLimit:      10000,
				chunkLimit:      10,
				counter:         fakeTokenCounter{},
			}

			chunks, _ := c.chunkLines(tt.lines)
			require.Len(t, chunks, tt.expectedChunks)

			for _, chunk := range chunks {
				require.Equal(t, tt.alternateScreen, chunk.isAlternateScreen)
			}
		})
	}
}

func TestCommandReconstructCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		command        commandData
		expectedChunks int
		validateChunks func(*testing.T, []chunk)
	}{
		{
			name: "simple text output",
			command: commandData{
				startTime: 0,
				endTime:   5 * time.Second,
				startSize: size{cols: 80, rows: 24},
				tokens: []token{
					{tokenType: tokenText, data: []byte("Hello world"), timestamp: 1 * time.Second},
					{tokenType: tokenText, data: []byte("\r\nSecond line"), timestamp: 2 * time.Second},
				},
			},
			expectedChunks: 1,
			validateChunks: func(t *testing.T, chunks []chunk) {
				require.Len(t, chunks, 1)
				chunk := chunks[0]
				require.False(t, chunk.isAlternateScreen)
				require.Nil(t, chunk.initialState) // First chunk has no initial state

				require.Len(t, chunk.lines, 2)

				require.Equal(t, "Hello world", chunk.lines[0].content)
				require.Equal(t, "Second line", chunk.lines[1].content)
			},
		},
		{
			name: "with resize",
			command: commandData{
				startTime: 0,
				endTime:   5 * time.Second,
				startSize: size{cols: 80, rows: 24},
				tokens: []token{
					{tokenType: tokenText, data: []byte("Before resize"), timestamp: 1 * time.Second},
					{tokenType: tokenResize, data: []byte("100:30"), timestamp: 2 * time.Second},
					{tokenType: tokenText, data: []byte("\r\nAfter resize"), timestamp: 3 * time.Second},
				},
			},
			expectedChunks: 1,
			validateChunks: func(t *testing.T, chunks []chunk) {
				require.Len(t, chunks, 1)
				chunk := chunks[0]
				require.False(t, chunk.isAlternateScreen)

				require.Equal(t, "Before resize", chunk.lines[0].content)
				require.Equal(t, "After resize", chunk.lines[1].content)
			},
		},
		{
			name: "alternate screen mode",
			command: commandData{
				startTime:         0,
				endTime:           10 * time.Second,
				startSize:         size{cols: 80, rows: 24},
				isAlternateScreen: true,
				tokens: []token{
					{tokenType: tokenText, data: []byte("Screen 1"), timestamp: 1 * time.Second},
					{tokenType: tokenText, data: []byte("\x1b[2J\x1b[H"), timestamp: 3 * time.Second}, // Clear screen
					{tokenType: tokenText, data: []byte("Screen 2"), timestamp: 5 * time.Second},
				},
			},
			expectedChunks: 1,
			validateChunks: func(t *testing.T, chunks []chunk) {
				require.Len(t, chunks, 1)
				chunk := chunks[0]
				require.True(t, chunk.isAlternateScreen)

				require.Len(t, chunk.lines, 4, "Should have two snapshots for two screen states")

				require.Equal(t, "Screen 1", chunk.lines[0].content)
				require.Empty(t, chunk.lines[1].content) // Cleared screen snapshot
				require.Equal(t, "Screen 2", chunk.lines[2].content)
				// Screenshot at the end should also contain Screen 2
				require.Equal(t, "Screen 2", chunk.lines[3].content)
			},
		},
		{
			name: "empty command",
			command: commandData{
				startTime: 0,
				endTime:   1 * time.Second,
				startSize: size{cols: 80, rows: 24},
				tokens:    []token{},
			},
			expectedChunks: 0,
			validateChunks: func(t *testing.T, chunks []chunk) {
				require.Empty(t, chunks)
			},
		},
		{
			name: "progress bar simulation",
			command: commandData{
				startTime: 0,
				endTime:   5 * time.Second,
				startSize: size{cols: 80, rows: 24},
				tokens: []token{
					{tokenType: tokenText, data: []byte("Progress: 0%"), timestamp: 1 * time.Second},
					{tokenType: tokenText, data: []byte("\rProgress: 25%"), timestamp: 2 * time.Second},
					{tokenType: tokenText, data: []byte("\rProgress: 50%"), timestamp: 3 * time.Second},
					{tokenType: tokenText, data: []byte("\rProgress: 75%"), timestamp: 4 * time.Second},
					{tokenType: tokenText, data: []byte("\rProgress: 100%\r\n"), timestamp: 5 * time.Second},
					{tokenType: tokenText, data: []byte("Done!"), timestamp: 5 * time.Second},
				},
			},
			expectedChunks: 1,
			validateChunks: func(t *testing.T, chunks []chunk) {
				require.Len(t, chunks, 1)
				chunk := chunks[0]

				require.Equal(t, "Progress: 100%", chunk.lines[0].content)
				require.Equal(t, "Done!", chunk.lines[1].content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconstructed := reconstructCommand(tt.command, 10000, 10, fakeTokenCounter{})

			require.NoError(t, reconstructed.error)

			require.Len(t, reconstructed.chunks, tt.expectedChunks, "Unexpected number of chunks")

			for i, chunk := range reconstructed.chunks {
				require.Equal(t, tt.command.isAlternateScreen, chunk.isAlternateScreen, "Chunk %d alternate screen flag mismatch", i)

				if len(chunk.lines) > 0 {
					require.Greater(t, chunk.totalTokens, 0, "Chunk %d with lines should have tokens counted", i)
				}

				for j := 1; j < len(chunk.lines); j++ {
					require.GreaterOrEqual(t, chunk.lines[j].timestamp, chunk.lines[j-1].timestamp,
						"Timestamps should be in order within chunk %d", i)
				}
			}

			tt.validateChunks(t, reconstructed.chunks)
		})
	}
}

// TestRecoverCommand verifies the defer/recover contract used by
// reconstructCommand: a panic inside the wrapped function (e.g. vt10x
// tripping over a corrupt recording) is converted into a per-command error
// result so surrounding commands still get summarized, and the returned error
// must not carry the panic reason or stack (which would otherwise leak into
// Summary.ErrorMessage / the LLM prompt).
//
// Written against the helper rather than through reconstructCommand so the
// assertion doesn't depend on vt10x's panic behavior for a specific byte
// sequence — vt10x is being hardened in parallel, and any trigger we pick
// there will eventually stop panicking.
func TestRecoverCommand(t *testing.T) {
	cmd := commandData{
		startTime:         2 * time.Second,
		endTime:           5 * time.Second,
		isAlternateScreen: true,
	}

	t.Run("panic becomes per-command error", func(t *testing.T) {
		result := recoverCommand(cmd, func() *reconstructedCommandData {
			panic("simulated vt10x panic")
		})
		require.NotNil(t, result)
		require.Error(t, result.error)
		require.Empty(t, result.chunks)
		require.Equal(t, cmd.startTime, result.startTime)
		require.Equal(t, cmd.endTime, result.endTime)
		require.Equal(t, cmd.isAlternateScreen, result.isAlternateScreen)
		require.Contains(t, result.error.Error(), "internal error reconstructing command")
		require.NotContains(t, result.error.Error(), "simulated vt10x panic")
		require.NotContains(t, result.error.Error(), "goroutine")
	})

	t.Run("non-panicking result passes through unchanged", func(t *testing.T) {
		want := &reconstructedCommandData{
			chunks:    []chunk{{lines: []line{{content: "ok"}}}},
			startTime: cmd.startTime,
			endTime:   cmd.endTime,
		}
		got := recoverCommand(cmd, func() *reconstructedCommandData { return want })
		require.Same(t, want, got)
	})
}

func TestCommandGenerateTerminalSnapshot(t *testing.T) {
	t.Parallel()

	c := &commandRecreator{
		vt:      vt10x.New(vt10x.WithSize(80, 24)),
		counter: fakeTokenCounter{},
	}

	_, err := c.vt.Write([]byte("Line 1\r\nLine 2\r\nLine 3"))
	require.NoError(t, err)

	snapshot := c.generateTerminalSnapshot(5 * time.Second)

	require.Equal(t, -1, snapshot.lineNumber)
	require.Equal(t, 5*time.Second, snapshot.timestamp)
	require.Contains(t, snapshot.content, "Line 1")
	require.Contains(t, snapshot.content, "Line 2")
	require.Contains(t, snapshot.content, "Line 3")
	require.Greater(t, snapshot.tokenCount, 0)
}

func FuzzReconstructCommand(f *testing.F) {
	f.Add([]byte("hello world"), int64(1000), int64(2000), 80, 24, 10000, 10, false)
	f.Add([]byte("line1\r\nline2\r\nline3"), int64(0), int64(5000), 100, 30, 5000, 5, false)
	f.Add([]byte("\x1b[2J\x1b[Hscreen clear"), int64(1000), int64(3000), 80, 24, 10000, 10, true)
	f.Add([]byte("Progress: 0%\rProgress: 100%"), int64(0), int64(1000), 80, 24, 10000, 10, false)

	f.Fuzz(func(t *testing.T, data []byte, startTime, endTime int64, cols, rows, tokenLimit, chunkLimit int, altScreen bool) {
		if cols < 1 || cols > 500 {
			cols = 80
		}
		if rows < 1 || rows > 500 {
			rows = 24
		}
		if tokenLimit < 100 {
			tokenLimit = 100
		}
		if tokenLimit > 100000 {
			tokenLimit = 100000
		}
		if chunkLimit < 0 {
			chunkLimit = 0
		}
		if chunkLimit > 1000 {
			chunkLimit = 1000
		}

		start := time.Duration(startTime)
		end := time.Duration(endTime)
		if end < start {
			start, end = end, start
		}
		if end < 0 {
			end = 0
		}

		timestamp := start + (end-start)/2
		if timestamp < start {
			timestamp = start
		}

		command := commandData{
			startTime:         start,
			endTime:           end,
			startSize:         size{cols: cols, rows: rows},
			isAlternateScreen: altScreen,
			tokens: []token{
				{tokenType: tokenText, data: data, timestamp: timestamp},
			},
		}

		result := reconstructCommand(command, tokenLimit, chunkLimit, fakeTokenCounter{})

		require.NotNil(t, result)
		require.NoError(t, result.error)
		require.Equal(t, start, result.startTime)
		require.Equal(t, end, result.endTime)
		require.Equal(t, altScreen, result.isAlternateScreen)

		if chunkLimit > 0 && len(result.chunks) > chunkLimit {
			require.True(t, result.truncated)
		}

		for i, chunk := range result.chunks {
			require.Equal(t, altScreen, chunk.isAlternateScreen)

			if i > 0 && !altScreen {
				require.GreaterOrEqual(t, chunk.totalTokens, 0)
			}

			for j := 1; j < len(chunk.lines); j++ {
				require.GreaterOrEqual(t, chunk.lines[j].timestamp, chunk.lines[j-1].timestamp)
			}

			for _, line := range chunk.lines {
				if !altScreen {
					require.GreaterOrEqual(t, line.lineNumber, -1)
					require.Less(t, line.lineNumber, rows)
				}
				require.GreaterOrEqual(t, line.tokenCount, 0)
			}
		}
	})
}

func FuzzReconstructCommandMultipleTokens(f *testing.F) {
	f.Add([]byte("first"), []byte("second"), []byte("third"), int64(1000), int64(2000), int64(3000), 80, 24)

	f.Fuzz(func(t *testing.T, data1, data2, data3 []byte, ts1, ts2, ts3 int64, cols, rows int) {
		if cols < 1 || cols > 500 {
			cols = 80
		}
		if rows < 1 || rows > 500 {
			rows = 24
		}

		timestamps := []int64{ts1, ts2, ts3}
		for i := 1; i < len(timestamps); i++ {
			if timestamps[i] < timestamps[i-1] {
				timestamps[i] = timestamps[i-1]
			}
		}

		command := commandData{
			startTime: time.Duration(timestamps[0]),
			endTime:   time.Duration(timestamps[2]),
			startSize: size{cols: cols, rows: rows},
			tokens: []token{
				{tokenType: tokenText, data: data1, timestamp: time.Duration(timestamps[0])},
				{tokenType: tokenText, data: data2, timestamp: time.Duration(timestamps[1])},
				{tokenType: tokenText, data: data3, timestamp: time.Duration(timestamps[2])},
			},
		}

		result := reconstructCommand(command, 10000, 10, fakeTokenCounter{})

		require.NotNil(t, result)
		require.NoError(t, result.error)

		for _, chunk := range result.chunks {
			for j := 1; j < len(chunk.lines); j++ {
				require.GreaterOrEqual(t, chunk.lines[j].timestamp, chunk.lines[j-1].timestamp)
			}
		}
	})
}

type writeOp struct {
	data      []byte
	timestamp time.Duration
}

type fakeTokenCounter struct{}

func (fakeTokenCounter) CountTokens(text string) int {
	return (len(text) + 3) / 4
}
