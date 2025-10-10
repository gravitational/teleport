package ttyterminal

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/session"
)

// parser processes a stream of tokens and groups them into commands based on terminal control sequences.
// It maintains state about the terminal, such as size and mode, to accurately capture command input
// and output boundaries. When a complete command is identified, it is emitted through a channel.
// The parser currently only handles bracketed paste sequences to delineate command input and output,
// but can be extended to include heuristic command detection for older shells that do not support
// bracketed paste.
// This implementation is not thread-safe and should be used by a single goroutine.
type parser struct {
	commandChan chan *command
	closeOnce   sync.Once

	size              size
	bracketPasteDepth int
	alternateMode     bool

	state          parserState
	lastEventTime  time.Duration
	currentCommand *command
}

type size struct {
	cols, rows int
}

type parserState int

const (
	stateAwaitingPrompt parserState = iota
	stateInputtingCommand
	stateCapturingOutput
)

const parserCommandBufferSize = 1024

func newParser() *parser {
	return &parser{
		commandChan: make(chan *command, parserCommandBufferSize),
		state:       stateAwaitingPrompt,
	}
}

func (p *parser) processTokens(ctx context.Context, tokens <-chan token) error {
	defer p.cleanup()

	for {
		select {
		case token, ok := <-tokens:
			if !ok {
				return nil
			}

			if err := p.processToken(token); err != nil {
				return trace.Wrap(err)
			}

		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

func (p *parser) processToken(token token) error {
	p.lastEventTime = token.timestamp

	switch token.tokenType {
	case tokenResize:
		return p.handleResize(token)

	case tokenAlternateScreenEnter:
		return p.enterAlternateMode()

	case tokenAlternateScreenExit:
		return p.exitAlternateMode()

	case tokenBracketPasteStart:
		return p.handleBracketPasteStart(token)

	case tokenBracketPasteEnd:
		return p.handleBracketPasteEnd(token)

	case tokenText:
		return p.handleText(token)

	default:
		return nil
	}
}

func (p *parser) handleResize(token token) error {
	size, err := session.UnmarshalTerminalParams(string(token.data))
	if err != nil {
		return trace.Wrap(err)
	}

	p.size.cols = size.W
	p.size.rows = size.H

	if p.currentCommand != nil {
		return p.appendTokenToCurrentCommand(token)
	}

	return nil
}

func (p *parser) handleBracketPasteStart(token token) error {
	if p.alternateMode {
		return p.handleText(token)
	}

	if p.bracketPasteDepth == 0 {
		switch p.state {
		case stateAwaitingPrompt, stateCapturingOutput:
			p.emitCurrentCommand()

			p.currentCommand = &command{
				startTime: token.timestamp,
				input: commandData{
					startTime: token.timestamp,
					startSize: size{
						cols: p.size.cols,
						rows: p.size.rows,
					},
				},
			}

			p.state = stateInputtingCommand
		case stateInputtingCommand:
			if err := p.appendTokenToCurrentCommand(token); err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		if err := p.appendTokenToCurrentCommand(token); err != nil {
			return trace.Wrap(err)
		}
	}

	p.bracketPasteDepth += 1

	return nil
}

func (p *parser) handleBracketPasteEnd(token token) error {
	if p.alternateMode {
		return p.handleText(token)
	}

	if p.currentCommand == nil {
		return trace.BadParameter("unexpected bracket paste end without a current command")
	}

	if p.bracketPasteDepth <= 0 {
		return trace.BadParameter("unexpected bracket paste end without matching start")
	}

	p.bracketPasteDepth -= 1

	if p.bracketPasteDepth == 0 {
		p.state = stateCapturingOutput

		p.currentCommand.input.endTime = token.timestamp
		p.currentCommand.output.startTime = token.timestamp
		p.currentCommand.output.startSize = size{
			cols: p.size.cols,
			rows: p.size.rows,
		}
	} else {
		if err := p.appendTokenToCurrentCommand(token); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (p *parser) handleText(token token) error {
	return p.appendTokenToCurrentCommand(token)
}

func (p *parser) enterAlternateMode() error {
	p.alternateMode = true
	p.markCurrentCommandAlternateScreen(true)

	return nil
}

func (p *parser) exitAlternateMode() error {
	p.alternateMode = false

	return nil
}

func (p *parser) commands() <-chan *command {
	return p.commandChan
}

func (p *parser) emitCurrentCommand() {
	if p.currentCommand == nil {
		return
	}

	defer func() {
		p.currentCommand = nil
	}()

	if len(p.currentCommand.input.tokens) == 0 && len(p.currentCommand.output.tokens) == 0 {
		return
	}

	if p.currentCommand.input.endTime == 0 {
		p.currentCommand.input.endTime = p.lastEventTime
	}

	if p.currentCommand.output.endTime == 0 {
		p.currentCommand.output.endTime = p.lastEventTime
	}

	p.currentCommand.endTime = p.currentCommand.output.endTime

	p.commandChan <- p.currentCommand
}

func (p *parser) appendTokenToCurrentCommand(token token) error {
	if p.currentCommand == nil {
		return trace.BadParameter("no current command to append token to")
	}

	switch p.state {
	case stateAwaitingPrompt, stateInputtingCommand:
		p.currentCommand.input.tokens = append(p.currentCommand.input.tokens, token)

	case stateCapturingOutput:
		p.currentCommand.output.tokens = append(p.currentCommand.output.tokens, token)
	}

	return nil
}

func (p *parser) markCurrentCommandAlternateScreen(alternate bool) {
	if p.currentCommand == nil {
		return
	}

	switch p.state {
	case stateAwaitingPrompt, stateInputtingCommand:
		p.currentCommand.input.isAlternateScreen = alternate

	case stateCapturingOutput:
		p.currentCommand.output.isAlternateScreen = alternate
	}
}

func (p *parser) cleanup() {
	p.emitCurrentCommand()

	p.closeOnce.Do(func() {
		close(p.commandChan)
	})
}
