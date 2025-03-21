import type { IncludedResourceMode } from 'shared-new/unifiedResources/types';

import type {
  AwsOidcPolicyPreset,
  Regions,
} from '../services/integrations/types';
import type { KubeResourceKind } from '../services/kube/types';
import type { MfaChallengeResponse } from '../services/mfa/types';
import type { RecordingType } from '../services/recording/types';
import type { ParticipantMode } from '../services/session/types';

export interface UrlParams {
  clusterId: string;
  sid?: string;
  login?: string;
  serverId?: string;
}

export interface UrlAppParams {
  fqdn: string;
  clusterId?: string;
  publicAddr?: string;
  arn?: string;
}

export interface CreateAppSessionParams {
  fqdn: string;
  // This API requires cluster_name and public_addr with underscores.
  cluster_name?: string;
  public_addr?: string;
  arn?: string;
  mfaResponse?: MfaChallengeResponse;
}

export interface UrlScpParams {
  clusterId: string;
  serverId: string;
  login: string;
  location: string;
  filename: string;
  moderatedSessionId?: string;
  fileTransferRequestId?: string;
  mfaResponse?: MfaChallengeResponse;
}

export interface UrlSshParams {
  login?: string;
  serverId?: string;
  sid?: string;
  mode?: ParticipantMode;
  clusterId: string;
}

export interface UrlKubeExecParams {
  clusterId: string;
  kubeId: string;
}

export interface UrlDbConnectParams {
  clusterId: string;
  serviceName: string;
}

export interface UrlSessionRecordingsParams {
  start: string;
  end: string;
  limit?: number;
  startKey?: string;
}

export interface UrlClusterEventsParams {
  start: string;
  end: string;
  limit?: number;
  include?: string;
  startKey?: string;
}

export interface UrlLauncherParams {
  fqdn: string;
  clusterId?: string;
  publicAddr?: string;
  arn?: string;
}

export interface UrlPlayerParams {
  clusterId: string;
  sid: string;
}

export interface UrlPlayerSearch {
  recordingType: RecordingType;
  durationMs?: number;
}

// /web/cluster/:clusterId/desktops/:desktopName/:username
export interface UrlDesktopParams {
  username?: string;
  desktopName?: string;
  clusterId: string;
}

export interface UrlListRolesParams {
  search?: string;
  limit?: number;
  startKey?: string;
}

export interface SortType {
  fieldName: string;
  dir: SortDir;
}

export type SortDir = 'ASC' | 'DESC';

export interface UrlResourcesParams {
  query?: string;
  search?: string;
  sort?: SortType;
  limit?: number;
  startKey?: string;
  searchAsRoles?: 'yes' | '';
  pinnedOnly?: boolean;
  includedResourceMode?: IncludedResourceMode;
  // TODO(bl-nero): Remove this once filters are expressed as advanced search.
  kinds?: string[];
}

export interface UrlKubeResourcesParams {
  query?: string;
  search?: string;
  sort?: SortType;
  limit?: number;
  startKey?: string;
  searchAsRoles?: 'yes' | '';
  kubeNamespace?: string;
  kubeCluster: string;
  kind: Omit<KubeResourceKind, '*'>;
}

export interface UrlIntegrationParams {
  name?: string;
  resourceType?: string;
  regions?: string[];
}

export interface UrlDeployServiceIamConfigureScriptParams {
  integrationName: string;
  region: Regions;
  awsOidcRoleArn: string;
  taskRoleArn: string;
  accountID: string;
}

export interface UrlAwsOidcConfigureIdp {
  integrationName: string;
  roleName: string;
  policyPreset?: AwsOidcPolicyPreset;
}

export interface UrlAwsConfigureIamScriptParams {
  region: Regions;
  iamRoleName: string;
  accountID: string;
}

export interface UrlAwsConfigureIamEc2AutoDiscoverWithSsmScriptParams {
  region: Regions;
  iamRoleName: string;
  ssmDocument: string;
  integrationName: string;
  accountID: string;
}

export interface UrlGcpWorkforceConfigParam {
  orgId: string;
  poolName: string;
  poolProviderName: string;
}

export interface UrlNotificationParams {
  clusterId: string;
  limit?: number;
  startKey?: string;
}

export type TeleportEdition = 'ent' | 'community' | 'oss';
