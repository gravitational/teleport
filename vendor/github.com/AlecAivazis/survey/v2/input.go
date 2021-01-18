package survey

import (
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
)

/*
Input is a regular text input that prints each character the user types on the screen
and accepts the input with the enter key. Response type is a string.

	name := ""
	prompt := &survey.Input{ Message: "What is your name?" }
	survey.AskOne(prompt, &name)
*/
type Input struct {
	Renderer
	Message       string
	Default       string
	Help          string
	Suggest       func(toComplete string) []string
	typedAnswer   string
	answer        string
	options       []core.OptionAnswer
	selectedIndex int
	showingHelp   bool
}

// data available to the templates when processing
type InputTemplateData struct {
	Input
	ShowAnswer    bool
	ShowHelp      bool
	Answer        string
	PageEntries   []core.OptionAnswer
	SelectedIndex int
	Config        *PromptConfig
}

// Templates with Color formatting. See Documentation: https://github.com/mgutz/ansi#style-format
var InputQuestionTemplate = `
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default+hb"}}{{ .Message }} {{color "reset"}}
{{- if .ShowAnswer}}
  {{- color "cyan"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else if .PageEntries -}}
  {{- .Answer}} [Use arrows to move, enter to select, type to continue]
  {{- "\n"}}
  {{- range $ix, $choice := .PageEntries}}
    {{- if eq $ix $.SelectedIndex }}{{color $.Config.Icons.SelectFocus.Format }}{{ $.Config.Icons.SelectFocus.Text }} {{else}}{{color "default"}}  {{end}}
    {{- $choice.Value}}
    {{- color "reset"}}{{"\n"}}
  {{- end}}
{{- else }}
  {{- if or (and .Help (not .ShowHelp)) .Suggest }}{{color "cyan"}}[
    {{- if and .Help (not .ShowHelp)}}{{ print .Config.HelpInput }} for help {{- if and .Suggest}}, {{end}}{{end -}}
    {{- if and .Suggest }}{{color "cyan"}}{{ print .Config.SuggestInput }} for suggestions{{end -}}
  ]{{color "reset"}} {{end}}
  {{- if .Default}}{{color "white"}}({{.Default}}) {{color "reset"}}{{end}}
  {{- .Answer -}}
{{- end}}`

func (i *Input) OnChange(key rune, config *PromptConfig) (bool, error) {
	if key == terminal.KeyEnter || key == '\n' {
		if i.answer != config.HelpInput || i.Help == "" {
			// we're done
			return true, nil
		} else {
			i.answer = ""
			i.showingHelp = true
		}
	} else if key == terminal.KeyDeleteWord || key == terminal.KeyDeleteLine {
		i.answer = ""
	} else if key == terminal.KeyEscape && i.Suggest != nil {
		if len(i.options) > 0 {
			i.answer = i.typedAnswer
		}
		i.options = nil
	} else if key == terminal.KeyArrowUp && len(i.options) > 0 {
		if i.selectedIndex == 0 {
			i.selectedIndex = len(i.options) - 1
		} else {
			i.selectedIndex--
		}
		i.answer = i.options[i.selectedIndex].Value
	} else if (key == terminal.KeyArrowDown || key == terminal.KeyTab) && len(i.options) > 0 {
		if i.selectedIndex == len(i.options)-1 {
			i.selectedIndex = 0
		} else {
			i.selectedIndex++
		}
		i.answer = i.options[i.selectedIndex].Value
	} else if key == terminal.KeyTab && i.Suggest != nil {
		options := i.Suggest(i.answer)
		i.selectedIndex = 0
		i.typedAnswer = i.answer
		if len(options) > 0 {
			i.answer = options[0]
			if len(options) == 1 {
				i.options = nil
			} else {
				i.options = core.OptionAnswerList(options)
			}
		}
	} else if key == terminal.KeyDelete || key == terminal.KeyBackspace {
		if i.answer != "" {
			i.answer = i.answer[0 : len(i.answer)-1]
		}
	} else if key >= terminal.KeySpace {
		i.answer += string(key)
		i.typedAnswer = i.answer
		i.options = nil
	}

	pageSize := config.PageSize
	opts, idx := paginate(pageSize, i.options, i.selectedIndex)
	err := i.Render(
		InputQuestionTemplate,
		InputTemplateData{
			Input:         *i,
			Answer:        i.answer,
			ShowHelp:      i.showingHelp,
			SelectedIndex: idx,
			PageEntries:   opts,
			Config:        config,
		},
	)

	return err != nil, err
}

func (i *Input) Prompt(config *PromptConfig) (interface{}, error) {
	// render the template
	err := i.Render(
		InputQuestionTemplate,
		InputTemplateData{
			Input:  *i,
			Config: config,
		},
	)
	if err != nil {
		return "", err
	}

	// start reading runes from the standard in
	rr := i.NewRuneReader()
	rr.SetTermMode()
	defer rr.RestoreTermMode()

	cursor := i.NewCursor()
	cursor.Hide()       // hide the cursor
	defer cursor.Show() // show the cursor when we're done

	// start waiting for input
	for {
		r, _, err := rr.ReadRune()
		if err != nil {
			return "", err
		}
		if r == terminal.KeyInterrupt {
			return "", terminal.InterruptErr
		}
		if r == terminal.KeyEndTransmission {
			break
		}

		b, err := i.OnChange(r, config)
		if err != nil {
			return "", err
		}

		if b {
			break
		}
	}

	// if the line is empty
	if len(i.answer) == 0 {
		// use the default value
		return i.Default, err
	}

	lineStr := i.answer

	i.AppendRenderedText(lineStr)

	// we're done
	return lineStr, err
}

func (i *Input) Cleanup(config *PromptConfig, val interface{}) error {
	// use the default answer when cleaning up the prompt if necessary
	ans := i.answer
	if ans == "" && i.Default != "" {
		ans = i.Default
	}

	// render the cleanup
	return i.Render(
		InputQuestionTemplate,
		InputTemplateData{
			Input:      *i,
			ShowAnswer: true,
			Config:     config,
			Answer:     ans,
		},
	)
}
