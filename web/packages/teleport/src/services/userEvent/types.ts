/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// these constants are 1:1 with constants found in lib/usagereporter/web/userevent.go
export enum CaptureEvent {
  // UserEvent types
  BannerClickEvent = 'tp.ui.banner.click',
  OnboardAddFirstResourceClickEvent = 'tp.ui.onboard.addFirstResource.click',
  OnboardAddFirstResourceLaterClickEvent = 'tp.ui.onboard.addFirstResourceLater.click',
  CreateNewRoleClickEvent = 'tp.ui.createNewRole.click',
  CreateNewRoleSaveClickEvent = 'tp.ui.createNewRoleSave.click',
  CreateNewRoleCancelClickEvent = 'tp.ui.createNewRoleCancel.click',
  CreateNewRoleViewDocumentationClickEvent = 'tp.ui.createNewRoleViewDocumentation.click',
  UiCallToActionClickEvent = 'tp.ui.callToAction.click',
  FeatureRecommendationEvent = 'tp.ui.feature.recommendation',

  // PreUserEvent types
  //   these events are unauthenticated,
  //   and require username in the request
  PreUserOnboardSetCredentialSubmitEvent = 'tp.ui.onboard.setCredential.submit',
  PreUserOnboardRegisterChallengeSubmitEvent = 'tp.ui.onboard.registerChallenge.submit',
  PreUserCompleteGoToDashboardClickEvent = 'tp.ui.onboard.completeGoToDashboard.click',
  PreUserRecoveryCodesContinueClickEvent = 'tp.ui.recoveryCodesContinue.click',
  PreUserRecoveryCodesCopyClickEvent = 'tp.ui.recoveryCodesCopy.click',
  PreUserRecoveryCodesPrintClickEvent = 'tp.ui.recoveryCodesPrint.click',
}

/**
 * IntegrationEnrollEvent defines integration enrollment
 * events.
 */
export enum IntegrationEnrollEvent {
  Started = 'tp.ui.integrationEnroll.start',
  Complete = 'tp.ui.integrationEnroll.complete',
  Step = 'tp.ui.integrationEnroll.step',
}

// IntegrationEnrollKind represents a integration type.
export enum IntegrationEnrollKind {
  Unspecified = 'INTEGRATION_ENROLL_KIND_UNSPECIFIED',
  Slack = 'INTEGRATION_ENROLL_KIND_SLACK',
  AwsOidc = 'INTEGRATION_ENROLL_KIND_AWS_OIDC',
  PagerDuty = 'INTEGRATION_ENROLL_KIND_PAGERDUTY',
  Email = 'INTEGRATION_ENROLL_KIND_EMAIL',
  Jira = 'INTEGRATION_ENROLL_KIND_JIRA',
  Discord = 'INTEGRATION_ENROLL_KIND_DISCORD',
  Mattermost = 'INTEGRATION_ENROLL_KIND_MATTERMOST',
  MsTeams = 'INTEGRATION_ENROLL_KIND_MS_TEAMS',
  OpsGenie = 'INTEGRATION_ENROLL_KIND_OPSGENIE',
  Okta = 'INTEGRATION_ENROLL_KIND_OKTA',
  Jamf = 'INTEGRATION_ENROLL_KIND_JAMF',
  ServiceNow = 'INTEGRATION_ENROLL_KIND_SERVICENOW',
  MachineID = 'INTEGRATION_ENROLL_KIND_MACHINE_ID',
  MachineIDGitHubActions = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS',
  MachineIDCircleCI = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_CIRCLECI',
  MachineIDGitLab = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITLAB',
  MachineIDJenkins = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_JENKINS',
  MachineIDAnsible = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_ANSIBLE',
  MachineIDAWS = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_AWS',
  MachineIDGCP = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GCP',
  MachineIDAzure = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_AZURE',
  MachineIDSpacelift = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_SPACELIFT',
  MachineIDKubernetes = 'INTEGRATION_ENROLL_KIND_MACHINE_ID_KUBERNETES',
  EntraId = 'INTEGRATION_ENROLL_KIND_ENTRA_ID',
  DatadogIncidentManagement = 'INTEGRATION_ENROLL_KIND_DATADOG_INCIDENT_MANAGEMENT',
  AwsIdentityCenter = 'INTEGRATION_ENROLL_KIND_AWS_IDENTITY_CENTER',
  GitHubRepoAccess = 'INTEGRATION_ENROLL_KIND_GITHUB_REPO_ACCESS',
}

/**
 * IntegrationEnrollStep defines configurable steps for an integration type.
 * Value matches with proto enums defined in the backend.
 */
export enum IntegrationEnrollStep {
  /**
   * AWSIC steps defined for AWS Idenity Center plugin.
   */
  ConnectOidc = 'INTEGRATION_ENROLL_STEP_AWSIC_CONNECT_OIDC',
  ImportResourceSetDefaultOwner = 'INTEGRATION_ENROLL_STEP_AWSIC_SET_ACCESSLIST_DEFAULT_OWNER',
  IdentitySourceUploadSamlMetadata = 'INTEGRATION_ENROLL_STEP_AWSIC_UPLOAD_AWS_SAML_SP_METADATA',
  ScimTestConnection = 'INTEGRATION_ENROLL_STEP_AWSIC_TEST_SCIM_CONNECTION',

  /**
   * GITHUBRA denotes GitHub Repo Access.
   */
  GitHubRaCreateIntegration = 'INTEGRATION_ENROLL_STEP_GITHUBRA_CREATE_INTEGRATION',
  GitHubRaCreateGitServer = 'INTEGRATION_ENROLL_STEP_GITHUBRA_CREATE_GIT_SERVER',
  GitHubRaConfigureSshCert = 'INTEGRATION_ENROLL_STEP_GITHUBRA_CONFIGURE_SSH_CERT',
  GitHubRaCreateRole = 'INTEGRATION_ENROLL_STEP_GITHUBRA_CREATE_ROLE',
}

/**
 * IntegrationEnrollStatusCode defines status codes for a given
 * integration configuration step event.
 * Value matches with proto enums defined in the backend.
 */
export enum IntegrationEnrollStatusCode {
  Success = 'INTEGRATION_ENROLL_STATUS_CODE_SUCCESS',
  Skipped = 'INTEGRATION_ENROLL_STATUS_CODE_SKIPPED',
  Error = 'INTEGRATION_ENROLL_STATUS_CODE_ERROR',
  Aborted = 'INTEGRATION_ENROLL_STATUS_CODE_ABORTED',
}

/**
 * IntegrationEnrollStepStatus defines fields for reporting
 * integration configuration step event.
 */
export type IntegrationEnrollStepStatus =
  | {
      code: Exclude<
        IntegrationEnrollStatusCode,
        IntegrationEnrollStatusCode.Error
      >;
    }
  | {
      code: IntegrationEnrollStatusCode.Error;
      error: string;
    };

/**
 * IntegrationEnrollEventData defines integration
 * enroll event. Use for start, complete and step events.
 */
export type IntegrationEnrollEventData = {
  id: string;
  kind: IntegrationEnrollKind;
  step?: IntegrationEnrollStep;
  status?: IntegrationEnrollStepStatus;
};

/**
 * IntegrationEnrollEventRequest defines integration enroll
 * event request as expected in the backend.
 */
export type IntegrationEnrollEventRequest = {
  event: IntegrationEnrollEvent;
  eventData: IntegrationEnrollEventData;
};

// These constants should match the constant defined in backend found in:
// lib/usagereporter/web/userevent.go
export enum DiscoverEvent {
  Started = 'tp.ui.discover.started',
  ResourceSelection = 'tp.ui.discover.resourceSelection',
  IntegrationAWSOIDCConnectEvent = 'tp.ui.discover.integration.awsoidc.connect',
  DatabaseRDSEnrollEvent = 'tp.ui.discover.database.enroll.rds',
  DeployService = 'tp.ui.discover.deployService',
  DatabaseRegister = 'tp.ui.discover.database.register',
  DatabaseConfigureMTLS = 'tp.ui.discover.database.configure.mtls',
  DatabaseConfigureIAMPolicy = 'tp.ui.discover.database.configure.iampolicy',
  CreateApplicationServer = 'tp.ui.discover.createAppServer',
  CreateDiscoveryConfig = 'tp.ui.discover.createDiscoveryConfig',
  KubeEKSEnrollEvent = 'tp.ui.discover.kube.enroll.eks',
  PrincipalsConfigure = 'tp.ui.discover.principals.configure',
  TestConnection = 'tp.ui.discover.testConnection',
  Completed = 'tp.ui.discover.completed',
}

// DiscoverResource represents a resource type.
// Constants should match the constant generated from backend proto files:
//  - usageevents/v1/usageevents.proto
//  - prehog/v1alpha/teleport.proto
export enum DiscoverEventResource {
  Server = 'DISCOVER_RESOURCE_SERVER',
  Kubernetes = 'DISCOVER_RESOURCE_KUBERNETES',
  KubernetesEks = 'DISCOVER_RESOURCE_KUBERNETES_EKS',
  DatabasePostgresSelfHosted = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_SELF_HOSTED',
  DatabaseMysqlSelfHosted = 'DISCOVER_RESOURCE_DATABASE_MYSQL_SELF_HOSTED',
  DatabaseMongodbSelfHosted = 'DISCOVER_RESOURCE_DATABASE_MONGODB_SELF_HOSTED',
  DatabasePostgresRds = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_RDS',
  DatabaseMysqlRds = 'DISCOVER_RESOURCE_DATABASE_MYSQL_RDS',

  DatabaseSqlServerRds = 'DISCOVER_RESOURCE_DATABASE_SQLSERVER_RDS',
  DatabasePostgresRedshift = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT',

  DatabaseSqlServerSelfHosted = 'DISCOVER_RESOURCE_DATABASE_SQLSERVER_SELF_HOSTED',
  DatabaseRedisSelfHosted = 'DISCOVER_RESOURCE_DATABASE_REDIS_SELF_HOSTED',

  DatabasePostgresGcp = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_GCP',
  DatabaseMysqlGcp = 'DISCOVER_RESOURCE_DATABASE_MYSQL_GCP',
  DatabaseSqlServerGcp = 'DISCOVER_RESOURCE_DATABASE_SQLSERVER_GCP',

  DatabasePostgresRedshiftServerless = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT_SERVERLESS',
  DatabasePostgresAzure = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_AZURE',
  DatabaseDynamoDb = 'DISCOVER_RESOURCE_DATABASE_DYNAMODB',
  DatabaseCassandraKeyspaces = 'DISCOVER_RESOURCE_DATABASE_CASSANDRA_KEYSPACES',
  DatabaseCassandraSelfHosted = 'DISCOVER_RESOURCE_DATABASE_CASSANDRA_SELF_HOSTED',
  DatabaseElasticSearchSelfHosted = 'DISCOVER_RESOURCE_DATABASE_ELASTICSEARCH_SELF_HOSTED',
  DatabaseRedisElasticache = 'DISCOVER_RESOURCE_DATABASE_REDIS_ELASTICACHE',
  DatabaseRedisMemoryDb = 'DISCOVER_RESOURCE_DATABASE_REDIS_MEMORYDB',
  DatabaseRedisAzureCache = 'DISCOVER_RESOURCE_DATABASE_REDIS_AZURE_CACHE',
  DatabaseRedisClusterSelfHosted = 'DISCOVER_RESOURCE_DATABASE_REDIS_CLUSTER_SELF_HOSTED',

  DatabaseMysqlAzure = 'DISCOVER_RESOURCE_DATABASE_MYSQL_AZURE',
  DatabaseSqlServerAzure = 'DISCOVER_RESOURCE_DATABASE_SQLSERVER_AZURE',
  DatabaseSqlServerMicrosoft = 'DISCOVER_RESOURCE_DATABASE_SQLSERVER_MICROSOFT',
  DatabaseCockroachDbSelfHosted = 'DISCOVER_RESOURCE_DATABASE_COCKROACHDB_SELF_HOSTED',
  DatabaseMongodbAtlas = 'DISCOVER_RESOURCE_DATABASE_MONGODB_ATLAS',
  DatabaseSnowflake = 'DISCOVER_RESOURCE_DATABASE_SNOWFLAKE',

  DatabaseDocRdsProxy = 'DISCOVER_RESOURCE_DOC_DATABASE_RDS_PROXY',
  DatabaseDocHighAvailability = 'DISCOVER_RESOURCE_DOC_DATABASE_HIGH_AVAILABILITY',
  DatabaseDocDynamicRegistration = 'DISCOVER_RESOURCE_DOC_DATABASE_DYNAMIC_REGISTRATION',

  ApplicationHttp = 'DISCOVER_RESOURCE_APPLICATION_HTTP',
  ApplicationTcp = 'DISCOVER_RESOURCE_APPLICATION_TCP',
  ApplicationAwsConsole = 'DISCOVER_RESOURCE_APPLICATION_AWS_CONSOLE',
  WindowsDesktop = 'DISCOVER_RESOURCE_WINDOWS_DESKTOP',
  WindowsDesktopNonAD = 'DISCOVER_RESOURCE_DOC_WINDOWS_DESKTOP_NON_AD',

  Ec2Instance = 'DISCOVER_RESOURCE_EC2_INSTANCE',

  SamlApplication = 'DISCOVER_RESOURCE_SAML_APPLICATION',
}

export enum DiscoverEventStatus {
  Success = 'DISCOVER_STATUS_SUCCESS',
  Skipped = 'DISCOVER_STATUS_SKIPPED',
  Error = 'DISCOVER_STATUS_ERROR',
  Aborted = 'DISCOVER_STATUS_ABORTED', // user exits the wizard
}

export type UserEvent = {
  event: CaptureEvent;
  alert?: string;
};

export type EventMeta = {
  username: string;
  mfaType?: string;
  loginFlow?: string;
};

export type PreUserEvent = UserEvent & EventMeta;

export type DiscoverEventRequest = Omit<UserEvent, 'event'> & {
  event: DiscoverEvent;
  eventData: DiscoverEventData;
};

export type DiscoverEventData = DiscoverEventStepStatus & {
  id: string;
  resource: DiscoverEventResource;
  // autoDiscoverResourcesCount is the number of
  // auto-discovered resources in the Auto Discovering resources screen.
  // This value is only considered for the 'tp.ui.discover.autoDiscoveredResources'.
  autoDiscoverResourcesCount?: number;
  // selectedResourcesCount is the number of
  // resources that a user has selected
  //
  // eg: number of RDS databases selected
  // in the RDS enrollment screen for event
  // tp.ui.discover.database.enroll.rds
  selectedResourcesCount?: number;

  // serviceDeploy is only considered for 'tp.ui.discover.deployService'
  // event and describes how an agent got deployed.
  serviceDeploy?: DiscoverServiceDeploy;

  // discoveryConfigMethod is only considered for 'tp.ui.discover.createDiscoveryConfig'
  // event and describes how discovery configured.
  discoveryConfigMethod?: DiscoverDiscoveryConfigMethod;
};

export type DiscoverEventStepStatus = {
  stepStatus: DiscoverEventStatus;
  stepStatusError?: string;
};

export type DiscoverServiceDeploy = {
  method: DiscoverServiceDeployMethod;
  type: DiscoverServiceDeployType;
};

export enum DiscoverServiceDeployMethod {
  Unspecified = 'DEPLOY_METHOD_UNSPECIFIED',
  Auto = 'DEPLOY_METHOD_AUTO',
  Manual = 'DEPLOY_METHOD_MANUAL',
}

export enum DiscoverServiceDeployType {
  Unspecified = 'DEPLOY_TYPE_UNSPECIFIED',
  InstallScript = 'DEPLOY_TYPE_INSTALL_SCRIPT',
  AmazonEcs = 'DEPLOY_TYPE_AMAZON_ECS',
}

export enum DiscoverDiscoveryConfigMethod {
  Unspecified = 'CONFIG_METHOD_UNSPECIFIED',
  AwsEc2Ssm = 'CONFIG_METHOD_AWS_EC2_SSM',
  AwsRdsEcs = 'CONFIG_METHOD_AWS_RDS_ECS',
  AwsEks = 'CONFIG_METHOD_AWS_EKS',
}

export enum CtaEvent {
  CTA_UNSPECIFIED = 0,
  CTA_AUTH_CONNECTOR = 1,
  CTA_ACTIVE_SESSIONS = 2,
  CTA_ACCESS_REQUESTS = 3,
  CTA_PREMIUM_SUPPORT = 4,
  CTA_TRUSTED_DEVICES = 5,
  CTA_UPGRADE_BANNER = 6,
  CTA_BILLING_SUMMARY = 7,
  CTA_ACCESS_LIST = 8,
  CTA_ACCESS_MONITORING = 9,
  CTA_EXTERNAL_AUDIT_STORAGE = 10,
  CTA_OKTA_USER_SYNC = 11,
  CTA_ENTRA_ID = 12,
  CTA_OKTA_SCIM = 13,
}

export enum Feature {
  FEATURES_UNSPECIFIED = 0,
  FEATURES_TRUSTED_DEVICES = 1,
}

export enum FeatureRecommendationStatus {
  FEATURE_RECOMMENDATION_STATUS_UNSPECIFIED = 0,
  FEATURE_RECOMMENDATION_STATUS_NOTIFIED = 1,
  FEATURE_RECOMMENDATION_STATUS_DONE = 2,
}

export type FeatureRecommendationEvent = {
  Feature: Feature;
  FeatureRecommendationStatus: FeatureRecommendationStatus;
};
