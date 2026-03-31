package ttyterminal

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"github.com/hinshun/vt10x"

	tokenizerpkg "github.com/gravitational/teleport/e/lib/auth/summarizer/tokenizer"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils/set"
)

// Command represents a terminal command execution with methods to retrieve
// the number of output chunks and generate prompts for each chunk.
type Command interface {
	// ChunkCount returns the number of output chunks for the command.
	ChunkCount() int
	// PromptForChunk generates a prompt string for the specified output chunk index,
	// including the reconstructed input and output for that chunk.
	PromptForChunk(index int) string
	// StartOffset returns the start time of the command, relative to the start of the session.
	StartOffset() time.Duration
	// EndOffset returns the end time of the command, relative to the start of the session.
	EndOffset() time.Duration
	// RawInput returns the raw input text of the command.
	RawInput() string
}

// ReconstructedCommand represents the reconstructed terminal input and output
// for a single command execution.
type ReconstructedCommand struct {
	input  *reconstructedCommandData
	output *reconstructedCommandData
}

// StartOffset returns the start time of the command, relative to the start of the session.
func (r *ReconstructedCommand) StartOffset() time.Duration {
	return r.input.startTime
}

// EndOffset returns the end time of the command, relative to the start of the session.
func (r *ReconstructedCommand) EndOffset() time.Duration {
	return r.output.endTime
}

// RawInput returns the raw input text of the command by joining all input chunk lines.
func (r *ReconstructedCommand) RawInput() string {
	var sb strings.Builder
	for _, chunk := range r.input.chunks {
		for _, line := range chunk.lines {
			sb.WriteString(line.content)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// ChunkCount returns the number of output chunks for the command.
func (r *ReconstructedCommand) ChunkCount() int {
	return len(r.output.chunks)
}

// PromptForChunk generates a prompt string for the specified output chunk index,
// including the reconstructed input and output for that chunk.
func (r *ReconstructedCommand) PromptForChunk(chunkIndex int) string {
	var sb strings.Builder

	if len(r.output.chunks) > 1 {
		fmt.Fprintf(&sb, " (Chunk %d of %d)", chunkIndex+1, len(r.output.chunks))
	}

	sb.WriteString("\n================\n\n")

	sb.WriteString("INPUT:\n")
	fmt.Fprintf(&sb, "Duration: %v\n\n", r.input.endTime-r.input.startTime)

	if r.input.error != nil {
		fmt.Fprintf(&sb, "[Error recreating input: %v]\n", r.input.error)
	} else {
		for _, chunk := range r.input.chunks {
			for _, line := range chunk.lines {
				sb.WriteString(line.content)
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("OUTPUT:\n")

	fmt.Fprintf(&sb, "Duration: %v\n", r.output.endTime-r.output.startTime)

	if r.output.error != nil {
		fmt.Fprintf(&sb, "[Error recreating output: %v]\n", r.output.error)

		return sb.String()
	}

	if chunkIndex >= len(r.output.chunks) {
		return sb.String()
	}

	chunk := r.output.chunks[chunkIndex]

	if r.output.isAlternateScreen {
		sb.WriteString("[Note: This output is from alternate screen mode (e.g., vim, less, top)]\n")
		sb.WriteString("[The following are complete terminal snapshots captured at intervals, not incremental updates]\n\n")

		if chunk.initialState != nil {
			sb.WriteString("[Previous Terminal Snapshot]:\n")
			fmt.Fprintf(&sb, "%s\n", chunk.initialState.content)
			sb.WriteString("\n")
		}

		sb.WriteString("[Terminal Snapshots During Session]:\n")
		for i, line := range chunk.lines {
			fmt.Fprintf(&sb, "\n[Snapshot %d at %v]:\n", i+1, line.timestamp)
			fmt.Fprintf(&sb, "%s\n", line.content)
		}

		return sb.String()
	}

	if chunk.initialState != nil {
		sb.WriteString("[Terminal State at Start of Chunk]:\n")
		fmt.Fprintf(&sb, "%s\n", chunk.initialState.content)
		sb.WriteString("\n[Incremental Changes]:\n")
	}

	for _, line := range chunk.lines {
		fmt.Fprintf(&sb, "%s\n", line.content)
	}

	return sb.String()
}

type reconstructedCommandData struct {
	chunks             []chunk
	startTime, endTime time.Duration
	error              error
	isAlternateScreen  bool
	truncated          bool
}

// chunk represents a segment of terminal output, consisting of a series of line changes
// over time. Each chunk may include an initial state line representing the complete
// terminal state at the start of the chunk (except for the first chunk).
type chunk struct {
	initialState      *line
	lines             []line
	totalTokens       int
	isAlternateScreen bool
}

// line represents a change to a specific line in the terminal at a given timestamp.
// If lineNumber is -1, it represents a full terminal snapshot (used in alternate screen mode).
type line struct {
	lineNumber int // -1 for full terminal snapshot
	content    string
	timestamp  time.Duration
	tokenCount int
}

// reconstructCommand takes the recorded tokens for a command's input or output
// and reconstructs the terminal state into a series of chunks. Each chunk contains
// a sequence of lines representing changes to the terminal over time.
// In alternate screen mode, periodic snapshots of the full terminal state are taken
// to ensure context is preserved, since lines may be overwritten or cleared.
// The chunks are designed to be passed as separate inputs to a language model,
// while maintaining the full context of the terminal state.
//
// The maxTokensPerChunk parameter limits the number of tokens in each chunk,
// ensuring that the chunks fit within the constraints of the language model.
// The initial state of the terminal at the start of each chunk (except the first)
// does not count against the token limit, ensuring context is preserved.
func reconstructCommand(command commandData, maxTokensPerChunk, chunkLimit int) *reconstructedCommandData {
	duration := command.endTime - command.startTime

	c := &commandRecreator{
		activeLines:      set.New[int](),
		completedLines:   make(map[int][]rune),
		vt:               vt10x.New(vt10x.WithSize(command.startSize.cols, command.startSize.rows)),
		tokens:           command.tokens,
		alternateScreen:  command.isAlternateScreen,
		snapshotInterval: calculateSnapshotInterval(duration),
		tokenLimit:       maxTokensPerChunk,
		chunkLimit:       chunkLimit,
	}

	lines, err := c.reconstructTerminalOutput()
	if err != nil {
		return &reconstructedCommandData{
			chunks:            nil,
			startTime:         command.startTime,
			endTime:           command.endTime,
			error:             err,
			isAlternateScreen: command.isAlternateScreen,
			truncated:         false,
		}
	}

	chunks, truncated := c.chunkLines(lines)

	return &reconstructedCommandData{
		chunks:            chunks,
		startTime:         command.startTime,
		endTime:           command.endTime,
		error:             nil,
		isAlternateScreen: command.isAlternateScreen,
		truncated:         truncated,
	}
}

type commandRecreator struct {
	lines            []line
	activeLines      set.Set[int]
	completedLines   map[int][]rune
	vt               vt10x.Terminal
	tokens           []token
	alternateScreen  bool
	snapshotInterval time.Duration
	tokenLimit       int
	chunkLimit       int
}

type command struct {
	startTime, endTime time.Duration
	input, output      commandData
}

type commandData struct {
	startTime, endTime time.Duration
	startSize          size
	tokens             []token
	isAlternateScreen  bool
}

func (c *commandRecreator) reconstructTerminalOutput() ([]line, error) {
	var lastTimestamp time.Duration
	var snapshots []line

	for _, t := range c.tokens {
		switch t.tokenType {
		case tokenText:
			if err := c.write(t.data, t.timestamp); err != nil {
				return nil, trace.Wrap(err)
			}

			// in alternate screen mode, take periodic snapshots of the full terminal state
			// to capture the full context of the terminal output
			// since lines may be overwritten or cleared
			if c.alternateScreen {
				if t.timestamp-lastTimestamp >= c.snapshotInterval {
					snapshots = append(snapshots, c.generateTerminalSnapshot(t.timestamp))
				}
			}

		case tokenResize:
			if err := c.resize(t.data); err != nil {
				return nil, trace.Wrap(err, "resizing terminal")
			}
		}

		lastTimestamp = t.timestamp
	}

	if c.alternateScreen {
		snapshots = append(snapshots, c.generateTerminalSnapshot(lastTimestamp))

		return snapshots, nil
	}

	c.flushActiveLines(lastTimestamp)

	return c.lines, nil
}

// chunkLines splits the given lines into chunks based on the maxTokensPerChunk limit.
// Each chunk may include an initial state line representing the complete terminal
// state at the start of the chunk (except for the first chunk). The initial state
// does not count against the token limit for the chunk, ensuring context is preserved.
// In alternate screen mode, each line is already a complete snapshot, so no additional
// initial state lines are needed.
// This produces a series of chunks that can be passed as separate inputs to a language model,
// while maintaining the full context of the terminal state.
func (c *commandRecreator) chunkLines(lines []line) ([]chunk, bool) {
	if len(lines) == 0 {
		return []chunk{}, false
	}

	currentTokens := 0

	var chunks []chunk
	var currentChunk chunk
	var lastCompleteState *line

	currentChunk.isAlternateScreen = c.alternateScreen

	for i, line := range lines {
		lineTokens := line.tokenCount

		// Check if adding this line would exceed the chunk limit, not taking into account
		// the initial state line, which does not count against the limit.
		if currentTokens > 0 && currentTokens+lineTokens > c.tokenLimit {
			// Generate a snapshot of current terminal state for next chunk
			if i > 0 && !c.alternateScreen {
				snapshot := c.generateTerminalSnapshot(lines[i-1].timestamp)
				lastCompleteState = &snapshot
			}

			currentChunk.totalTokens = currentTokens

			if currentChunk.initialState != nil {
				currentChunk.totalTokens += currentChunk.initialState.tokenCount
			}

			chunks = append(chunks, currentChunk)

			if c.chunkLimit > 0 && len(chunks) >= c.chunkLimit {
				return chunks, true
			}

			// Start new chunk with the last complete state
			currentChunk = chunk{
				initialState:      lastCompleteState,
				isAlternateScreen: c.alternateScreen,
			}

			currentTokens = 0
		}

		currentChunk.lines = append(currentChunk.lines, line)
		currentTokens += lineTokens

		if c.alternateScreen {
			lineCopy := line
			lastCompleteState = &lineCopy
		}
	}

	// Add the last chunk if it has content
	if len(currentChunk.lines) > 0 {
		currentChunk.totalTokens = currentTokens

		if currentChunk.initialState != nil {
			currentChunk.totalTokens += currentChunk.initialState.tokenCount
		}

		chunks = append(chunks, currentChunk)
	}

	return chunks, false
}

func (c *commandRecreator) generateTerminalSnapshot(timestamp time.Duration) line {
	_, height := c.vt.Size()

	var lines [][]rune

	for y := range height {
		lines = append(lines, c.getTerminalLineContent(y))
	}

	// Trim trailing empty lines
	lastNonEmptyLine := len(lines) - 1
	for lastNonEmptyLine >= 0 && len(lines[lastNonEmptyLine]) == 0 {
		lastNonEmptyLine -= 1
	}

	var snapshot strings.Builder
	for i := 0; i <= lastNonEmptyLine; i++ {
		for _, r := range lines[i] {
			snapshot.WriteRune(r)
		}
		if i < lastNonEmptyLine {
			snapshot.WriteRune('\n')
		}
	}

	content := snapshot.String()
	return line{
		lineNumber: -1,
		content:    content,
		timestamp:  timestamp,
		tokenCount: tokenizerpkg.CountTokens(content),
	}
}

func (c *commandRecreator) resize(data []byte) error {
	size, err := session.UnmarshalTerminalParams(string(data))
	if err != nil {
		return trace.Wrap(err, "unmarshalling terminal resize data")
	}

	c.vt.Resize(size.W, size.H)

	// Remove any active lines that are now out of bounds
	maps.DeleteFunc(c.activeLines, func(lineNum int, _ struct{}) bool {
		return lineNum >= size.H
	})

	// Remove any completed lines that are now out of bounds
	maps.DeleteFunc(c.completedLines, func(lineNum int, _ []rune) bool {
		return lineNum >= size.H
	})

	return nil
}

// write writes data to the terminal and tracks changes to active lines.
// In alternate screen mode, it simply writes the data, as snapshots of the full terminal
// state will be taken periodically.
// In normal mode, it captures the previous content of active lines to detect clears or changes,
// and tracks the lines that were changed by the write operation.
func (c *commandRecreator) write(data []byte, timestamp time.Duration) error {
	if c.alternateScreen {
		_, err := c.vt.Write(data)
		return trace.Wrap(err, "writing terminal data in alternate screen mode")
	}

	// Capture the current content of all active lines before the write
	// so we can detect if they were cleared or changed
	activeLineContents := make(map[int][]rune)
	for lineNum := range c.activeLines {
		activeLineContents[lineNum] = c.getTerminalLineContent(lineNum)
	}

	changedLines, err := c.vt.WriteWithChanges(data)
	if err != nil {
		return trace.Wrap(err, "writing terminal data in normal mode")
	}

	c.trackLineChanges(changedLines, activeLineContents, timestamp)

	return nil
}

// trackLineChanges processes terminal line updates by distinguishing between active lines
// (currently being written) and completed lines (no longer being modified).
// When lines are changed, any previously active lines that are not currently being changed are
// considered complete and recorded with their final content. The most recently changed lines
// become active lines for future tracking.
// Additionally, it handles lines that have been cleared (content removed) by recording their last known content
// before they were cleared, ensuring that important output is not lost.
// This method helps maintain an accurate history of terminal output changes over time.
func (c *commandRecreator) trackLineChanges(changedLines []int, previousActiveContents map[int][]rune, timestamp time.Duration) {
	changesSet := set.New[int]()
	for _, lineNum := range changedLines {
		changesSet.Add(lineNum)
	}

	// Sort active lines to process them in order
	var activeLineNums []int
	for lineNum := range c.activeLines {
		activeLineNums = append(activeLineNums, lineNum)
	}

	sort.Ints(activeLineNums)

	for _, lineNum := range activeLineNums {
		previousContent := previousActiveContents[lineNum]
		currentContent := c.getTerminalLineContent(lineNum)

		// Record the line if it had content before being cleared
		if len(previousContent) != 0 && len(currentContent) == 0 {
			lastRecorded, exists := c.completedLines[lineNum]
			if !exists || !slices.Equal(lastRecorded, previousContent) {
				content := string(previousContent)
				c.lines = append(c.lines, line{
					lineNumber: lineNum,
					content:    content,
					timestamp:  timestamp,
					tokenCount: tokenizerpkg.CountTokens(content),
				})

				c.completedLines[lineNum] = previousContent
			}
		}

		if len(currentContent) == 0 {
			delete(c.activeLines, lineNum)
			continue
		}

		// If the line was changed in this write, skip processing - it will be handled when other lines
		// are changed, so we only record the final state of the line
		if _, changed := changesSet[lineNum]; changed {
			continue
		}

		lastContent, exists := c.completedLines[lineNum]
		if !exists || !slices.Equal(lastContent, currentContent) {
			content := string(currentContent)
			c.lines = append(c.lines, line{
				lineNumber: lineNum,
				content:    content,
				timestamp:  timestamp,
				tokenCount: tokenizerpkg.CountTokens(content),
			})

			c.completedLines[lineNum] = currentContent
		}

		delete(c.activeLines, lineNum)
	}

	for _, lineNum := range changedLines {
		c.activeLines.Add(lineNum)
	}
}

// flushActiveLines processes all currently active lines, checking their content
// against the last recorded content. If the content has changed, it records a new
// line change with the given timestamp. After processing, it clears the active lines.
func (c *commandRecreator) flushActiveLines(timestamp time.Duration) {
	activeLineContents := make(map[int][]rune)
	for lineNum := range c.activeLines {
		activeLineContents[lineNum] = c.getTerminalLineContent(lineNum)
	}

	// Pass empty changedLines to mark all active lines as completed
	c.trackLineChanges([]int{}, activeLineContents, timestamp)
}

func (c *commandRecreator) getTerminalLineContent(lineNum int) []rune {
	width, _ := c.vt.Size()

	// Find the last non-space character in the line, so we can trim trailing spaces
	lastNonSpace := -1
	for x := width - 1; x >= 0; x-- {
		glyph := c.vt.Cell(x, lineNum)
		r := glyph.Char
		if r != 0 && r != ' ' && isPrintableChar(r) {
			lastNonSpace = x
			break
		}
	}

	if lastNonSpace == -1 {
		return nil
	}

	result := make([]rune, lastNonSpace+1)

	for x := 0; x <= lastNonSpace; x++ {
		glyph := c.vt.Cell(x, lineNum)
		r := glyph.Char

		if r == 0 || r == ' ' || !isPrintableChar(r) {
			result[x] = ' '
		} else {
			result[x] = r
		}
	}

	return result
}

func isPrintableChar(r rune) bool {
	if r == utf8.RuneError {
		return false
	}

	if r >= '!' && r <= '~' {
		return true
	}

	// Box Drawing
	if r >= 0x2500 && r <= 0x257F {
		return true
	}

	// Block Elements
	if r >= 0x2580 && r <= 0x259F {
		return true
	}

	if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return true
	}

	return false
}

// calculateSnapshotInterval determines the interval at which to take full terminal
// snapshots in alternate screen mode, based on the total duration of the command.
// The goal is to balance context preservation with token efficiency by adjusting
// the interval to keep the number of snapshots within a target range.
//
// e.g.
// for a 5-second command, it would take a snapshot every 1 second, resulting in 5 snapshots total.
// for a 1-minute command, it would take a snapshot every 1 second, resulting in 60 snapshots total.
// for a 10-minute command, it would take a snapshot every 10 seconds, resulting in 60 snapshots total.
// for a 1-hour command, it would take a snapshot every 1 minute, resulting in 60 snapshots total.
// for a 3-hour command, it would take a snapshot every 1 minute, resulting in 180 snapshots total.
func calculateSnapshotInterval(duration time.Duration) time.Duration {
	const (
		targetMaxSnapshots = 60
		minInterval        = 1 * time.Second
		maxInterval        = 1 * time.Minute
		maxSnapshots       = 180
	)

	interval := duration / targetMaxSnapshots

	if interval < minInterval {
		return minInterval
	}

	if interval > maxInterval {
		interval = maxInterval
	}

	if duration/interval > maxSnapshots {
		interval = duration / maxSnapshots
	}

	return interval
}
