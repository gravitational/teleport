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

package kubetui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// view represents the current active view.
type view int

const (
	viewResourcePicker view = iota
	viewResourceList
	viewLogs
	viewDescribe
	viewNamespaces
	viewEdit
	viewConfirm
	viewPortForward
	viewContainerPicker
	viewFileCopy
)

// ModelOption is a functional option for NewModel.
type ModelOption func(*Model)

// WithPortForward enables the port-forward action for pods.
// This should only be used from tsh (CLI), not from the web UI.
func WithPortForward() ModelOption {
	return func(m *Model) { m.portForwardEnabled = true }
}

// WithFileCopy enables the copy action for pods.
// This should only be used from tsh (CLI), not from the web UI.
func WithFileCopy() ModelOption {
	return func(m *Model) { m.fileCopyEnabled = true }
}

// ExecRequest holds the parameters needed to exec into a pod.
type ExecRequest struct {
	Namespace string
	Pod       string
	Container string
}

// Model is the root Bubble Tea model for the Kubernetes TUI.
type Model struct {
	client    *Client
	cluster   string
	namespace string

	currentView view
	width       int
	height      int

	resourcePicker resourcePickerModel
	resourceList   resourceListModel
	logs           logModel
	describe       describeModel
	namespaces     namespaceModel
	edit           editModel
	confirm        confirmModel
	portForward     portForwardModel
	containerPicker containerPickerModel
	fileCopy        fileCopyModel

	portForwardEnabled bool
	fileCopyEnabled    bool

	// command buffer for vim-style commands like ":q", ":ns"
	commandMode   bool
	commandBuffer string

	// execRequest is set when the user picks "exec"; triggers tea.Quit
	// so the handler can run the K8s exec session outside Bubble Tea.
	execRequest *ExecRequest
}

// NewModel creates a new TUI model with the given Kubernetes client.
func NewModel(clientset kubernetes.Interface, restConfig *rest.Config, cluster string, w, h int, opts ...ModelOption) Model {
	client := NewClient(clientset, restConfig, cluster)
	m := Model{
		client:         client,
		cluster:        cluster,
		currentView:    viewResourcePicker,
		resourcePicker: newResourcePickerModel(),
		width:          w,
		height:         h,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return tea.WindowSizeMsg{
				Width:  m.width,
				Height: m.height,
			}
		},
		discoverAndRegisterResources(m.client),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.propagateSize(msg)

	case tea.KeyMsg:
		// Handle command mode
		if m.commandMode {
			return m.handleCommand(msg)
		}

		// Global key bindings
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys(":"))):
			// Only allow command mode outside the edit and confirm views
			if m.currentView != viewEdit && m.currentView != viewConfirm {
				m.commandMode = true
				m.commandBuffer = ""
				return m, nil
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
			if m.currentView == viewConfirm {
				return m.handleEscape()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			// Sub-views navigate back to resource list; resource list and
			// picker handle their own esc.
			switch m.currentView {
			case viewLogs:
				return m.handleEscape()
			case viewDescribe:
				return m.handleEscape()
			case viewNamespaces:
				return m.handleEscape()
			case viewEdit:
				return m.handleEscape()
			case viewConfirm:
				return m.handleEscape()
			case viewPortForward:
				return m.handleEscape()
			case viewContainerPicker:
				return m.handleEscape()
			case viewFileCopy:
				return m.handleEscape()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			return m, tea.Quit
		}

	case resourceTypeSelectedMsg:
		return m.switchToResourceType(msg.resourceType)

	case backToPickerMsg:
		m.currentView = viewResourcePicker
		return m, nil

	case resourceActionMsg:
		return m.handleResourceAction(msg)

	case resourcesLoadedMsg:
		var cmd tea.Cmd
		m.resourceList, cmd = m.resourceList.Update(msg)
		return m, cmd

	case namespacesLoadedMsg:
		var cmd tea.Cmd
		m.namespaces, cmd = m.namespaces.Update(msg)
		return m, cmd

	case resourceDescribedMsg:
		var cmd tea.Cmd
		m.describe, cmd = m.describe.Update(msg)
		return m, cmd

	case logLineMsg, logErrorMsg, logDoneMsg:
		var cmd tea.Cmd
		m.logs, cmd = m.logs.Update(msg)
		return m, cmd

	case containersLoadedMsg:
		var cmd tea.Cmd
		m.containerPicker, cmd = m.containerPicker.Update(msg)
		return m, cmd

	case containerSelectedMsg:
		return m.handleContainerSelected(msg)

	case fileCopyDoneMsg:
		var cmd tea.Cmd
		m.fileCopy, cmd = m.fileCopy.Update(msg)
		return m, cmd

	case containerPortsLoadedMsg, portForwardStartedMsg:
		var cmd tea.Cmd
		m.portForward, cmd = m.portForward.Update(msg)
		return m, cmd

	case resourceYAMLLoadedMsg, resourceSavedMsg:
		var cmd tea.Cmd
		m.edit, cmd = m.edit.Update(msg)
		return m, cmd

	case resourceDeletedMsg:
		if msg.err == nil {
			// Delete succeeded — go back to resource list and refresh.
			m.currentView = viewResourceList
			return m, m.resourceList.fetchResources
		}
		// Delete failed — let the confirm model display the error.
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd

	case discoveryDoneMsg:
		// Discovery complete. The picker will re-render with the full list.
		return m, nil
	}

	// Delegate to current view
	return m.updateCurrentView(msg)
}

func (m Model) View() string {
	var content string

	switch m.currentView {
	case viewResourcePicker:
		content = m.resourcePicker.View()
	case viewResourceList:
		content = m.resourceList.View()
	case viewLogs:
		content = m.logs.View()
	case viewDescribe:
		content = m.describe.View()
	case viewNamespaces:
		content = m.namespaces.View()
	case viewEdit:
		content = m.edit.View()
	case viewConfirm:
		content = m.confirm.View()
	case viewPortForward:
		content = m.portForward.View()
	case viewContainerPicker:
		content = m.containerPicker.View()
	case viewFileCopy:
		content = m.fileCopy.View()
	}

	// Status bar
	var status string
	if m.commandMode {
		status = statusBarActiveStyle.Width(m.width).Render(
			fmt.Sprintf(":%s\u2588", m.commandBuffer),
		)
	} else {
		rtName := ""
		if m.resourceList.resourceType != nil {
			rtName = " | " + m.resourceList.resourceType.Name
		}
		status = statusBarStyle.Width(m.width).Render(
			fmt.Sprintf(" %s | ns: %s%s ", m.cluster, m.displayNamespace(), rtName),
		)
	}

	return content + "\n" + status
}

func (m Model) displayNamespace() string {
	if m.namespace == "" {
		return "all"
	}
	return m.namespace
}

func (m Model) propagateSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	contentMsg := tea.WindowSizeMsg{
		Width:  msg.Width,
		Height: msg.Height - 1,
	}

	m.resourcePicker.width = contentMsg.Width
	m.resourcePicker.height = contentMsg.Height
	m.resourceList.width = contentMsg.Width
	m.resourceList.height = contentMsg.Height

	var cmd tea.Cmd
	switch m.currentView {
	case viewLogs:
		m.logs, cmd = m.logs.Update(contentMsg)
	case viewDescribe:
		m.describe, cmd = m.describe.Update(contentMsg)
	case viewNamespaces:
		m.namespaces.width = contentMsg.Width
		m.namespaces.height = contentMsg.Height
	case viewEdit:
		m.edit, cmd = m.edit.Update(contentMsg)
	case viewConfirm:
		m.confirm, cmd = m.confirm.Update(contentMsg)
	case viewPortForward:
		m.portForward, cmd = m.portForward.Update(contentMsg)
	case viewContainerPicker:
		m.containerPicker, cmd = m.containerPicker.Update(contentMsg)
	case viewFileCopy:
		m.fileCopy, cmd = m.fileCopy.Update(contentMsg)
	}
	return m, cmd
}

func (m Model) handleCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.commandMode = false
		m.commandBuffer = ""
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		m.commandMode = false
		cmd := m.commandBuffer
		m.commandBuffer = ""
		return m.executeCommand(cmd)

	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.commandBuffer) > 0 {
			m.commandBuffer = m.commandBuffer[:len(m.commandBuffer)-1]
		}
		return m, nil

	default:
		if len(msg.String()) == 1 {
			m.commandBuffer += msg.String()
		}
		return m, nil
	}
}

func (m Model) executeCommand(cmd string) (tea.Model, tea.Cmd) {
	switch cmd {
	case "q", "quit":
		return m, tea.Quit
	case "ns":
		m.currentView = viewNamespaces
		m.namespaces = newNamespaceModel(m.client, m.namespace)
		return m, m.namespaces.Init()
	default:
		if rt := LookupResourceType(cmd); rt != nil {
			return m.switchToResourceType(rt)
		}
		return m, nil
	}
}

func (m Model) switchToResourceType(rt *ResourceType) (tea.Model, tea.Cmd) {
	m.currentView = viewResourceList
	m.resourceList = newResourceListModel(m.client, rt)
	disabled := make(map[string]bool)
	if !m.portForwardEnabled {
		disabled["port-forward"] = true
	}
	if !m.fileCopyEnabled {
		disabled["copy"] = true
	}
	if len(disabled) > 0 {
		m.resourceList.disabledActions = disabled
	}
	m.resourceList.namespace = m.namespace
	m.resourceList.width = m.width
	m.resourceList.height = m.height - 1
	return m, m.resourceList.Init()
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case viewLogs:
		m.logs.cleanup()
		m.currentView = viewResourceList
		return m, nil
	case viewDescribe:
		m.currentView = viewResourceList
		return m, nil
	case viewNamespaces:
		m.currentView = viewResourceList
		return m, nil
	case viewEdit:
		m.currentView = viewResourceList
		return m, nil
	case viewConfirm:
		m.currentView = viewResourceList
		return m, nil
	case viewPortForward:
		m.portForward.cleanup()
		m.currentView = viewResourceList
		return m, nil
	case viewContainerPicker:
		m.currentView = viewResourceList
		return m, nil
	case viewFileCopy:
		m.currentView = viewResourceList
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) handleResourceAction(msg resourceActionMsg) (tea.Model, tea.Cmd) {
	switch msg.action {
	case "logs":
		m.currentView = viewContainerPicker
		m.containerPicker = newContainerPickerModel(m.client, containerPickerLogs, msg.namespace, msg.name, m.width, m.height-1)
		return m, m.containerPicker.Init()
	case "view":
		m.currentView = viewDescribe
		m.describe = newContentViewModel(m.client, msg.resourceType, msg.namespace, msg.name, m.width, m.height-1, "View", msg.resourceType.ContentFunc)
		return m, m.describe.Init()
	case "describe":
		m.currentView = viewDescribe
		m.describe = newDescribeModel(m.client, msg.resourceType, msg.namespace, msg.name, m.width, m.height-1)
		return m, m.describe.Init()
	case "edit":
		m.currentView = viewEdit
		m.edit = newEditModel(m.client, msg.resourceType, msg.namespace, msg.name, m.width, m.height-1)
		return m, m.edit.Init()
	case "delete":
		m.currentView = viewConfirm
		m.confirm = newConfirmModel(m.client, msg.resourceType, msg.namespace, msg.name, m.width, m.height-1)
		return m, nil
	case "copy":
		m.currentView = viewContainerPicker
		m.containerPicker = newContainerPickerModel(m.client, containerPickerCopy, msg.namespace, msg.name, m.width, m.height-1)
		return m, m.containerPicker.Init()
	case "port-forward":
		m.currentView = viewPortForward
		m.portForward = newPortForwardModel(m.client, msg.namespace, msg.name, m.width, m.height-1)
		return m, m.portForward.Init()
	case "exec":
		m.currentView = viewContainerPicker
		m.containerPicker = newContainerPickerModel(m.client, containerPickerExec, msg.namespace, msg.name, m.width, m.height-1)
		return m, m.containerPicker.Init()
	}
	return m, nil
}

func (m Model) handleContainerSelected(msg containerSelectedMsg) (tea.Model, tea.Cmd) {
	switch msg.action {
	case containerPickerExec:
		m.execRequest = &ExecRequest{
			Namespace: msg.namespace,
			Pod:       msg.pod,
			Container: msg.container,
		}
		return m, tea.Quit
	case containerPickerLogs:
		m.currentView = viewLogs
		m.logs = newLogModel(m.client, msg.namespace, msg.pod, msg.container, m.width, m.height-1)
		return m, m.logs.streamLogs()
	case containerPickerCopy:
		m.currentView = viewFileCopy
		m.fileCopy = newFileCopyModel(m.client, msg.namespace, msg.pod, msg.container, m.width, m.height-1)
		return m, m.fileCopy.Init()
	}
	return m, nil
}

// WantsExec returns true if the model quit in order to run an exec session.
func (m Model) WantsExec() bool {
	return m.execRequest != nil
}

// GetExecRequest returns the pending exec request, or nil.
func (m Model) GetExecRequest() *ExecRequest {
	return m.execRequest
}

// ClearExec clears the exec request so the TUI can be restarted.
func (m *Model) ClearExec() {
	m.execRequest = nil
}

// Client returns the underlying Kubernetes client.
func (m Model) Client() *Client {
	return m.client
}

func (m Model) handleNamespaceSelect() (tea.Model, tea.Cmd) {
	ns := m.namespaces.selectedNamespace()
	m.namespace = ns

	// If we had a resource type selected, refresh its list with the new namespace.
	if m.resourceList.resourceType != nil {
		return m.switchToResourceType(m.resourceList.resourceType)
	}

	// Otherwise go back to the picker.
	m.currentView = viewResourcePicker
	return m, nil
}

func (m Model) updateCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.currentView {
	case viewResourcePicker:
		m.resourcePicker, cmd = m.resourcePicker.Update(msg)
	case viewResourceList:
		m.resourceList, cmd = m.resourceList.Update(msg)
	case viewLogs:
		m.logs, cmd = m.logs.Update(msg)
	case viewDescribe:
		m.describe, cmd = m.describe.Update(msg)
	case viewEdit:
		m.edit, cmd = m.edit.Update(msg)
	case viewConfirm:
		m.confirm, cmd = m.confirm.Update(msg)
	case viewPortForward:
		m.portForward, cmd = m.portForward.Update(msg)
	case viewContainerPicker:
		m.containerPicker, cmd = m.containerPicker.Update(msg)
	case viewFileCopy:
		m.fileCopy, cmd = m.fileCopy.Update(msg)
	case viewNamespaces:
		m.namespaces, cmd = m.namespaces.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter"))) {
				return m.handleNamespaceSelect()
			}
		}
	}
	return m, cmd
}
