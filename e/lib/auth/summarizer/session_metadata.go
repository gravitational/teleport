package summarizer

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
)

const sessionMetadataHeader = "SESSION METADATA:\n"

// buildSessionMetadata extracts server/cluster metadata from the session end event and formats it as a string prefix for command prompts.
func buildSessionMetadata(sessionEnd apievents.AuditEvent, kind types.SessionKind, now time.Time) string {
	end, ok := sessionEnd.(*apievents.SessionEnd)
	if !ok || end == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(sessionMetadataHeader)
	sb.WriteString(sessionTimesFromEvent(sessionEnd, now))

	switch kind {
	case types.SSHSessionKind:
		if end.ServerHostname != "" {
			fmt.Fprintf(&sb, "Server hostname: %s\n", end.ServerHostname)
		}
		if len(end.ServerLabels) > 0 {
			fmt.Fprintf(&sb, "Server labels: %s\n", formatLabels(end.ServerLabels))
		}

	case types.KubernetesSessionKind:
		if end.KubernetesCluster != "" {
			fmt.Fprintf(&sb, "Kubernetes cluster: %s\n", end.KubernetesCluster)
		}
		if len(end.KubernetesLabels) > 0 {
			fmt.Fprintf(&sb, "Kubernetes labels: %s\n", formatLabels(end.KubernetesLabels))
		}
		if end.KubernetesPodName != "" {
			fmt.Fprintf(&sb, "Pod name: %s\n", end.KubernetesPodName)
		}
		if end.KubernetesPodNamespace != "" {
			fmt.Fprintf(&sb, "Pod namespace: %s\n", end.KubernetesPodNamespace)
		}
	}

	sb.WriteString("\n")

	return sb.String()
}

func sessionTimesFromEvent(sessionEnd apievents.AuditEvent, now time.Time) string {
	var startTime, endTime time.Time

	switch end := sessionEnd.(type) {
	case *apievents.SessionEnd:
		if end == nil {
			return ""
		}
		startTime = end.StartTime
		endTime = end.EndTime
	case *apievents.DatabaseSessionEnd:
		if end == nil {
			return ""
		}
		startTime = end.StartTime
		endTime = end.EndTime
	default:
		slog.ErrorContext(context.Background(), "unexpected event type for session metadata", "type", fmt.Sprintf("%T", sessionEnd))
		return ""
	}

	if startTime.IsZero() || endTime.IsZero() {
		return fmt.Sprintf("Current date and time: %s\n", now.UTC().Format(time.RFC3339))
	}

	return fmt.Sprintf(
		"Session start time: %s\nSession end time: %s\nSession duration: %s\nCurrent date and time: %s\n",
		startTime.UTC().Format(time.RFC3339),
		endTime.UTC().Format(time.RFC3339),
		endTime.Sub(startTime).Truncate(time.Second),
		now.UTC().Format(time.RFC3339),
	)
}

func formatLabels(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for _, k := range slices.Sorted(maps.Keys(labels)) {
		parts = append(parts, k+"="+labels[k])
	}

	return strings.Join(parts, ", ")
}
