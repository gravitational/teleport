/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package common

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/trace"
	"golang.org/x/term"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/defaults"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

const (
	updateInterval = 3 * time.Second
	maxWidth       = 80
)

type authRotateCommand struct {
	cmd             *kingpin.CmdClause
	interactiveMode bool
	manualMode      bool
	caType          string
	targetPhase     string
	gracePeriod     time.Duration
}

func (c *authRotateCommand) Initialize(authCmd *kingpin.CmdClause) {
	c.cmd = authCmd.Command("rotate", "Rotate certificate authorities in the cluster. Starts in interactive mode by default, provide --type to manually send rotation requests.")
	c.cmd.Flag("interactive", "Enable interactive mode").BoolVar(&c.interactiveMode)
	c.cmd.Flag("manual", "Activate manual rotation, set rotation phases manually").BoolVar(&c.manualMode)
	c.cmd.Flag("type", fmt.Sprintf("Certificate authority to rotate, one of: %s", strings.Join(getCertAuthTypes(), ", "))).EnumVar(&c.caType, getCertAuthTypes()...)
	c.cmd.Flag("phase", fmt.Sprintf("Target rotation phase to set, used in manual rotation, one of: %v", strings.Join(types.RotatePhases, ", "))).StringVar(&c.targetPhase)
	c.cmd.Flag("grace-period", "Grace period keeps previous certificate authorities signatures valid, if set to 0 will force users to re-login and nodes to re-register.").
		Default(fmt.Sprintf("%v", defaults.RotationGracePeriod)).
		DurationVar(&c.gracePeriod)
}

func (c *authRotateCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	if c.cmd.FullCommand() == cmd {
		client, clientClose, err := clientFunc(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}
		defer clientClose(ctx)

		return true, trace.Wrap(c.Run(ctx, client))
	}
	return false, nil
}

func (c *authRotateCommand) Run(ctx context.Context, client *authclient.Client) error {
	if c.interactiveMode {
		return trace.Wrap(c.runInteractive(ctx, client))
	}
	if !c.manualMode && c.caType == "" && c.targetPhase == "" && c.gracePeriod == defaults.RotationGracePeriod {
		// If the user passed zero arguments, default to interactive mode.
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return trace.BadParameter("required flag --type not provided, not starting interactive mode because stdin does not appear to be a terminal")
		}
		return trace.Wrap(c.runInteractive(ctx, client))
	}

	return trace.Wrap(c.runNoninteractive(ctx, client))
}

func (c *authRotateCommand) runNoninteractive(ctx context.Context, client *authclient.Client) error {
	if c.caType == "" {
		return trace.BadParameter("required flag --type not provided")
	}
	req := types.RotateRequest{
		Type:        types.CertAuthType(c.caType),
		TargetPhase: c.targetPhase,
		GracePeriod: &c.gracePeriod,
	}
	if c.manualMode {
		req.Mode = types.RotationModeManual
	} else {
		req.Mode = types.RotationModeAuto
	}
	if err := client.RotateCertAuthority(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	if c.targetPhase != "" {
		fmt.Printf("Updated rotation phase to %q. To check status use 'tctl status'\n", c.targetPhase)
	} else {
		fmt.Printf("Initiated certificate authority rotation. To check status use 'tctl status'\n")
	}
	return nil
}

func (c *authRotateCommand) runInteractive(ctx context.Context, client *authclient.Client) error {
	pingResp, err := client.Ping(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to ping cluster")
	}
	m := newRotateModel(client, pingResp, types.CertAuthType(c.caType))
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err = p.Run()
	return trace.Wrap(err)
}

type authRotateStyle struct {
	formTheme    *huh.Theme
	normal       lipgloss.Style
	title        lipgloss.Style
	highlight    lipgloss.Style
	errorMessage lipgloss.Style
}

var formTheme = huh.ThemeBase16()
var authRotateTheme = authRotateStyle{
	formTheme:    formTheme,
	normal:       lipgloss.NewStyle(),
	title:        formTheme.Focused.Title,
	highlight:    formTheme.Focused.SelectedOption,
	errorMessage: formTheme.Focused.ErrorMessage.SetString(""),
}

type rotateModel struct {
	client   *authclient.Client
	pingResp proto.PingResponse

	logsModel                     *writerModel
	rotateStatusModel             *rotateStatusModel
	caTypeModel                   *caTypeModel
	currentPhaseModel             *currentPhaseModel
	waitForCurrentPhaseReadyModel *waitForReadyModel
	targetPhaseModel              *targetPhaseModel
	confirmed                     bool
	sendRotateRequestModel        *sendRotateRequestModel
	mfaPromptModel                *writerModel
	waitForTargetPhaseReadyModel  *waitForReadyModel
	continueBinding               key.Binding
	newBinding                    key.Binding
	quitBinding                   key.Binding
	help                          help.Model
}

func newRotateModel(client *authclient.Client, pingResp proto.PingResponse, caType types.CertAuthType) *rotateModel {
	m := &rotateModel{
		client:            client,
		pingResp:          pingResp,
		logsModel:         newWriterModel(authRotateTheme.normal),
		rotateStatusModel: newRotateStatusModel(client, pingResp),
		caTypeModel:       newCATypeModel(caType),
		mfaPromptModel:    newWriterModel(authRotateTheme.errorMessage),
		continueBinding:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "continue rotating selected CA")),
		newBinding:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "rotate a new CA")),
		quitBinding:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		help:              help.New(),
	}
	if caType != "" {
		m.currentPhaseModel = newCurrentPhaseModel(client, pingResp, caType)
	}
	setupLoggers(m.logsModel)
	setupMFAPrompt(client, pingResp, m.mfaPromptModel)
	return m
}

// Init implements [tea.Model]. It is the first function that will be called by
// bubbletea.
func (m *rotateModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.rotateStatusModel.init(),
		m.caTypeModel.init(),
	}
	if m.currentPhaseModel != nil {
		cmds = append(cmds, m.currentPhaseModel.init())
	}
	return tea.Batch(cmds...)
}

// Update implements [tea.Model], it is called every time a message is received.
// The update method reacts to the message and updates the state of the model.
// All messages are passed to the update method of all active submodels, each model
// may optionally return a [tea.Cmd] to trigger future updates with new messages.
func (m *rotateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.quitBinding):
			return m, tea.Quit
		}
	}

	cmds = append(cmds, m.rotateStatusModel.update(msg))

	cmds = append(cmds, m.caTypeModel.update(msg))
	if m.caTypeModel.caType == "" {
		// Return early if the user hasn't picked a CA type yet.
		return m, tea.Batch(cmds...)
	}

	// Now that we have a CA type, init the current phase model if we haven't yet.
	if m.currentPhaseModel == nil {
		m.currentPhaseModel = newCurrentPhaseModel(m.client, m.pingResp, m.caTypeModel.caType)
		cmds = append(cmds, m.currentPhaseModel.init())
	}
	cmds = append(cmds, m.currentPhaseModel.update(msg))
	if m.currentPhaseModel.phase == "" {
		// Return early if we haven't got the current phase yet.
		return m, tea.Batch(cmds...)
	}

	// Now that we've got the current phase, init the waitForCurrentPhaseReady
	// model if we haven't yet and the current phase is not standby.
	if m.waitForCurrentPhaseReadyModel == nil && m.currentPhaseModel.phase != "standby" {
		m.waitForCurrentPhaseReadyModel = newWaitForReadyModel(m.client, m.currentPhaseModel.caID, m.currentPhaseModel.phase)
		cmds = append(cmds, m.waitForCurrentPhaseReadyModel.init())
	}
	if m.waitForCurrentPhaseReadyModel != nil {
		cmds = append(cmds, m.waitForCurrentPhaseReadyModel.update(msg))
		if !m.waitForCurrentPhaseReadyModel.ready() {
			// Return early if the current phase is not ready yet.
			return m, tea.Batch(cmds...)
		}
	}

	// Now that we know the current phase, init the target phase model if we haven't yet.
	if m.targetPhaseModel == nil {
		m.targetPhaseModel = newTargetPhaseModel(m.caTypeModel.caType, m.currentPhaseModel.phase)
		cmds = append(cmds, m.targetPhaseModel.init())
	}
	cmds = append(cmds, m.targetPhaseModel.update(msg))
	if m.targetPhaseModel.targetPhase == "" {
		// Return early if we haven't got the target phase yet.
		return m, tea.Batch(cmds...)
	}

	// Wait for the user to confirm the rotate request.
	if !m.confirmed {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "n", "N":
				// Go back to the beginning.
				m = newRotateModel(m.client, m.pingResp, "")
				return m, m.Init()
			case "y", "Y":
				m.confirmed = true
			default:
				return m, tea.Batch(cmds...)
			}
		default:
			return m, tea.Batch(cmds...)
		}
	}

	// Now that we got user confirmation, send the rotate request.
	if m.sendRotateRequestModel == nil {
		m.sendRotateRequestModel = newSendRotateRequestModel(m.client, m.caTypeModel.caType, m.targetPhaseModel.targetPhase)
		cmds = append(cmds, m.sendRotateRequestModel.init())
		return m, tea.Batch(cmds...)
	}
	cmds = append(cmds, m.sendRotateRequestModel.update(msg))
	if !m.sendRotateRequestModel.success {
		// Return early if the rotate request hasn't been successfully sent yet.
		return m, tea.Batch(cmds...)
	}

	// Now that we've sent the rotate request, init the waitForTargetPhaseReady model if we haven't yet.
	if m.waitForTargetPhaseReadyModel == nil {
		m.waitForTargetPhaseReadyModel = newWaitForReadyModel(m.client, m.currentPhaseModel.caID, m.targetPhaseModel.targetPhase)
		cmds = append(cmds, m.waitForTargetPhaseReadyModel.init())
	}
	cmds = append(cmds, m.waitForTargetPhaseReadyModel.update(msg))

	// If we've made it this far, let the user restart with the keybinds.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.continueBinding):
			newModel := newRotateModel(m.client, m.pingResp, m.caTypeModel.caType)
			newModel.waitForCurrentPhaseReadyModel = m.waitForTargetPhaseReadyModel
			return newModel, newModel.Init()
		case key.Matches(msg, m.newBinding):
			newModel := newRotateModel(m.client, m.pingResp, "")
			return newModel, newModel.Init()
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements [tea.Model], it renders the program's UI, which is just a
// string. The view is rendered after every Update.
func (m *rotateModel) View() string {
	var sb strings.Builder
	writeln(&sb, m.logsModel.view())
	writeln(&sb, m.rotateStatusModel.view())
	writeln(&sb, m.caTypeModel.view())
	if m.caTypeModel.caType == "" {
		return sb.String()
	}

	writeln(&sb, m.currentPhaseModel.view())
	if m.currentPhaseModel.phase == "" {
		return sb.String()
	}

	if m.waitForCurrentPhaseReadyModel != nil && !m.confirmed {
		writeln(&sb, m.waitForCurrentPhaseReadyModel.view())
		if !m.waitForCurrentPhaseReadyModel.ready() {
			return sb.String()
		}
	}

	writeln(&sb, m.targetPhaseModel.view())
	if m.targetPhaseModel.targetPhase == "" {
		return sb.String()
	}

	sb.WriteString(authRotateTheme.normal.Render("Send request to rotate "))
	sb.WriteString(authRotateTheme.highlight.Render(string(m.caTypeModel.caType)))
	sb.WriteString(authRotateTheme.normal.Render(" CA to "))
	sb.WriteString(authRotateTheme.highlight.Render(m.targetPhaseModel.targetPhase))
	sb.WriteString(authRotateTheme.normal.Render(" phase? (y/n): "))
	if !m.confirmed {
		return sb.String()
	}
	writeln(&sb, authRotateTheme.highlight.PaddingBottom(1).Render("y"))

	writeln(&sb, m.sendRotateRequestModel.view())
	if !m.sendRotateRequestModel.success {
		if mfaPrompt := m.mfaPromptModel.view(); len(mfaPrompt) > 0 {
			writeln(&sb, mfaPrompt)
		}
		return sb.String()
	}

	writeln(&sb, m.waitForTargetPhaseReadyModel.view())
	if !m.waitForTargetPhaseReadyModel.ready() {
		return sb.String()
	}

	helpBindings := []key.Binding{m.continueBinding, m.newBinding, m.quitBinding}
	if m.waitForTargetPhaseReadyModel.targetPhase == "standby" {
		helpBindings = helpBindings[1:]
	}
	writeln(&sb, authRotateTheme.normal.Render(m.help.ShortHelpView(helpBindings)))

	return sb.String()
}

type rotateStatusModel struct {
	client   *authclient.Client
	pingResp proto.PingResponse
	spinner  spinner.Model

	status *statusModel
	err    error
}

func newRotateStatusModel(client *authclient.Client, pingResp proto.PingResponse) *rotateStatusModel {
	status, err := newStatusModel(context.TODO(), client, pingResp)
	return &rotateStatusModel{
		client:   client,
		pingResp: pingResp,
		spinner: spinner.New(spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"", ".", "..", "...", "...", "...", "...", "...", "..", ".", ""},
			FPS:    time.Second / 8,
		})),
		status: status,
		err:    trace.Wrap(err),
	}
}

func (m *rotateStatusModel) updateRotateStatus(_ time.Time) tea.Msg {
	rotateStatus, err := newStatusModel(context.TODO(), m.client, m.pingResp)
	if err != nil {
		return newTaggedMsg(err, rotateStatusTag{})
	}
	return newTaggedMsg(rotateStatus, rotateStatusTag{})
}

type rotateStatusTag struct{}

func (m *rotateStatusModel) init() tea.Cmd {
	return tea.Batch(
		tea.Tick(updateInterval, m.updateRotateStatus),
		m.spinner.Tick)
}

func (m *rotateStatusModel) update(msg tea.Msg) tea.Cmd {
	msg, ok := matchTaggedMsg(msg, rotateStatusTag{})
	if !ok {
		s, msg := m.spinner.Update(msg)
		m.spinner = s
		return msg
	}
	switch msg := msg.(type) {
	case error:
		m.err = trace.Wrap(msg)
	case *statusModel:
		m.status = msg
	}
	return tea.Tick(updateInterval, m.updateRotateStatus)
}

func (m *rotateStatusModel) view() string {
	if m.err != nil {
		return authRotateTheme.errorMessage.Render("Error fetching cluster status:", m.err.Error())
	}

	var table strings.Builder
	m.status.renderText(&table, false /*debug*/)

	var sb strings.Builder
	sb.WriteString(authRotateTheme.title.Render("Current status"))
	writeln(&sb, authRotateTheme.title.Render(m.spinner.View()))
	sb.WriteString(authRotateTheme.normal.
		Render(table.String()))
	return sb.String()
}

type caTypeModel struct {
	caType types.CertAuthType
	form   *huh.Form
}

func newCATypeModel(caType types.CertAuthType) *caTypeModel {
	return &caTypeModel{
		caType: caType,
		form:   newSelectForm("Choose CA to rotate:", types.CertAuthTypes...),
	}
}

func (m *caTypeModel) init() tea.Cmd {
	if m.caType != "" {
		return nil
	}
	return m.form.Init()
}

func (m *caTypeModel) update(msg tea.Msg) tea.Cmd {
	if m.caType != "" {
		return nil
	}
	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)
	if m.form.State == huh.StateCompleted {
		m.caType = m.form.Get("selected").(types.CertAuthType)
	}
	return cmd
}

func (m *caTypeModel) view() string {
	if m.caType == "" {
		return m.form.View()
	}
	var sb strings.Builder
	sb.WriteString(authRotateTheme.normal.Render("Rotating the "))
	sb.WriteString(authRotateTheme.highlight.Render(string(m.caType)))
	sb.WriteString(authRotateTheme.normal.Render(" CA."))
	return sb.String()
}

type currentPhaseModel struct {
	client   *authclient.Client
	pingResp proto.PingResponse

	spinner spinner.Model
	caType  types.CertAuthType
	caID    types.CertAuthID
	phase   string
	err     error
}

func newCurrentPhaseModel(client *authclient.Client, pingResp proto.PingResponse, caType types.CertAuthType) *currentPhaseModel {
	return &currentPhaseModel{
		client:   client,
		pingResp: pingResp,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		caType:   caType,
	}
}

func (m *currentPhaseModel) init() tea.Cmd {
	return tea.Batch(m.getCurrentPhase, m.spinner.Tick)
}

func (m *currentPhaseModel) getCurrentPhase() tea.Msg {
	m.caID = types.CertAuthID{
		Type:       m.caType,
		DomainName: m.pingResp.ClusterName,
	}
	ca, err := m.client.GetCertAuthority(context.TODO(), m.caID, false /*loadSigningKeys*/)
	if err != nil {
		return newTaggedMsg(trace.Wrap(err, "failed to fetch CA status"), currentPhaseTag{})
	}
	return newTaggedMsg(cmp.Or(ca.GetRotation().Phase, "standby"), currentPhaseTag{})
}

type currentPhaseTag struct{}

func (m *currentPhaseModel) update(msg tea.Msg) tea.Cmd {
	if m.phase != "" {
		// Already got the current phase, no need for more updates.
		return nil
	}
	msg, ok := matchTaggedMsg(msg, currentPhaseTag{})
	if !ok {
		s, cmd := m.spinner.Update(msg)
		m.spinner = s
		return cmd
	}
	switch msg := msg.(type) {
	case string:
		m.phase = msg
	case error:
		m.err = trace.Wrap(msg)
		return tea.Quit
	}
	return nil
}

func (m *currentPhaseModel) view() string {
	if m.phase == "" {
		var sb strings.Builder
		sb.WriteString(authRotateTheme.highlight.Render(m.spinner.View()))
		sb.WriteString(authRotateTheme.normal.Render("Fetching current CA rotation phase"))
		return sb.String()
	}
	var sb strings.Builder
	sb.WriteString(authRotateTheme.normal.Render("Current rotation phase is "))
	sb.WriteString(authRotateTheme.highlight.Render(m.phase))
	sb.WriteString(authRotateTheme.normal.Render("."))
	if remaining := remainingPhases(m.phase); len(remaining) > 0 {
		sb.WriteString(authRotateTheme.normal.Render("\nRemaining phases: "))
		for len(remaining) > 1 {
			phase := remaining[0]
			remaining = remaining[1:]
			sb.WriteString(authRotateTheme.highlight.Render(phase))
			sb.WriteString(authRotateTheme.normal.Render(", "))
		}
		sb.WriteString(authRotateTheme.highlight.Render(remaining[0]))
		sb.WriteString(authRotateTheme.normal.Render("."))
	}
	return sb.String()
}

type targetPhaseModel struct {
	caType       types.CertAuthType
	currentPhase string
	targetPhase  string
	form         *huh.Form
}

func newTargetPhaseModel(caType types.CertAuthType, currentPhase string) *targetPhaseModel {
	options := nextPhases(currentPhase)
	if len(options) == 1 {
		return &targetPhaseModel{
			caType:       caType,
			currentPhase: currentPhase,
			targetPhase:  options[0],
		}
	}
	return &targetPhaseModel{
		caType:       caType,
		currentPhase: currentPhase,
		form:         newSelectForm("Select target phase:", options...),
	}
}

func (m *targetPhaseModel) init() tea.Cmd {
	if m.form == nil {
		return nil
	}
	return m.form.Init()
}

func (m *targetPhaseModel) update(msg tea.Msg) tea.Cmd {
	if m.targetPhase != "" {
		return nil
	}
	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)
	if m.form.State == huh.StateCompleted {
		m.targetPhase = m.form.GetString("selected")
	}
	return cmd
}

func (m *targetPhaseModel) view() string {
	if m.targetPhase == "" {
		return m.form.View()
	}
	var sb strings.Builder
	sb.WriteString(authRotateTheme.normal.Render("Target rotation phase is "))
	sb.WriteString(authRotateTheme.highlight.Render(m.targetPhase))
	writeln(&sb, authRotateTheme.normal.Render("."))
	sb.WriteString(authRotateTheme.normal.Width(maxWidth).
		MarginTop(1).MarginBottom(1).MarginLeft(2).
		Render(phaseHelpText(m.caType, m.currentPhase, m.targetPhase)))
	return sb.String()
}

type sendRotateRequestModel struct {
	client      *authclient.Client
	spinner     spinner.Model
	caType      types.CertAuthType
	targetPhase string
	success     bool
	err         error
}

type sendRotateRequestTag struct{}

func newSendRotateRequestModel(client *authclient.Client, caType types.CertAuthType, targetPhase string) *sendRotateRequestModel {
	return &sendRotateRequestModel{
		client:      client,
		spinner:     spinner.New(spinner.WithSpinner(spinner.Dot)),
		caType:      caType,
		targetPhase: targetPhase,
	}
}

func (m *sendRotateRequestModel) sendRotateRequest() tea.Msg {
	err := m.client.RotateCertAuthority(context.TODO(), types.RotateRequest{
		Type:        m.caType,
		TargetPhase: m.targetPhase,
		Mode:        types.RotationModeManual,
	})
	return newTaggedMsg(trace.Wrap(err), sendRotateRequestTag{})
}

func (m *sendRotateRequestModel) init() tea.Cmd {
	return tea.Batch(m.sendRotateRequest, m.spinner.Tick)
}

func (m *sendRotateRequestModel) update(msg tea.Msg) tea.Cmd {
	if m.success {
		return nil
	}
	msg, ok := matchTaggedMsg(msg, sendRotateRequestTag{})
	if !ok {
		s, cmd := m.spinner.Update(msg)
		m.spinner = s
		return cmd
	}
	switch msg := msg.(type) {
	case error:
		m.err = trace.Wrap(msg)
	}
	if m.err == nil {
		m.success = true
	}
	return nil
}

func (m *sendRotateRequestModel) view() string {
	if m.err != nil {
		return authRotateTheme.errorMessage.Render("Error sending rotate request:", m.err.Error())
	}
	if !m.success {
		var sb strings.Builder
		sb.WriteString(authRotateTheme.highlight.Render(m.spinner.View()))
		sb.WriteString(authRotateTheme.normal.Render("Sending CA rotation request"))
		return sb.String()
	}
	var sb strings.Builder
	sb.WriteString(authRotateTheme.highlight.Render("✓ "))
	switch m.targetPhase {
	case "init":
		sb.WriteString(authRotateTheme.normal.Render("Initiated certificate authority rotation."))
	default:
		sb.WriteString(authRotateTheme.normal.Render("Updated rotation phase to "))
		sb.WriteString(authRotateTheme.highlight.Render(m.targetPhase))
		sb.WriteString(authRotateTheme.normal.Render("."))
	}
	return sb.String()
}

type writerModel struct {
	style lipgloss.Style
	buf   []byte
	mu    sync.Mutex
}

func newWriterModel(style lipgloss.Style) *writerModel {
	return &writerModel{style: style}
}

func (m *writerModel) view() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.buf) == 0 {
		return ""
	}
	// This will always be printed by the caller with writeln, remove trailing
	// newlines if present.
	b := m.buf
	if b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return m.style.Render(string(b))
}

func (m *writerModel) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf = append(m.buf, b...)
	return len(b), nil
}

type waitForReadyModel struct {
	client             *authclient.Client
	targetPhase        string
	kindReadyModels    []*waitForKindReadyModel
	manualSteps        []string
	acknowledged       bool
	skipped            bool
	acknowledgeBinding key.Binding
	skipBinding        key.Binding
	quitBinding        key.Binding
	help               help.Model
}

func newWaitForReadyModel(client *authclient.Client, caID types.CertAuthID, targetPhase string) *waitForReadyModel {
	m := &waitForReadyModel{
		client:             client,
		targetPhase:        targetPhase,
		manualSteps:        manualSteps(caID.Type, targetPhase),
		acknowledgeBinding: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "acknowledge manual steps completed")),
		skipBinding:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "skip all checks (unsafe)")),
		quitBinding:        key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		help:               help.New(),
	}
	if caID.Type != types.HostCA {
		return m
	}
	m.kindReadyModels = []*waitForKindReadyModel{
		newWaitForKindReadyModel(
			targetPhase, "auth_servers", adaptServerGetter(client.GetAuthServers)).withMinReady(1),
		newWaitForKindReadyModel(
			targetPhase, "proxies", adaptServerGetter(client.GetProxies)),
		newWaitForKindReadyModel(
			targetPhase, "nodes", adaptServerGetter(func() ([]types.Server, error) {
				return apiclient.GetAllResources[types.Server](context.TODO(), client, &proto.ListResourcesRequest{
					ResourceType:        types.KindNode,
					Namespace:           apidefaults.Namespace,
					PredicateExpression: `resource.sub_kind == ""`,
				})
			})),
		newWaitForKindReadyModel(
			targetPhase, "app_servers", adaptServerGetter(func() ([]types.AppServer, error) {
				return client.GetApplicationServers(context.TODO(), apidefaults.Namespace)
			})),
		newWaitForKindReadyModel(
			targetPhase, "db_servers", adaptServerGetter(func() ([]types.DatabaseServer, error) {
				return client.GetDatabaseServers(context.TODO(), apidefaults.Namespace)
			})),
		newWaitForKindReadyModel(
			targetPhase, "kube_servers", adaptServerGetter(func() ([]types.KubeServer, error) {
				return client.GetKubernetesServers(context.TODO())
			})),
	}
	return m
}

func adaptServerGetter[T rotatable](f func() ([]T, error)) func() ([]rotatable, error) {
	return func() ([]rotatable, error) {
		servers, err := f()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out := make([]rotatable, len(servers))
		for i, server := range servers {
			out[i] = server
		}
		return out, nil
	}
}

func (m *waitForReadyModel) ready() bool {
	if m.skipped {
		return true
	}
	if len(m.manualSteps) > 0 && !m.acknowledged {
		return false
	}
	for _, kindReadyModel := range m.kindReadyModels {
		if !kindReadyModel.ready() {
			return false
		}
	}
	return true
}

func (m *waitForReadyModel) init() tea.Cmd {
	var cmds []tea.Cmd
	for _, kindReadyModel := range m.kindReadyModels {
		cmds = append(cmds, kindReadyModel.init())
	}
	return tea.Batch(cmds...)
}

func (m *waitForReadyModel) update(msg tea.Msg) tea.Cmd {
	if m.ready() {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.acknowledgeBinding):
			m.acknowledged = true
		case key.Matches(msg, m.skipBinding):
			m.skipped = true
			for _, kindReadyModel := range m.kindReadyModels {
				kindReadyModel.skipped = true
			}
		}
	}
	var cmds []tea.Cmd
	for i := range m.kindReadyModels {
		if m.kindReadyModels[i].ready() {
			continue
		}
		cmds = append(cmds, m.kindReadyModels[i].update(msg))
	}
	return tea.Batch(cmds...)
}

func (m *waitForReadyModel) view() string {
	var sb strings.Builder
	for _, kindReadyModel := range m.kindReadyModels {
		writeln(&sb, kindReadyModel.view())
	}
	manualStepPrefix := authRotateTheme.errorMessage.Render("! ")
	if m.acknowledged {
		manualStepPrefix = authRotateTheme.highlight.Render("✓ ")
	}
	for _, manualStep := range m.manualSteps {
		writeln(&sb, lipgloss.JoinHorizontal(0,
			manualStepPrefix,
			authRotateTheme.normal.Width(maxWidth-2).Render(manualStep),
		))
	}
	if !m.ready() {
		helpKeys := []key.Binding{m.acknowledgeBinding, m.skipBinding, m.quitBinding}
		if m.acknowledged {
			helpKeys = helpKeys[1:]
		}
		writeln(&sb, authRotateTheme.normal.PaddingTop(1).Render(
			m.help.ShortHelpView(helpKeys),
		))
	}
	return sb.String()
}

type readyStatus struct {
	totalCount, readyCount int
}

type waitForKindReadyModel struct {
	targetPhase      string
	desc             string
	getter           func() ([]rotatable, error)
	minReady         int
	spinner          spinner.Model
	readyStatus      readyStatus
	err              error
	gotFirstResponse bool
	skipped          bool
}

type rotatable interface {
	GetRotation() types.Rotation
}

func newWaitForKindReadyModel(targetPhase string, desc string, getter func() ([]rotatable, error)) *waitForKindReadyModel {
	return &waitForKindReadyModel{
		targetPhase: targetPhase,
		desc:        desc,
		getter:      getter,
		spinner:     spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

func (m *waitForKindReadyModel) withMinReady(n int) *waitForKindReadyModel {
	m.minReady = n
	return m
}

func (m *waitForKindReadyModel) getKindServersStatus() tea.Msg {
	servers, err := m.getter()
	if err != nil {
		return newTaggedMsg(trace.Wrap(err), m.desc)
	}
	ready := 0
	for _, server := range servers {
		phase := server.GetRotation().Phase
		if phase == m.targetPhase || m.targetPhase == "standby" && phase == "" {
			ready++
		}
	}
	return newTaggedMsg(readyStatus{totalCount: len(servers), readyCount: ready}, m.desc)
}

func (m *waitForKindReadyModel) ready() bool {
	return m.gotFirstResponse &&
		m.readyStatus.readyCount >= m.minReady &&
		m.readyStatus.readyCount == m.readyStatus.totalCount
}

func (m *waitForKindReadyModel) init() tea.Cmd {
	return tea.Batch(m.getKindServersStatus, m.spinner.Tick)
}

func (m *waitForKindReadyModel) update(msg tea.Msg) tea.Cmd {
	msg, ok := matchTaggedMsg(msg, m.desc)
	if !ok {
		s, cmd := m.spinner.Update(msg)
		m.spinner = s
		return cmd
	}
	switch msg := msg.(type) {
	case error:
		m.err = trace.Wrap(msg)
		return tea.Tick(updateInterval, func(time.Time) tea.Msg { return m.getKindServersStatus() })
	case readyStatus:
		m.gotFirstResponse = true
		m.err = nil
		m.readyStatus = msg
		if m.ready() {
			return nil
		}
		return tea.Tick(updateInterval, func(time.Time) tea.Msg { return m.getKindServersStatus() })
	}
	return nil
}

func (m *waitForKindReadyModel) view() string {
	if m.err != nil {
		var sb strings.Builder
		sb.WriteString(authRotateTheme.errorMessage.Render("x "))
		sb.WriteString(authRotateTheme.normal.Render("Error fetching "))
		sb.WriteString(authRotateTheme.highlight.Render(m.desc))
		sb.WriteString(authRotateTheme.normal.Render(" status: "))
		sb.WriteString(authRotateTheme.errorMessage.Render(m.err.Error()))
		return sb.String()
	}
	if m.ready() {
		var sb strings.Builder
		sb.WriteString(authRotateTheme.highlight.Render("✓ "))
		if m.readyStatus.totalCount == 0 {
			sb.WriteString(authRotateTheme.normal.Render("No "))
			sb.WriteString(authRotateTheme.highlight.Render(m.desc))
			sb.WriteString(authRotateTheme.normal.Render(" found."))
			return sb.String()
		}
		sb.WriteString(authRotateTheme.normal.Render("All "))
		sb.WriteString(authRotateTheme.highlight.Render(m.desc))
		sb.WriteString(authRotateTheme.normal.Render(" are in the "))
		sb.WriteString(authRotateTheme.highlight.Render(m.targetPhase))
		sb.WriteString(authRotateTheme.normal.Render(
			fmt.Sprintf(" phase (%d/%d).", m.readyStatus.readyCount, m.readyStatus.totalCount)))
		return sb.String()
	}
	var sb strings.Builder
	if m.skipped {
		sb.WriteString(authRotateTheme.errorMessage.Render("! "))
	} else {
		sb.WriteString(authRotateTheme.highlight.Render(m.spinner.View()))
	}
	if m.gotFirstResponse {
		if m.skipped {
			sb.WriteString(authRotateTheme.normal.Render("Skipped waiting for "))
		} else {
			sb.WriteString(authRotateTheme.normal.Render("Waiting for "))
		}
		sb.WriteString(authRotateTheme.highlight.Render(m.desc))
		sb.WriteString(authRotateTheme.normal.Render(" to enter "))
		sb.WriteString(authRotateTheme.highlight.Render(m.targetPhase))
		sb.WriteString(authRotateTheme.normal.Render(fmt.Sprintf(" phase (%d/%d). ",
			m.readyStatus.readyCount, m.readyStatus.totalCount)))
	} else {
		if m.skipped {
			sb.WriteString(authRotateTheme.normal.Render("Skipped checking current rotation phase of "))
		} else {
			sb.WriteString(authRotateTheme.normal.Render("Checking current rotation phase of "))
		}
		sb.WriteString(authRotateTheme.highlight.Render(m.desc))
		sb.WriteString(authRotateTheme.normal.Render(". "))
	}
	sb.WriteString(authRotateTheme.normal.Render(fmt.Sprintf("Run 'tctl get %s' to check status.", m.desc)))
	return sb.String()

}

type taggedMsg[T comparable] struct {
	msg tea.Msg
	tag T
}

func newTaggedMsg[T comparable](msg tea.Msg, tag T) taggedMsg[T] {
	return taggedMsg[T]{
		msg: msg,
		tag: tag,
	}
}

func matchTaggedMsg[T comparable](msg tea.Msg, tag T) (tea.Msg, bool) {
	if msg, ok := msg.(taggedMsg[T]); ok && msg.tag == tag {
		return msg.msg, true
	}
	return msg, false
}

func phaseHelpText(caType types.CertAuthType, currentPhase, targetPhase string) string {
	var sb strings.Builder
	switch targetPhase {
	case "init":
		initPhaseHelpText(&sb, caType)
	case "update_clients":
		updateClientsPhaseHelpText(&sb, caType)
	case "update_servers":
		updateServersPhaseHelpText(&sb, caType)
	case "rollback":
		rollbackPhaseHelpText(&sb)
	case "standby":
		standbyPhaseHelpText(&sb, caType, currentPhase)
	}
	return sb.String()
}

func initPhaseHelpText(sb *strings.Builder, caType types.CertAuthType) {
	sb.WriteString("The init phase initiates a new Certificate Authority (CA) rotation. ")
	sb.WriteString("New CA key pairs and certificates will be generated and must be trusted but will not yet be used.")
	switch caType {
	case types.HostCA:
		sb.WriteString("\nDuring this phase all Teleport services will automatically begin to trust the new SSH host key and X509 CA certificate.")
	}
}

func updateClientsPhaseHelpText(sb *strings.Builder, caType types.CertAuthType) {
	sb.WriteString("In the update_clients phase the new CA keys become the active signing keys for all new certificates issued by the CA. ")
	sb.WriteString("Clients will immediately begin to use their new certificates, but servers will continue to use their original certificates.")
	switch caType {
	case types.HostCA:
		sb.WriteString("\nDuring this phase, all Teleport services will automatically retrieve new certificates issued by the new CA.")
	case types.OpenSSHCA:
		sb.WriteString("\nAll new connections to OpenSSH hosts will begin to use certificates issued by the new CA keys.")
	case types.UserCA:
		sb.WriteString("\nAll new connections to Windows desktops will begin to use certificates issued by the new CA certificate. ")
	case types.DatabaseClientCA:
		sb.WriteString("\nAll new database connections will begin to use certificates issued by the new CA certificate.")
	default:
		sb.WriteString("\nAll client certificates issued by this CA must be re-issued before proceeding to the update_servers phase.")
	}
}

func updateServersPhaseHelpText(sb *strings.Builder, caType types.CertAuthType) {
	sb.WriteString("In the update_servers phase servers will begin to use certificates issued by the new CA.")
}

func rollbackPhaseHelpText(sb *strings.Builder) {
	sb.WriteString("In the rollback phase the original CA keys become the active signing keys for all new certificates issued by the CA. ")
	sb.WriteString("The new CA certificates/keys remain trusted until proceeding to the standby phase.")
}

func standbyPhaseHelpText(sb *strings.Builder, caType types.CertAuthType, previousPhase string) {
	sb.WriteString("The standby phase completes the ")
	switch previousPhase {
	case "rollback":
		sb.WriteString("rollback")
	default:
		sb.WriteString("rotation")
	}
	sb.WriteByte('.')

	switch caType {
	case types.HostCA:
		sb.WriteString("\nAfter entering the standby phase all Teleport Services will stop trusting the ")
		switch previousPhase {
		case "rollback":
			sb.WriteString("new CA and exclusively trust the original CA")
		default:
			sb.WriteString("old CA")
		}
		sb.WriteString(" X509 certificate and SSH key.")
	}
}

func manualSteps(caType types.CertAuthType, phase string) []string {
	const trustedClusterStep = "Wait up to 30 minutes for any root or leaf clusters to follow the rotation."
	const remoteReloginStep = "If you are currently using tctl remotely and logged in with tsh, you must log out and log back in."
	const offlineNodesStep = "If any Teleport services may currently be offline, wait for them to come online and follow the rotation."
	switch caType {
	case types.HostCA:
		switch phase {
		case "init":
			return []string{offlineNodesStep, trustedClusterStep}
		case "update_clients":
			return []string{offlineNodesStep, trustedClusterStep, remoteReloginStep}
		case "update_servers":
			return []string{
				"Any OpenSSH hosts must be issued new host certificates signed by the new CA.",
				offlineNodesStep,
				trustedClusterStep,
			}
		case "rollback":
			return []string{
				"Any OpenSSH host certificates reissued during the rotation must be reissued again to revert to the original issuing CA.",
				offlineNodesStep,
				trustedClusterStep,
			}
		case "standby":
			return []string{offlineNodesStep, trustedClusterStep}
		}
	case types.OpenSSHCA:
		switch phase {
		case "init":
			return []string{
				"Any OpenSSH hosts must be updated to trust both the new and old CA keys.",
				trustedClusterStep,
			}
		case "update_clients":
			return []string{trustedClusterStep}
		case "update_servers":
			return []string{trustedClusterStep}
		case "rollback":
			return []string{
				"Any OpenSSH hosts updated to trust the new CA keys during the update_servers phase should be reverted to only trust the original CA keys.",
				trustedClusterStep,
			}
		case "standby":
			return []string{
				"Any OpenSSH hosts should be updated to stop trusting the CA keys that have now been rotated out.",
				trustedClusterStep,
			}
		}
	case types.UserCA:
		switch phase {
		case "init":
			return []string{
				"All Windows desktops must be updated to trust both the new and old CA certificates.",
				trustedClusterStep,
			}
		case "update_clients":
			return []string{trustedClusterStep}
		case "update_servers":
			return []string{
				"Wait up to 30 hours for all user sessions to expire, or else users may have to log out and log back in.",
				trustedClusterStep,
				remoteReloginStep,
			}
		case "rollback":
			return []string{
				"Any Windows desktops updated to trust the new CA certificate during the update_servers phase should be reverted to only trust the original CA certificate.",
				trustedClusterStep,
			}
		case "standby":
			return []string{
				"All Windows desktops should be updated to stop trusting the CA certificates that have now been rotated out.",
				trustedClusterStep,
			}
		}
	case types.DatabaseCA:
		switch phase {
		case "init":
			return []string{
				"If you also need to rotate the db_client CA, rotate it to the init phase now to reconfigure self-hosted databases with new server certificates and trusted client CAs simultaneously.",
				"All self-hosted databases must be issued new certificates signed by the new CA.",
			}
		case "rollback":
			return []string{"Any self-hosted database certificates reissued during the rotation must be reissued again to revert to the original issuing CA."}
		}
	case types.DatabaseClientCA:
		switch phase {
		case "init":
			return []string{
				"If you also need to rotate the db_client CA, rotate it to the init phase now to reconfigure self-hosted databases with new server certificates and trusted client CAs simultaneously.",
				"All self-hosted databases must be updated to trust both the new and old CA certificates.",
			}
		case "standby":
			return []string{"All self-hosted databases should be updated to stop trusting the CA certificates that have now been rotated out."}
		}
	case types.SAMLIDPCA:
		switch phase {
		case "update_clients":
			return []string{"Any service providers that rely on the SAML IdP must by updated to trust the new CA, follow the SAML IdP guide: https://goteleport.com/docs/admin-guides/access-controls/idps/saml-guide/"}
		case "rollback":
			return []string{"Any service provider configuration changes made during the rotation must be reverted."}
		}
	case types.OIDCIdPCA:
		// No manual steps required.
		return nil
	case types.SPIFFECA:
		// TODO(strideynet): populate any known manual steps during SPIFFE CA rotation.
		fallthrough
	case types.OktaCA:
		// TODO(smallinsky): populate any known manual steps during Okta CA rotation.
		fallthrough
	case types.AWSRACA:
		// TODO(marco): populate any known manual steps during AWS IAM Roles Anywhere CA rotation.
		fallthrough
	case types.BoundKeypairCA:
		// TODO(timothyb89): add any manual steps; this should mostly be handled automatically.
		fallthrough
	default:
		return []string{"Consult the CA rotation docs for any manual steps that may be required: https://goteleport.com/docs/admin-guides/management/operations/ca-rotation/"}
	}
	return nil
}

func nextPhases(currentPhase string) []string {
	switch currentPhase {
	case "standby":
		return []string{"init"}
	case "init":
		return []string{"update_clients", "rollback"}
	case "update_clients":
		return []string{"update_servers", "rollback"}
	case "update_servers":
		return []string{"standby", "rollback"}
	case "rollback":
		return []string{"standby"}
	}
	return nil
}

var (
	optimisticPhases = [...]string{"init", "update_clients", "update_servers", "standby"}
)

func remainingPhases(afterPhase string) []string {
	switch afterPhase {
	case "standby":
		return optimisticPhases[:]
	case "init":
		return optimisticPhases[1:]
	case "update_clients":
		return optimisticPhases[2:]
	case "update_servers":
		return optimisticPhases[3:]
	case "rollback":
		return []string{"standby"}
	}
	return nil
}

func writeln(sb *strings.Builder, s string) {
	sb.WriteString(s)
	sb.WriteByte('\n')
}

func setupLoggers(logWriter io.Writer) {
	slog.SetDefault(slog.New(logutils.NewSlogTextHandler(
		logWriter,
		logutils.SlogTextHandlerConfig{EnableColors: true},
	)))
}

func setupMFAPrompt(client *authclient.Client, pingResp proto.PingResponse, promptWriter io.Writer) {
	client.SetMFAPromptConstructor(func(opts ...mfa.PromptOpt) mfa.Prompt {
		promptCfg := libmfa.NewPromptConfig(pingResp.ProxyPublicAddr, opts...)
		return libmfa.NewCLIPrompt(&libmfa.CLIPromptConfig{
			PromptConfig: *promptCfg,
			Writer:       promptWriter,
			StdinFunc: func() prompt.StdinReader {
				return brokenStdinReader{}
			},
		})
	})
}

var errNoStdin = fmt.Errorf("interactive CA rotation does not support reading passwords from stdin")

// brokenStdinReader implements [prompt.StdinReader] and returns errNoStdin for
// all methods. Currently this should be unnecessary because MFA for admin
// actions only applies when the only MFA method is webauthn, which should never
// prompt for a password. If we ever enable MFA for admin actions with OTP,
// we'll hit this error instead of bubbletea competing for stdin with the
// password prompt.
type brokenStdinReader struct{}

func (brokenStdinReader) IsTerminal() bool                               { return true }
func (brokenStdinReader) ReadContext(_ context.Context) ([]byte, error)  { return nil, errNoStdin }
func (brokenStdinReader) ReadPassword(_ context.Context) ([]byte, error) { return nil, errNoStdin }

func newSelectForm[T comparable](title string, options ...T) *huh.Form {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Quit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	selectField := &fieldWithKeyBinds{
		Field: huh.NewSelect[T]().
			Key("selected").
			Options(huh.NewOptions(options...)...).
			Title(title),
		keyBinds: []key.Binding{
			keyMap.Select.Up,
			keyMap.Select.Down,
			keyMap.Select.Submit,
			keyMap.Quit,
		},
	}
	return huh.NewForm(
		huh.NewGroup(selectField).WithKeyMap(keyMap),
	).WithTheme(authRotateTheme.formTheme)
}

type fieldWithKeyBinds struct {
	huh.Field
	keyBinds []key.Binding
}

func (f *fieldWithKeyBinds) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	field, cmd := f.Field.Update(msg)
	f.Field = field.(huh.Field)
	return f, cmd
}

func (f *fieldWithKeyBinds) KeyBinds() []key.Binding {
	return f.keyBinds
}
