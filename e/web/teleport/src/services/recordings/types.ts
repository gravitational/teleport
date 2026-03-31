import type { ComponentType } from 'react';

import {
  Archive,
  ArrowFatLinesUp,
  Database,
  Ellipsis,
  FlowArrow,
  FolderShared,
  Key,
  KeyHole,
  Magnifier,
  Network as NetworkIcon,
  Play,
  PushPin,
  Question,
  Scan,
  ShieldCheck,
  ShieldWarning,
  Terminal,
  Upload,
  Warning,
  Wrench,
} from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';

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
  CommandAnalysisFailed = 'command_analysis_failed',
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
  Unspecified = 'COMMAND_CATEGORY_UNSPECIFIED',
  FileOperation = 'COMMAND_CATEGORY_FILE_OPERATION',
  Network = 'COMMAND_CATEGORY_NETWORK',
  Process = 'COMMAND_CATEGORY_PROCESS',
  SystemConfiguration = 'COMMAND_CATEGORY_SYSTEM_CONFIG',
  DataAccess = 'COMMAND_CATEGORY_DATA_ACCESS',
  Authentication = 'COMMAND_CATEGORY_AUTHENTICATION',
  Other = 'COMMAND_CATEGORY_OTHER',
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
};

export function formatCommandCategory(category: CommandCategory): string {
  return commandCategoryLabels[category] ?? category;
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
};

export function getCommandCategoryIcon(
  category: CommandCategory
): ComponentType<IconProps> {
  return commandCategoryIcons[category] ?? Question;
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
  inferenceErrorMessage?: string;
}
