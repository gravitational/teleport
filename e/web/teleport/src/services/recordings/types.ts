import { RiskLevel } from 'teleport/services/recordings';

export enum RecordingSummaryState {
  Pending = 'SUMMARY_STATE_PENDING',
  Success = 'SUMMARY_STATE_SUCCESS',
  Error = 'SUMMARY_STATE_ERROR',
}

interface BaseRecordingSummary {
  sessionId: string;
}

interface RecordingSummaryPending extends BaseRecordingSummary {
  inferenceStartedAt: string;
  state: RecordingSummaryState.Pending;
}

interface RecordingSummarySuccess extends BaseRecordingSummary {
  state: RecordingSummaryState.Success;
  content: string;
  inferenceStartedAt: string;
  inferenceFinishedAt: string;
  enhancedSummary?: EnhancedSummary;
}

interface RecordingSummaryError extends BaseRecordingSummary {
  state: RecordingSummaryState.Error;
  errorMessage: string;
}

export type SessionRecordingSummary =
  | RecordingSummaryPending
  | RecordingSummarySuccess
  | RecordingSummaryError;

export enum NeedsFurtherReview {
  TooLarge = 'too_large',
}

export interface EnhancedSummary {
  shortDescription: string;
  detailedDescription: string;
  riskLevel: RiskLevel;
  needsFurtherReview?: NeedsFurtherReview;
  suspiciousActivities: string[];
  compromiseIndicators: boolean;
  notableCommandIndexes: number[];
  commands: CommandAnalysis[];
}

export enum CommandCategory {
  FileOperation = 'File Operation',
  Network = 'Network',
  Process = 'Process',
  SystemConfiguration = 'System Configuration',
  DataAccess = 'Data Access',
  Authentication = 'Authentication',
  Other = 'Other',
}

export enum ThreatCategory {
  Reconnaissance = 'Reconnaissance',
  Execution = 'Execution',
  Persistence = 'Persistence',
  PrivilegeEscalation = 'Privilege Escalation',
  DefenseEvasion = 'Defense Evasion',
  CredentialAccess = 'Credential Access',
  Discovery = 'Discovery',
  LateralMovement = 'Lateral Movement',
  Collection = 'Collection',
  Exfiltration = 'Exfiltration',
  Impact = 'Impact',
}

export interface CommandAnalysis {
  command: string;
  category: CommandCategory;
  success: boolean;
  riskLevel: RiskLevel;
  riskScore: number;
  threatCategory: ThreatCategory;
  timelineTitle: string;
  timelineSubtitle?: string;
  shortDescription: string;
  detailedDescription: string;
  errorMessages: string[];
  suspiciousFlags: string[];
  sensitiveItems: string[];
  suspiciousPatterns: string[];
  indicatorOfCompromise: string[];
  hasSensitiveData: boolean;
  privilegeEscalation: boolean;
  dataExfiltration: boolean;
  persistence: boolean;
  startOffset: number;
  endOffset: number;
}
