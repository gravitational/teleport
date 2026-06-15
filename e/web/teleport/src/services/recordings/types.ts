import type { ComponentType } from 'react';

import {
  Archive,
  ArrowFatLinesUp,
  Clock,
  Code,
  Database,
  Download,
  Ellipsis,
  FlowArrow,
  FolderShared,
  Git,
  Graph,
  Key,
  KeyHole,
  Kubernetes,
  Magnifier,
  Network as NetworkIcon,
  Play,
  PushPin,
  Question,
  Scan,
  Share,
  ShieldCheck,
  ShieldWarning,
  Terminal,
  Upload,
  User,
  Warning,
  Wrench,
} from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';

import { RiskLevel } from 'teleport/services/recordings';

export enum RecordingSummaryState {
  Pending = 'SUMMARY_STATE_PENDING',
  Success = 'SUMMARY_STATE_SUCCESS',
  Error = 'SUMMARY_STATE_ERROR',
  NoInferencePolicy = 'SUMMARY_STATE_NO_INFERENCE_POLICY',
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

interface RecordingSummaryNoInferencePolicy extends BaseRecordingSummary {
  state: RecordingSummaryState.NoInferencePolicy;
}

export type SessionRecordingSummary =
  | RecordingSummaryPending
  | RecordingSummarySuccess
  | RecordingSummaryError
  | RecordingSummaryNoInferencePolicy;

interface RecordingSummarySuccessResponse extends Omit<
  RecordingSummarySuccess,
  'enhancedSummary'
> {
  enhancedSummary?: EnhancedSummaryResponse;
}

export type SessionRecordingSummaryResponse =
  | RecordingSummaryPending
  | RecordingSummarySuccessResponse
  | RecordingSummaryError
  | RecordingSummaryNoInferencePolicy;

export enum NeedsFurtherReview {
  TooLarge = 'too_large',
  CommandAnalysisFailed = 'command_analysis_failed',
  FailedToFetchAccessRequest = 'failed_to_fetch_access_request',
  AccessRequestResourceMismatch = 'access_request_resource_mismatch',
}

export interface RiskScoreReason {
  reason: string;
  scoreImpact: number;
}

// EnhancedSummaryResponse mirrors the wire shape served by the proxy.
// Both the new session_events / needs_further_review_reasons fields and the deprecated
// commands / needs_further_review fields may be present (a newer proxy populates both for
// cross-version compat; an older proxy populates only the deprecated fields).
export interface EnhancedSummaryResponse {
  shortDescription: string;
  detailedDescription: string;
  riskLevel: RiskLevel;
  suspiciousActivities: string[];
  compromiseIndicators: boolean;
  notableCommandIndexes: number[];
  sessionEvents?: SessionEvent[];
  needsFurtherReviewReasons?: NeedsFurtherReview[];
  riskScoreReasons?: RiskScoreReason[];
  /** @deprecated kept for older proxies; prefer sessionEvents after normalization. */
  commands?: CommandAnalysis[];
  /** @deprecated kept for older proxies; prefer needsFurtherReviewReasons after normalization. */
  needsFurtherReview?: NeedsFurtherReview;
}

// EnhancedSummary is the canonical, version-independent shape that UI components consume.
// The deprecated `commands` / `needsFurtherReview` fields from the wire shape are folded
// into `sessionEvents` / `needsFurtherReviewReasons` during normalization.
export interface EnhancedSummary extends Omit<
  EnhancedSummaryResponse,
  | 'commands'
  | 'needsFurtherReview'
  | 'sessionEvents'
  | 'needsFurtherReviewReasons'
> {
  sessionEvents: SessionEvent[];
  needsFurtherReviewReasons: NeedsFurtherReview[];
}

export enum CommandCategory {
  Unspecified = 'COMMAND_CATEGORY_UNSPECIFIED',
  FileOperation = 'COMMAND_CATEGORY_FILE_OPERATION',
  Network = 'COMMAND_CATEGORY_NETWORK',
  Process = 'COMMAND_CATEGORY_PROCESS',
  SystemConfiguration = 'COMMAND_CATEGORY_SYSTEM_CONFIG',
  DataAccess = 'COMMAND_CATEGORY_DATA_ACCESS',
  Authentication = 'COMMAND_CATEGORY_AUTHENTICATION',
  Other = 'COMMAND_CATEGORY_OTHER',
  PackageManagement = 'COMMAND_CATEGORY_PACKAGE_MANAGEMENT',
  Container = 'COMMAND_CATEGORY_CONTAINER',
  SourceControl = 'COMMAND_CATEGORY_SOURCE_CONTROL',
  Scheduling = 'COMMAND_CATEGORY_SCHEDULING',
  Monitoring = 'COMMAND_CATEGORY_MONITORING',
  UserManagement = 'COMMAND_CATEGORY_USER_MANAGEMENT',
  Transfer = 'COMMAND_CATEGORY_TRANSFER',
  Development = 'COMMAND_CATEGORY_DEVELOPMENT',
}

const commandCategoryLabels: Record<CommandCategory, string> = {
  [CommandCategory.Unspecified]: 'Unspecified',
  [CommandCategory.FileOperation]: 'File Operation',
  [CommandCategory.Network]: 'Network',
  [CommandCategory.Process]: 'Process',
  [CommandCategory.SystemConfiguration]: 'System Configuration',
  [CommandCategory.DataAccess]: 'Data Access',
  [CommandCategory.Authentication]: 'Authentication',
  [CommandCategory.Other]: 'Other',
  [CommandCategory.PackageManagement]: 'Package Management',
  [CommandCategory.Container]: 'Container',
  [CommandCategory.SourceControl]: 'Source Control',
  [CommandCategory.Scheduling]: 'Scheduling',
  [CommandCategory.Monitoring]: 'Monitoring',
  [CommandCategory.UserManagement]: 'User Management',
  [CommandCategory.Transfer]: 'Transfer',
  [CommandCategory.Development]: 'Development',
};

export function formatCommandCategory(category: CommandCategory): string {
  return commandCategoryLabels[category] ?? category;
}

export enum ThreatCategory {
  Unspecified = 'THREAT_CATEGORY_UNSPECIFIED',
  Reconnaissance = 'THREAT_CATEGORY_RECONNAISSANCE',
  Execution = 'THREAT_CATEGORY_EXECUTION',
  Persistence = 'THREAT_CATEGORY_PERSISTENCE',
  PrivilegeEscalation = 'THREAT_CATEGORY_PRIVILEGE_ESCALATION',
  DefenseEvasion = 'THREAT_CATEGORY_DEFENSE_EVASION',
  CredentialAccess = 'THREAT_CATEGORY_CREDENTIAL_ACCESS',
  Discovery = 'THREAT_CATEGORY_DISCOVERY',
  LateralMovement = 'THREAT_CATEGORY_LATERAL_MOVEMENT',
  Collection = 'THREAT_CATEGORY_COLLECTION',
  Exfiltration = 'THREAT_CATEGORY_EXFILTRATION',
  Impact = 'THREAT_CATEGORY_IMPACT',
  None = 'THREAT_CATEGORY_NONE',
}

const threatCategoryLabels: Record<ThreatCategory, string> = {
  [ThreatCategory.Unspecified]: 'Unspecified',
  [ThreatCategory.Reconnaissance]: 'Reconnaissance',
  [ThreatCategory.Execution]: 'Execution',
  [ThreatCategory.Persistence]: 'Persistence',
  [ThreatCategory.PrivilegeEscalation]: 'Privilege Escalation',
  [ThreatCategory.DefenseEvasion]: 'Defense Evasion',
  [ThreatCategory.CredentialAccess]: 'Credential Access',
  [ThreatCategory.Discovery]: 'Discovery',
  [ThreatCategory.LateralMovement]: 'Lateral Movement',
  [ThreatCategory.Collection]: 'Collection',
  [ThreatCategory.Exfiltration]: 'Exfiltration',
  [ThreatCategory.Impact]: 'Impact',
  [ThreatCategory.None]: 'None',
};

export function formatThreatCategory(category: ThreatCategory): string {
  return threatCategoryLabels[category] ?? category;
}

const commandCategoryIcons: Record<
  CommandCategory,
  ComponentType<IconProps>
> = {
  [CommandCategory.Unspecified]: Question,
  [CommandCategory.FileOperation]: FolderShared,
  [CommandCategory.Network]: NetworkIcon,
  [CommandCategory.Process]: Terminal,
  [CommandCategory.SystemConfiguration]: Wrench,
  [CommandCategory.DataAccess]: Database,
  [CommandCategory.Authentication]: Key,
  [CommandCategory.Other]: Ellipsis,
  [CommandCategory.PackageManagement]: Download,
  [CommandCategory.Container]: Kubernetes,
  [CommandCategory.SourceControl]: Git,
  [CommandCategory.Scheduling]: Clock,
  [CommandCategory.Monitoring]: Graph,
  [CommandCategory.UserManagement]: User,
  [CommandCategory.Transfer]: Share,
  [CommandCategory.Development]: Code,
};

export function getCommandCategoryIcon(
  category: CommandCategory
): ComponentType<IconProps> {
  return commandCategoryIcons[category] ?? Question;
}

const threatCategoryIcons: Record<ThreatCategory, ComponentType<IconProps>> = {
  [ThreatCategory.Unspecified]: Question,
  [ThreatCategory.Reconnaissance]: Scan,
  [ThreatCategory.Execution]: Play,
  [ThreatCategory.Persistence]: PushPin,
  [ThreatCategory.PrivilegeEscalation]: ArrowFatLinesUp,
  [ThreatCategory.DefenseEvasion]: ShieldWarning,
  [ThreatCategory.CredentialAccess]: KeyHole,
  [ThreatCategory.Discovery]: Magnifier,
  [ThreatCategory.LateralMovement]: FlowArrow,
  [ThreatCategory.Collection]: Archive,
  [ThreatCategory.Exfiltration]: Upload,
  [ThreatCategory.Impact]: Warning,
  [ThreatCategory.None]: ShieldCheck,
};

export function getThreatCategoryIcon(
  category: ThreatCategory
): ComponentType<IconProps> {
  return threatCategoryIcons[category] ?? Question;
}

// CommandAnalysis is the deprecated event shape served by pre-v19 proxies.
// New code should consume SessionEvent instead; this type is preserved so the
// normalizer can translate legacy responses without losing data.
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
  mitreAttackIds?: string[];
  inferenceErrorMessage?: string;
}

export interface CommandEventDetails {
  command: string;
  success: boolean;
  errorMessages?: string[];
}

export interface DesktopEventDetails {
  applications?: string[];
  visibleUrls?: string[];
  visibleFilePaths?: string[];
  activeWindowTitle?: string;
}

// CommandSessionEvent is a SessionEvent narrowed to one that carries command
// details. Components that only render command-kind events should accept this
// type so callers cannot pass a desktop-only event by mistake.
export type CommandSessionEvent = SessionEvent & {
  commandEventDetails: CommandEventDetails;
};

// SessionEvent is the per-event shape served by v19+ proxies. Either
// commandEventDetails or desktopEventDetails will be populated depending on
// the session kind.
export interface SessionEvent {
  category: CommandCategory;
  riskLevel: RiskLevel;
  riskScore: number;
  threatCategory: ThreatCategory;
  timelineTitle: string;
  timelineSubtitle?: string;
  shortDescription: string;
  detailedDescription: string;
  suspiciousFlags?: string[];
  sensitiveItems?: string[];
  suspiciousPatterns?: string[];
  iocs?: string[];
  mitreAttackIds?: string[];
  hasSensitiveData: boolean;
  privilegeEscalation: boolean;
  dataExfiltration: boolean;
  persistence: boolean;
  startOffset: number;
  endOffset: number;
  inferenceErrorMessage?: string;
  commandEventDetails?: CommandEventDetails;
  desktopEventDetails?: DesktopEventDetails;
}
