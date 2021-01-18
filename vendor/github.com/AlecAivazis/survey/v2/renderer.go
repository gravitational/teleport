package survey

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	goterm "golang.org/x/crypto/ssh/terminal"
)

type Renderer struct {
	stdio          terminal.Stdio
	renderedErrors bytes.Buffer
	renderedText   bytes.Buffer
}

type ErrorTemplateData struct {
	Error error
	Icon  Icon
}

var ErrorTemplate = `{{color .Icon.Format }}{{ .Icon.Text }} Sorry, your reply was invalid: {{ .Error.Error }}{{color "reset"}}
`

func (r *Renderer) WithStdio(stdio terminal.Stdio) {
	r.stdio = stdio
}

func (r *Renderer) Stdio() terminal.Stdio {
	return r.stdio
}

func (r *Renderer) NewRuneReader() *terminal.RuneReader {
	return terminal.NewRuneReader(r.stdio)
}

func (r *Renderer) NewCursor() *terminal.Cursor {
	return &terminal.Cursor{
		In:  r.stdio.In,
		Out: r.stdio.Out,
	}
}

func (r *Renderer) Error(config *PromptConfig, invalid error) error {
	// cleanup the currently rendered errors
	r.resetPrompt(r.countLines(r.renderedErrors))
	r.renderedErrors.Reset()

	// cleanup the rest of the prompt
	r.resetPrompt(r.countLines(r.renderedText))
	r.renderedText.Reset()

	userOut, layoutOut, err := core.RunTemplate(ErrorTemplate, &ErrorTemplateData{
		Error: invalid,
		Icon:  config.Icons.Error,
	})
	if err != nil {
		return err
	}

	// send the message to the user
	fmt.Fprint(terminal.NewAnsiStdout(r.stdio.Out), userOut)

	// add the printed text to the rendered error buffer so we can cleanup later
	r.appendRenderedError(layoutOut)

	return nil
}

func (r *Renderer) Render(tmpl string, data interface{}) error {
	// cleanup the currently rendered text
	lineCount := r.countLines(r.renderedText)
	r.resetPrompt(lineCount)
	r.renderedText.Reset()

	// render the template summarizing the current state
	userOut, layoutOut, err := core.RunTemplate(tmpl, data)
	if err != nil {
		return err
	}

	// print the summary
	fmt.Fprint(terminal.NewAnsiStdout(r.stdio.Out), userOut)

	// add the printed text to the rendered text buffer so we can cleanup later
	r.AppendRenderedText(layoutOut)

	// nothing went wrong
	return nil
}

// appendRenderedError appends text to the renderer's error buffer
// which is used to track what has been printed. It is not exported
// as errors should only be displayed via Error(config, error).
func (r *Renderer) appendRenderedError(text string) {
	r.renderedErrors.WriteString(text)
}

// AppendRenderedText appends text to the renderer's text buffer
// which is used to track of what has been printed. The buffer is used
// to calculate how many lines to erase before updating the prompt.
func (r *Renderer) AppendRenderedText(text string) {
	r.renderedText.WriteString(text)
}

func (r *Renderer) resetPrompt(lines int) {
	// clean out current line in case tmpl didnt end in newline
	cursor := r.NewCursor()
	cursor.HorizontalAbsolute(0)
	terminal.EraseLine(r.stdio.Out, terminal.ERASE_LINE_ALL)
	// clean up what we left behind last time
	for i := 0; i < lines; i++ {
		cursor.PreviousLine(1)
		terminal.EraseLine(r.stdio.Out, terminal.ERASE_LINE_ALL)
	}
}

func (r *Renderer) termWidth() (int, error) {
	fd := int(r.stdio.Out.Fd())
	termWidth, _, err := goterm.GetSize(fd)
	return termWidth, err
}

// countLines will return the count of `\n` with the addition of any
// lines that have wrapped due to narrow terminal width
func (r *Renderer) countLines(buf bytes.Buffer) int {
	w, err := r.termWidth()
	if err != nil || w == 0 {
		// if we got an error due to terminal.GetSize not being supported
		// on current platform then just assume a very wide terminal
		w = 10000
	}

	bufBytes := buf.Bytes()

	count := 0
	curr := 0
	delim := -1
	for curr < len(bufBytes) {
		// read until the next newline or the end of the string
		relDelim := bytes.IndexRune(bufBytes[curr:], '\n')
		if relDelim != -1 {
			count += 1 // new line found, add it to the count
			delim = curr + relDelim
		} else {
			delim = len(bufBytes) // no new line found, read rest of text
		}

		if lineWidth := utf8.RuneCount(bufBytes[curr:delim]); lineWidth > w {
			// account for word wrapping
			count += lineWidth / w
			if (lineWidth % w) == 0 {
				// content whose width is exactly a multiplier of available width should not
				// count as having wrapped on the last line
				count -= 1
			}
		}
		curr = delim + 1
	}

	return count
}
