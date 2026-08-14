package summarizer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/ttyterminal"
	"github.com/gravitational/teleport/lib/session"
)

func TestAnalyseSessionCommands_NoCommands(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command)
	close(commands)

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-empty")
	sessionAnalysis, commandAnalyses, err := analyzeSessionCommands(ctx, &provider, pool, commands, &details)
	require.ErrorContains(t, err, "no commands to analyze")

	require.Nil(t, sessionAnalysis)
	require.Nil(t, commandAnalyses)
}

func TestAnalyseSessionCommands_SingleCommand(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command, 1)
	commands <- &mockCommand{[]string{"ls -la"}}
	close(commands)

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-single")
	sessionAnalysis, commandAnalyses, err := analyzeSessionCommands(ctx, &provider, pool, commands, &details)
	require.NoError(t, err)

	require.NotNil(t, sessionAnalysis)
	require.Equal(t, "Chunk synthesis", sessionAnalysis.ShortDescription)
	require.Equal(t, "low", sessionAnalysis.RiskLevel)
	require.Equal(t, 15, sessionAnalysis.RiskScore)

	require.Len(t, commandAnalyses, 1)
	require.Equal(t, "ls -la", commandAnalyses[0].Command)
}

func TestAnalyseSessionCommands_MultipleCommands(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command, 3)
	commands <- &mockCommand{[]string{"ls -la"}}
	commands <- &mockCommand{[]string{"cat /etc/passwd"}}
	commands <- &mockCommand{[]string{"exit"}}
	close(commands)

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-multiple")
	sessionAnalysis, commandAnalyses, err := analyzeSessionCommands(ctx, &provider, pool, commands, &details)
	require.NoError(t, err)

	require.NotNil(t, sessionAnalysis)
	require.Len(t, commandAnalyses, 3)
}

func TestAnalyseSessionCommands_ErrorFromProvider(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command, 1)
	commands <- &mockCommand{[]string{"error command"}}
	close(commands)

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-error")
	sessionAnalysis, commandAnalyses, err := analyzeSessionCommands(ctx, &provider, pool, commands, &details)
	require.NoError(t, err)

	require.NotNil(t, sessionAnalysis)
	require.True(t, sessionAnalysis.CommandAnalysisFailed)

	require.Len(t, commandAnalyses, 1)
	require.Equal(t, "error command", commandAnalyses[0].Command)
	require.Contains(t, commandAnalyses[0].ShortDescription, "Command analysis failed")
	require.Equal(t, "high", commandAnalyses[0].RiskLevel)
}

func TestAnalyseSessionCommands_PartialFailure(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command, 3)
	commands <- &mockCommand{[]string{"ls -la"}}
	commands <- &mockCommand{[]string{"error command"}}
	commands <- &mockCommand{[]string{"exit"}}
	close(commands)

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-partial-failure")
	sessionAnalysis, commandAnalyses, err := analyzeSessionCommands(ctx, &provider, pool, commands, &details)
	require.NoError(t, err)

	require.NotNil(t, sessionAnalysis)
	require.True(t, sessionAnalysis.CommandAnalysisFailed)

	require.Len(t, commandAnalyses, 3)

	// First command succeeded
	require.Equal(t, "ls -la", commandAnalyses[0].Command)
	require.Equal(t, "Command executed", commandAnalyses[0].ShortDescription)

	// Second command failed gracefully
	require.Equal(t, "error command", commandAnalyses[1].Command)
	require.Contains(t, commandAnalyses[1].ShortDescription, "Command analysis failed")
	require.Equal(t, "high", commandAnalyses[1].RiskLevel)
	require.Equal(t, "other", commandAnalyses[1].Category)
	require.Equal(t, "command analysis error", commandAnalyses[1].InferenceErrorMessage)

	// Third command succeeded
	require.Equal(t, "exit", commandAnalyses[2].Command)
	require.Equal(t, "Command executed", commandAnalyses[2].ShortDescription)
}

func TestAnalyseSessionCommands_FailedCommandBumpsRiskToMedium(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command, 1)
	commands <- &mockCommand{[]string{"error command"}}
	close(commands)

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-risk-bump")
	sessionAnalysis, _, err := analyzeSessionCommands(ctx, &provider, pool, commands, &details)
	require.NoError(t, err)

	require.NotNil(t, sessionAnalysis)
	require.True(t, sessionAnalysis.CommandAnalysisFailed)

	// The LLM returned "low" for the session synthesis, but it should be bumped to at least "medium" because
	// a command failed to be analyzed.
	require.Equal(t, "medium", sessionAnalysis.RiskLevel)
}

func TestAnalyseSessionCommands_FailedCommandKeepsHigherRisk(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command, 1)
	commands <- &mockCommand{[]string{"error command"}}
	close(commands)

	provider := &mockSessionProviderWithRisk{sessionRiskLevel: "high", sessionRiskScore: 75}

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-risk-keep-high")
	sessionAnalysis, _, err := analyzeSessionCommands(ctx, provider, pool, commands, &details)
	require.NoError(t, err)

	require.NotNil(t, sessionAnalysis)
	require.True(t, sessionAnalysis.CommandAnalysisFailed)

	// The LLM returned "high" which is above "medium", so it should stay.
	require.Equal(t, "high", sessionAnalysis.RiskLevel)
}

func TestAnalyseSessionCommands_PartialFailureBumpsRiskToMedium(t *testing.T) {
	ctx := t.Context()

	commands := make(chan ttyterminal.Command, 2)
	commands <- &mockCommand{[]string{"ls -la"}}
	commands <- &mockCommand{[]string{"error command"}}
	close(commands)

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	details := createSessionDetails("test-session-partial-risk-bump")
	sessionAnalysis, commandAnalyses, err := analyzeSessionCommands(ctx, &provider, pool, commands, &details)
	require.NoError(t, err)

	require.NotNil(t, sessionAnalysis)
	require.True(t, sessionAnalysis.CommandAnalysisFailed)
	require.Len(t, commandAnalyses, 2)

	// The LLM returned "low" for the session synthesis, but it should be bumped to at least "medium" because a
	// command failed to analyze.
	require.Equal(t, "medium", sessionAnalysis.RiskLevel)
}

func TestSummarizeChunksInParallel(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-parallel")

	chunks := []*commandChunk{
		{
			entries: []*commandEntry{
				{command: "cmd1", shortDescription: "First command"},
			},
		},
		{
			entries: []*commandEntry{
				{command: "cmd2", shortDescription: "Second command"},
			},
		},
	}

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	s := &sessionAnalyzer{
		pool:      pool,
		provider:  &provider,
		sessionID: sessionID,
		username:  "testuser",
		loginName: "ubuntu",
	}

	results := s.summarizeChunksInParallel(ctx, chunks)

	require.Len(t, results, 2)
	for _, result := range results {
		require.NotNil(t, result)
		require.NoError(t, result.error)
		require.Equal(t, "Chunk synthesis", result.analysis.ShortDescription)
	}
}

func TestSummarizeChunksInParallel_Error(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-parallel-error")

	chunks := []*commandChunk{
		{
			entries: []*commandEntry{
				{command: "error synthesis", shortDescription: "Should fail"},
			},
		},
	}

	var provider mockSessionInferenceProvider

	pool := newWorkerPool(5)

	s := &sessionAnalyzer{
		pool:      pool,
		provider:  &provider,
		sessionID: sessionID,
		username:  "testuser",
		loginName: "ubuntu",
	}

	results := s.summarizeChunksInParallel(ctx, chunks)
	for _, result := range results {
		require.ErrorContains(t, result.error, "synthesis error")
		require.Nil(t, result.analysis)
	}
}

func TestSynthesizeChunk(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-synthesize")

	chunk := &commandChunk{
		entries: []*commandEntry{
			{command: "ls", shortDescription: "List files"},
			{command: "pwd", shortDescription: "Print working directory"},
		},
	}

	var provider mockSessionInferenceProvider

	s := &sessionAnalyzer{
		provider:  &provider,
		sessionID: sessionID,
		username:  "testuser",
		loginName: "ubuntu",
	}

	result, err := s.synthesizeChunk(ctx, chunk)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Equal(t, "Chunk synthesis", result.ShortDescription)
}

func TestSynthesizeChunkSummaries(t *testing.T) {
	ctx := t.Context()

	sessionID := session.ID("test-session-final-synthesis")

	chunkSummaries := []*summarizedSessionResult{
		{analysis: &schema.SessionAnalysis{ShortDescription: "First chunk", RiskLevel: "low", RiskScore: 10}},
		{analysis: &schema.SessionAnalysis{ShortDescription: "Second chunk", RiskLevel: "medium", RiskScore: 40}},
	}

	var provider mockSessionInferenceProvider

	s := &sessionAnalyzer{
		provider:  &provider,
		sessionID: sessionID,
		username:  "testuser",
		loginName: "ubuntu",
	}

	result, err := s.synthesizeChunkSummaries(ctx, chunkSummaries)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Equal(t, "Final synthesis", result.ShortDescription)
}

// mockSessionProviderWithRisk embeds the base mock but overrides the session
// synthesis to return a configurable risk level.
type mockSessionProviderWithRisk struct {
	mockSessionInferenceProvider
	sessionRiskLevel string
	sessionRiskScore int
}

func (m *mockSessionProviderWithRisk) SummarizeMultipleCommands(
	_ context.Context,
	_ session.ID,
	_, _, _ string,
) (*schema.SessionAnalysis, error) {
	return &schema.SessionAnalysis{
		ShortDescription: "Session analysis",
		RiskLevel:        m.sessionRiskLevel,
		RiskScore:        m.sessionRiskScore,
	}, nil
}

type mockSessionInferenceProvider struct{}

func (m *mockSessionInferenceProvider) SummarizeCommand(
	ctx context.Context,
	sessionID session.ID,
	username,
	loginName,
	prompt string,
) (*schema.CommandAnalysis, error) {
	if strings.HasSuffix(prompt, "error command") {
		return nil, errors.New("command analysis error")
	}

	cmd := "unknown"
	if strings.HasSuffix(prompt, "ls -la") {
		cmd = "ls -la"
	} else if strings.HasSuffix(prompt, "cat /etc/passwd") {
		cmd = "cat /etc/passwd"
	} else if strings.HasSuffix(prompt, "exit") {
		cmd = "exit"
	}

	return &schema.CommandAnalysis{
		Command:          cmd,
		ShortDescription: "Command executed",
		RiskLevel:        "low",
		RiskScore:        10,
		Success:          true,
	}, nil
}

func (m *mockSessionInferenceProvider) SummarizeMultipleCommands(
	ctx context.Context,
	sessionID session.ID,
	username,
	loginName,
	prompt string,
) (*schema.SessionAnalysis, error) {
	if strings.Contains(prompt, "error synthesis") {
		return nil, errors.New("synthesis error")
	}

	if strings.Contains(prompt, "Synthesize the following chunk summaries") {
		return &schema.SessionAnalysis{
			ShortDescription: "Final synthesis",
			RiskLevel:        "medium",
			RiskScore:        35,
		}, nil
	}

	if strings.Contains(prompt, "Synthesize the following command executions") {
		return &schema.SessionAnalysis{
			ShortDescription: "Chunk synthesis",
			RiskLevel:        "low",
			RiskScore:        15,
		}, nil
	}

	return &schema.SessionAnalysis{
		ShortDescription: "Session analysis",
		RiskLevel:        "low",
		RiskScore:        10,
	}, nil
}

func createSessionDetails(sessionID session.ID) sessionDetails {
	return sessionDetails{
		sessionID: sessionID,
		username:  "testuser",
		loginName: "ubuntu",
	}
}
