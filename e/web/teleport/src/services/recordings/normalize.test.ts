import {
  normalizeEnhancedSummary,
  normalizeSessionRecordingSummary,
} from 'e-teleport/services/recordings/normalize';
import {
  CommandCategory,
  NeedsFurtherReview,
  RecordingSummaryState,
  ThreatCategory,
  type CommandAnalysis,
  type EnhancedSummaryResponse,
  type SessionEvent,
  type SessionRecordingSummaryResponse,
} from 'e-teleport/services/recordings/types';
import { RiskLevel } from 'teleport/services/recordings';

function baseSummary(
  overrides: Partial<EnhancedSummaryResponse>
): EnhancedSummaryResponse {
  return {
    shortDescription: '',
    detailedDescription: '',
    riskLevel: RiskLevel.Low,
    suspiciousActivities: [],
    compromiseIndicators: false,
    notableCommandIndexes: [],
    ...overrides,
  };
}

const baseCommand: CommandAnalysis = {
  command: 'ls -la',
  category: CommandCategory.FileOperation,
  success: true,
  riskLevel: RiskLevel.Low,
  riskScore: 1,
  threatCategory: ThreatCategory.None,
  timelineTitle: 'Listed files',
  shortDescription: '',
  detailedDescription: '',
  errorMessages: [],
  suspiciousFlags: [],
  sensitiveItems: [],
  suspiciousPatterns: [],
  indicatorOfCompromise: [],
  hasSensitiveData: false,
  privilegeEscalation: false,
  dataExfiltration: false,
  persistence: false,
  startOffset: 0,
  endOffset: 100,
};

const baseEvent: SessionEvent = {
  category: CommandCategory.FileOperation,
  riskLevel: RiskLevel.Low,
  riskScore: 1,
  threatCategory: ThreatCategory.None,
  timelineTitle: 'Listed files',
  shortDescription: '',
  detailedDescription: '',
  hasSensitiveData: false,
  privilegeEscalation: false,
  dataExfiltration: false,
  persistence: false,
  startOffset: 0,
  endOffset: 100,
  commandEventDetails: {
    command: 'ls -la',
    success: true,
  },
};

describe('normalizeEnhancedSummary', () => {
  it('passes sessionEvents through when present', () => {
    const result = normalizeEnhancedSummary(
      baseSummary({ sessionEvents: [baseEvent] })
    );

    expect(result.sessionEvents).toEqual([baseEvent]);
  });

  it('derives sessionEvents from deprecated commands when sessionEvents missing', () => {
    const result = normalizeEnhancedSummary(
      baseSummary({ commands: [baseCommand] })
    );

    expect(result.sessionEvents).toHaveLength(1);
    expect(result.sessionEvents[0].commandEventDetails).toEqual({
      command: 'ls -la',
      success: true,
      errorMessages: [],
    });
    expect(result.sessionEvents[0].category).toBe(
      CommandCategory.FileOperation
    );
    expect(result.sessionEvents[0].timelineTitle).toBe('Listed files');
  });

  it('prefers sessionEvents over commands when both are present', () => {
    const newEvent: SessionEvent = {
      ...baseEvent,
      commandEventDetails: { command: 'from sessionEvents', success: true },
    };
    const oldCommand: CommandAnalysis = {
      ...baseCommand,
      command: 'from commands',
    };
    const result = normalizeEnhancedSummary(
      baseSummary({ sessionEvents: [newEvent], commands: [oldCommand] })
    );

    expect(result.sessionEvents).toHaveLength(1);
    expect(result.sessionEvents[0].commandEventDetails?.command).toBe(
      'from sessionEvents'
    );
  });

  it('returns empty sessionEvents when neither field is present', () => {
    const result = normalizeEnhancedSummary(baseSummary({}));
    expect(result.sessionEvents).toEqual([]);
  });

  it('passes needsFurtherReviewReasons through when present', () => {
    const result = normalizeEnhancedSummary(
      baseSummary({
        needsFurtherReviewReasons: [
          NeedsFurtherReview.TooLarge,
          NeedsFurtherReview.CommandAnalysisFailed,
        ],
      })
    );

    expect(result.needsFurtherReviewReasons).toEqual([
      NeedsFurtherReview.TooLarge,
      NeedsFurtherReview.CommandAnalysisFailed,
    ]);
  });

  it('wraps deprecated singular needsFurtherReview in an array', () => {
    const result = normalizeEnhancedSummary(
      baseSummary({ needsFurtherReview: NeedsFurtherReview.TooLarge })
    );

    expect(result.needsFurtherReviewReasons).toEqual([
      NeedsFurtherReview.TooLarge,
    ]);
  });

  it('prefers needsFurtherReviewReasons over singular needsFurtherReview', () => {
    const result = normalizeEnhancedSummary(
      baseSummary({
        needsFurtherReviewReasons: [NeedsFurtherReview.CommandAnalysisFailed],
        needsFurtherReview: NeedsFurtherReview.TooLarge,
      })
    );

    expect(result.needsFurtherReviewReasons).toEqual([
      NeedsFurtherReview.CommandAnalysisFailed,
    ]);
  });

  it('returns empty needsFurtherReviewReasons when neither field is present', () => {
    const result = normalizeEnhancedSummary(baseSummary({}));
    expect(result.needsFurtherReviewReasons).toEqual([]);
  });

  it('preserves desktop events without inventing command details', () => {
    const desktopEvent: SessionEvent = {
      ...baseEvent,
      commandEventDetails: undefined,
      desktopEventDetails: {
        applications: ['chrome'],
        activeWindowTitle: 'Gmail',
      },
    };
    const result = normalizeEnhancedSummary(
      baseSummary({ sessionEvents: [desktopEvent] })
    );

    expect(result.sessionEvents[0].desktopEventDetails).toEqual({
      applications: ['chrome'],
      activeWindowTitle: 'Gmail',
    });
    expect(result.sessionEvents[0].commandEventDetails).toBeUndefined();
  });
});

describe('normalizeSessionRecordingSummary', () => {
  it('passes pending summaries through unchanged', () => {
    const pending: SessionRecordingSummaryResponse = {
      state: RecordingSummaryState.Pending,
      inferenceStartedAt: '2025-09-01T11:59:00Z',
      sessionId: 'session-1',
    };

    expect(normalizeSessionRecordingSummary(pending)).toEqual(pending);
  });

  it('passes error summaries through unchanged', () => {
    const error: SessionRecordingSummaryResponse = {
      state: RecordingSummaryState.Error,
      errorMessage: 'failed to summarize',
      sessionId: 'session-1',
    };

    expect(normalizeSessionRecordingSummary(error)).toEqual(error);
  });

  it('passes successful summaries without an enhanced summary through unchanged', () => {
    const success: SessionRecordingSummaryResponse = {
      state: RecordingSummaryState.Success,
      content: 'summary text',
      inferenceStartedAt: '2025-09-01T11:59:00Z',
      inferenceFinishedAt: '2025-09-01T12:00:00Z',
      sessionId: 'session-1',
    };

    expect(normalizeSessionRecordingSummary(success)).toEqual(success);
  });

  it('normalizes the enhanced summary of successful summaries', () => {
    const result = normalizeSessionRecordingSummary({
      state: RecordingSummaryState.Success,
      content: 'summary text',
      inferenceStartedAt: '2025-09-01T11:59:00Z',
      inferenceFinishedAt: '2025-09-01T12:00:00Z',
      sessionId: 'session-1',
      enhancedSummary: baseSummary({
        sessionEvents: [baseEvent],
        needsFurtherReview: NeedsFurtherReview.TooLarge,
      }),
    });

    expect(result).toEqual({
      state: RecordingSummaryState.Success,
      content: 'summary text',
      inferenceStartedAt: '2025-09-01T11:59:00Z',
      inferenceFinishedAt: '2025-09-01T12:00:00Z',
      sessionId: 'session-1',
      enhancedSummary: {
        shortDescription: '',
        detailedDescription: '',
        riskLevel: RiskLevel.Low,
        suspiciousActivities: [],
        compromiseIndicators: false,
        notableCommandIndexes: [],
        sessionEvents: [baseEvent],
        needsFurtherReviewReasons: [NeedsFurtherReview.TooLarge],
      },
    });
  });
});
