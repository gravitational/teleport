/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package testercli

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mark3labs/mcp-go/mcp"

	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

type modelState int

const (
	modelStateInit modelState = iota
	modelStateMainMenu

	modelStateToolsList
	modelStateToolsCallInput
	modelStateToolsCallOutput

	modelStateResourcesList
	modelStateResourceRead

	modelStateLogs
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type model struct {
	cfg    Config
	width  int
	height int

	spinner  spinner.Model
	mainMenu table.Model
	logsView viewport.Model

	toolsCallInputIndex int
	toolsCall           mcp.Tool
	toolsCallInputs     []textinput.Model
	toolsCallResult     *mcp.CallToolResult
	toolsListResult     *mcp.ListToolsResult
	toolsListModel      table.Model

	resourcesReadResult *mcp.ReadResourceResult
	resourcesListResult *mcp.ListResourcesResult
	resourcesListModel  table.Model

	footMenu *footMenu
	client   *testerCLI
	state    modelState
	mu       sync.Mutex
}

func (m *model) initSpinner() {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.spinner = s
	m.footMenu = &footMenu{
		focused: true,
	}
}

func (m *model) Init() tea.Cmd {
	m.initSpinner()
	go func() {
		client, err := m.connect()
		if err != nil {
			m.Update(err)
		} else {
			m.Update(client)
		}
	}()
	return tea.Sequence(
		tea.SetWindowTitle("Teleport MCP Tester CLI"),
		m.spinner.Tick,
	)
}

func (m *model) connect() (*testerCLI, error) {
	return run(context.Background(), m.cfg)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateLocked(msg)
}

func (m *model) close() {
	if m.client != nil {
		m.client.client.Close()
	}
}

func (m *model) updateLocked(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.toolsListModel.SetWidth(msg.Width)
		m.mainMenu.SetWidth(msg.Width)
		m.logsView.Width = msg.Width - 4
		m.logsView.Height = msg.Height - 6
		return m, nil
	case *testerCLI:
		m.client = msg
		m.mainMenu = makeMainMenu(m.width)
		m.state = modelStateMainMenu
		m.footMenu.focused = true
		m.footMenu.actions = []footMenuAction{
			footMenuSelect,
			footMenuQuit,
		}
	case *mcp.ListToolsResult:
		m.state = modelStateToolsList
		m.toolsListResult = msg
		m.toolsListModel = makeToolsListModel(msg, m.width)
		m.footMenu.focused = true
		m.footMenu.actions = []footMenuAction{
			footMenuSelect,
			footMenuBack,
			footMenuQuit,
		}
	case *mcp.ListResourcesResult:
		m.state = modelStateResourcesList
		m.resourcesListResult = msg
		m.resourcesListModel = makeResourcesListModel(msg, m.width)
		m.footMenu.focused = true
		m.footMenu.actions = []footMenuAction{
			footMenuSelect,
			footMenuBack,
			footMenuQuit,
		}
	case *mcp.CallToolResult:
		m.state = modelStateToolsCallOutput
		m.toolsCallResult = msg
		m.footMenu.focused = true
		m.footMenu.actions = []footMenuAction{
			footMenuBack,
			footMenuQuit,
		}
	case *mcp.ReadResourceResult:
		m.state = modelStateResourceRead
		m.resourcesReadResult = msg
		m.footMenu.focused = true
		m.footMenu.actions = []footMenuAction{
			footMenuBack,
			footMenuQuit,
		}
	case mcp.Tool:
		m.toolsCall = msg
		m.state = modelStateToolsCallInput
		params := maps.Keys(m.toolsCall.InputSchema.Properties)
		var inputs []textinput.Model
		for _, name := range slices.Sorted(params) {
			t := textinput.New()
			t.Width = m.width - 4
			t.Placeholder = name
			t.CharLimit = 80
			t.Cursor.Style = focusedStyle
			inputs = append(inputs, t)
		}
		m.toolsCallInputIndex = 0
		m.toolsCallInputs = inputs
		m.footMenu.focused = false
		m.footMenu.actions = []footMenuAction{
			footMenuSubmit,
			footMenuBack,
			footMenuQuit,
		}
	case tea.KeyMsg:
		if m.state == modelStateToolsCallInput {
			switch msg.Type {
			case tea.KeyUp:
				if m.toolsCallInputIndex == len(m.toolsCallInputs) {
					m.footMenu.focused = false
				}
				m.toolsCallInputIndex = max(m.toolsCallInputIndex-1, 0)
			case tea.KeyDown, tea.KeyEnter:
				m.toolsCallInputIndex = min(m.toolsCallInputIndex+1, len(m.toolsCallInputs))
				if m.toolsCallInputIndex == len(m.toolsCallInputs) {
					m.footMenu.focused = true
				}
			}
		}
		switch m.footMenu.update(msg) {
		case footMenuSubmit:
			switch m.state {
			case modelStateToolsCallInput:
				args := make(map[string]any)
				for _, input := range m.toolsCallInputs {
					key := input.Placeholder
					value := strings.TrimSpace(input.Value())
					if boolValue, err := strconv.ParseBool(value); err == nil {
						args[key] = boolValue
					} else if intValue, err := strconv.Atoi(value); err == nil {
						args[key] = intValue
					} else if doubleValue, err := strconv.ParseFloat(value, 64); err == nil {
						args[key] = doubleValue
					} else if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
						args[key] = value[1 : len(value)-1]
					} else {
						args[key] = value
					}
				}
				callResult, err := m.client.client.CallTool(context.TODO(), mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Name:      m.toolsCall.Name,
						Arguments: args,
					},
				})
				if err != nil {
					return m.updateLocked(err)
				} else {
					return m.updateLocked(callResult)
				}
			}
		case footMenuQuit:
			return m, tea.Quit
		case footMenuBack:
			switch m.state {
			case modelStateToolsCallOutput, modelStateToolsCallInput:
				m.state = modelStateToolsList
				m.footMenu.focused = true
				m.footMenu.actions = []footMenuAction{
					footMenuSelect,
					footMenuBack,
					footMenuQuit,
				}
			case modelStateResourceRead:
				m.state = modelStateResourcesList
				m.footMenu.focused = true
				m.footMenu.actions = []footMenuAction{
					footMenuSelect,
					footMenuBack,
					footMenuQuit,
				}
			default:
				m.state = modelStateMainMenu
				m.footMenu.focused = true
				m.footMenu.actions = []footMenuAction{
					footMenuSelect,
					footMenuQuit,
				}
			}
			m.footMenu.cur = 0
		case footMenuSelect:
			switch m.state {
			case modelStateMainMenu:
				switch m.mainMenu.SelectedRow()[0] {
				case mainMenuToolsList.String():
					tools, err := m.client.client.ListTools(context.TODO(), mcp.ListToolsRequest{})
					switch {
					case err != nil:
						return m.updateLocked(err)
					default:
						//TODO handle empty tool
						return m.updateLocked(tools)
					}
				case mainMenuResourcesList.String():
					resources, err := m.client.client.ListResources(context.TODO(), mcp.ListResourcesRequest{})
					switch {
					case err != nil:
						return m.updateLocked(err)
					default:
						return m.updateLocked(resources)
					}
				case mainMenuProtocolLogs.String():
					var buf bytes.Buffer
					m.client.recorder.dump(&buf)
					wrapped := lipgloss.NewStyle().Width(m.logsView.Width).Render(buf.String())
					m.logsView.SetContent(wrapped)
					m.logsView.GotoBottom()
					m.state = modelStateLogs
					m.footMenu.focused = true
					m.footMenu.actions = []footMenuAction{
						footMenuBack,
						footMenuQuit,
					}
				}
			case modelStateToolsList:
				index := slices.IndexFunc(m.toolsListResult.Tools, func(t mcp.Tool) bool {
					return t.Name == m.toolsListModel.SelectedRow()[0]
				})
				if index < 0 {
					return m, nil
				}
				if len(m.toolsListResult.Tools[index].InputSchema.Properties) > 0 {
					return m.updateLocked(m.toolsListResult.Tools[index])
				}
				callResult, err := m.client.client.CallTool(context.TODO(), mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Name: m.toolsListModel.SelectedRow()[0],
					},
				})
				if err != nil {
					return m.updateLocked(err)
				} else {
					return m.updateLocked(callResult)
				}
			case modelStateResourcesList:
				index := slices.IndexFunc(m.resourcesListResult.Resources, func(t mcp.Resource) bool {
					return t.Name == m.resourcesListModel.SelectedRow()[0]
				})
				if index < 0 {
					return m, nil
				}
				readResult, err := m.client.client.ReadResource(context.TODO(), mcp.ReadResourceRequest{
					Params: mcp.ReadResourceParams{
						URI: m.resourcesListResult.Resources[index].URI,
					},
				})
				if err != nil {
					return m.updateLocked(err)
				} else {
					return m.updateLocked(readResult)
				}
			}
		}
	case error:
		return m, tea.Sequence(
			tea.Println(msg),
			tea.Quit,
		)
	}
	switch m.state {
	case modelStateInit:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case modelStateMainMenu:
		m.mainMenu, cmd = m.mainMenu.Update(msg)
		return m, cmd
	case modelStateToolsList:
		m.toolsListModel, cmd = m.toolsListModel.Update(msg)
		return m, cmd
	case modelStateResourcesList:
		m.resourcesListModel, cmd = m.resourcesListModel.Update(msg)
		return m, cmd
	case modelStateLogs:
		m.logsView, cmd = m.logsView.Update(msg)
		return m, cmd
	case modelStateToolsCallInput:
		cmds := make([]tea.Cmd, len(m.toolsCallInputs))
		// Only text inputs with Focus() set will respond, so it's safe to simply
		// update all of them here without any further logic.
		for i := range m.toolsCallInputs {
			if i == m.toolsCallInputIndex {
				// Set focused state
				m.toolsCallInputs[i].Focus()
				m.toolsCallInputs[i].PromptStyle = focusedStyle
				m.toolsCallInputs[i].TextStyle = focusedStyle
				continue
			}
			// Remove focused state
			m.toolsCallInputs[i].Blur()
			m.toolsCallInputs[i].PromptStyle = noStyle
			m.toolsCallInputs[i].TextStyle = noStyle
		}
		for i := range m.toolsCallInputs {
			m.toolsCallInputs[i], cmds[i] = m.toolsCallInputs[i].Update(msg)
		}
		return m, tea.Batch(cmds...)
	default:
		return m, nil
	}
}

func (m *model) View() string {
	switch m.state {
	case modelStateInit:
		return fmt.Sprintf("%s Executing command:\n%v %v\n",
			m.spinner.View(),
			m.cfg.Command,
			strings.Join(m.cfg.Args, " "),
		)
	case modelStateMainMenu:
		header := fmt.Sprintf(`🚀 Connected to %q (version %s)
`,
			m.client.init.ServerInfo.Name,
			m.client.init.ServerInfo.Version,
		)
		return header +
			baseStyle.Render(m.mainMenu.View()) +
			m.footMenu.view()
	case modelStateToolsList:
		return baseStyle.Render(m.toolsListModel.View()) +
			m.footMenu.view()
	case modelStateResourcesList:
		m.footMenu.focused = true
		m.footMenu.actions = []footMenuAction{
			footMenuSelect,
			footMenuBack,
			footMenuQuit,
		}
		return baseStyle.Render(m.resourcesListModel.View()) +
			m.footMenu.view()

	case modelStateToolsCallOutput:
		var sb strings.Builder
		sb.WriteString("✅ Success. Result:")
		for _, content := range m.toolsCallResult.Content {
			sb.WriteString("\n")
			switch t := content.(type) {
			case mcp.TextContent:
				sb.WriteString(t.Text)
			case mcp.ImageContent:
				sb.WriteString(fmt.Sprintf("<%T mimeType=%q dataSize=\"%d\">", t, t.MIMEType, len(t.Data)))
			case mcp.AudioContent:
				sb.WriteString(fmt.Sprintf("<%T mimeType=%q dataSize=\"%d\">", t, t.MIMEType, len(t.Data)))
			default:
				sb.WriteString(fmt.Sprintf("<%T>", t))
			}
		}
		return baseStyle.Render(sb.String()) + m.footMenu.view()

	case modelStateResourceRead:
		var sb strings.Builder
		sb.WriteString("✅ Success. Result:")
		for _, content := range m.resourcesReadResult.Contents {
			sb.WriteString("\n")
			switch t := content.(type) {
			case mcp.TextResourceContents:
				sb.WriteString(t.Text)
			case mcp.BlobResourceContents:
				sb.WriteString(fmt.Sprintf("<%T mimeType=%q dataSize=%d>", t, t.MIMEType, len(t.Blob)))
			default:
				sb.WriteString(fmt.Sprintf("<%T>", t))
			}
		}
		return baseStyle.Render(sb.String()) + m.footMenu.view()

	case modelStateLogs:
		percent := float64(m.logsView.ScrollPercent()) * 100
		footer := fmt.Sprintf("\n[ %3.0f%% ]", percent)
		footerStyled := lipgloss.NewStyle().
			Width(m.logsView.Width).
			Align(lipgloss.Right).
			Render(footer)
		return baseStyle.Render(m.logsView.View()+footerStyled) + m.footMenu.view()

	case modelStateToolsCallInput:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📝 Input parameters for tool %q:", m.toolsCall.Name))
		for i := range m.toolsCallInputs {
			sb.WriteRune('\n')
			sb.WriteString(m.toolsCallInputs[i].View())
		}
		return baseStyle.Render(sb.String()) + m.footMenu.view()

	default:
		return ""
	}
}

type mainMenuAction int

const (
	mainMenuToolsList mainMenuAction = iota
	mainMenuResourcesList
	mainMenuProtocolLogs
)

func (a mainMenuAction) String() string {
	switch a {
	case mainMenuToolsList:
		return "> tools"
	case mainMenuResourcesList:
		return "> resources"
	case mainMenuProtocolLogs:
		return "> protocol logs"
	default:
		return "<unknown>"
	}
}

func makeMainMenu(width int) table.Model {
	columns := []table.Column{
		{Title: "Main menu:", Width: width - 4},
	}
	actions := []mainMenuAction{
		mainMenuToolsList,
		mainMenuResourcesList,
		mainMenuProtocolLogs,
	}
	rows := sliceutils.Map(actions, func(a mainMenuAction) table.Row {
		return table.Row{a.String()}
	})
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(len(actions)+1),
		table.WithWidth(width-4),
	)
	setTableStyle(&t)
	return t
}

func setTableStyle(t *table.Model) {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)
}

func makeToolsListModel(msg *mcp.ListToolsResult, width int) table.Model {
	var rows []table.Row
	var toolNameWidth int
	for _, tool := range msg.Tools {
		if len(tool.Name) > toolNameWidth && len(tool.Name) < width/2 {
			toolNameWidth = len(tool.Name)
		}
		rows = append(rows, table.Row{
			tool.Name,
			tool.Description,
			strings.Join(slices.Collect(maps.Keys(tool.InputSchema.Properties)), ","),
		})
	}
	argWidth := (width - 4 - toolNameWidth) / 2
	descWidth := width - 8 - toolNameWidth - argWidth
	columns := []table.Column{
		{Title: "Tool Name", Width: toolNameWidth},
		{Title: "Description", Width: descWidth},
		{Title: "Arguments", Width: argWidth},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(20, len(msg.Tools))+1),
		table.WithWidth(width-4),
	)
	setTableStyle(&t)
	return t
}

func makeResourcesListModel(msg *mcp.ListResourcesResult, width int) table.Model {
	var rows []table.Row
	for _, resource := range msg.Resources {
		rows = append(rows, table.Row{
			resource.Name,
			resource.URI,
			resource.MIMEType,
			resource.Description,
		})
	}
	columns := []table.Column{
		{Title: "Resource Name", Width: 20},
		{Title: "URI", Width: 30},
		{Title: "Mime Type", Width: 20},
		{Title: "Description", Width: width - 80},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(20, len(msg.Resources))+1),
		table.WithWidth(width-4),
	)
	setTableStyle(&t)
	return t
}
