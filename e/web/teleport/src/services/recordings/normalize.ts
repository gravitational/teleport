import {
  CommandCategory,
  NeedsFurtherReview,
  RecordingSummaryState,
  ThreatCategory,
  type CommandAnalysis,
  type CommandSessionEvent,
  type DesktopEventDetails,
  type EnhancedSummary,
  type EnhancedSummaryResponse,
  type SessionEvent,
  type SessionRecordingSummary,
  type SessionRecordingSummaryResponse,
} from 'e-teleport/services/recordings/types';

// normalizeSessionRecordingSummary converts a raw summary response into the canonical
// SessionRecordingSummary, normalizing the enhanced summary when present.
export function normalizeSessionRecordingSummary(
  summary: SessionRecordingSummaryResponse
): SessionRecordingSummary {
  if (summary.state !== RecordingSummaryState.Success) {
    return summary;
  }

  const { enhancedSummary, ...rest } = summary;
  if (!enhancedSummary) {
    return rest;
  }

  return {
    ...rest,
    enhancedSummary: normalizeEnhancedSummary(enhancedSummary),
  };
}

// normalizeEnhancedSummary produces a canonical EnhancedSummary regardless of which proxy answered.
// Behavior:
//   - Prefers `sessionEvents` when present.
//   - Falls back to deriving SessionEvents from the deprecated `commands` array
//     (rolling-upgrade / pre-v19 proxy case).
//   - Prefers `needsFurtherReviewReasons` when present; otherwise wraps the deprecated
//     singular `needsFurtherReview` in an array.
export function normalizeEnhancedSummary(
  summary: EnhancedSummaryResponse
): EnhancedSummary {
  const sessionEvents =
    summary.sessionEvents && summary.sessionEvents.length > 0
      ? summary.sessionEvents
      : commandsToSessionEvents(summary.commands ?? []);

  let needsFurtherReviewReasons: NeedsFurtherReview[] = [];
  if (
    summary.needsFurtherReviewReasons &&
    summary.needsFurtherReviewReasons.length > 0
  ) {
    needsFurtherReviewReasons = summary.needsFurtherReviewReasons;
  } else if (summary.needsFurtherReview) {
    needsFurtherReviewReasons = [summary.needsFurtherReview];
  }

  return {
    shortDescription: summary.shortDescription,
    detailedDescription: summary.detailedDescription,
    riskLevel: summary.riskLevel,
    suspiciousActivities: summary.suspiciousActivities,
    compromiseIndicators: summary.compromiseIndicators,
    notableCommandIndexes: summary.notableCommandIndexes,
    riskScoreReasons: summary.riskScoreReasons,
    sessionEvents,
    needsFurtherReviewReasons,
  };
}

function commandsToSessionEvents(commands: CommandAnalysis[]): SessionEvent[] {
  return commands.map(c => ({
    category: c.category ?? CommandCategory.Unspecified,
    riskLevel: c.riskLevel,
    riskScore: c.riskScore,
    threatCategory: c.threatCategory ?? ThreatCategory.Unspecified,
    timelineTitle: c.timelineTitle,
    timelineSubtitle: c.timelineSubtitle,
    shortDescription: c.shortDescription,
    detailedDescription: c.detailedDescription,
    suspiciousFlags: c.suspiciousFlags,
    sensitiveItems: c.sensitiveItems,
    suspiciousPatterns: c.suspiciousPatterns,
    iocs: c.indicatorOfCompromise,
    mitreAttackIds: c.mitreAttackIds,
    hasSensitiveData: c.hasSensitiveData,
    privilegeEscalation: c.privilegeEscalation,
    dataExfiltration: c.dataExfiltration,
    persistence: c.persistence,
    startOffset: c.startOffset,
    endOffset: c.endOffset,
    inferenceErrorMessage: c.inferenceErrorMessage,
    commandEventDetails: {
      command: c.command,
      success: c.success,
      errorMessages: c.errorMessages,
    },
  }));
}

// isCommandSessionEvent narrows a SessionEvent to one carrying command details.
export function isCommandSessionEvent(
  event: SessionEvent
): event is CommandSessionEvent {
  return !!event.commandEventDetails;
}

// isDesktopEvent narrows a SessionEvent to one carrying desktop details.
export function isDesktopEvent(
  event: SessionEvent
): event is SessionEvent & { desktopEventDetails: DesktopEventDetails } {
  return !!event.desktopEventDetails;
}
