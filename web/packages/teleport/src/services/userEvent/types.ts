/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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

  // PreUserEvent types
  //   these events are unauthenticated,
  //   and require username in the request

  PreUserOnboardSetCredentialSubmitEvent = 'tp.ui.onboard.setCredential.submit',
  PreUserOnboardRegisterChallengeSubmitEvent = 'tp.ui.onboard.registerChallenge.submit',
  PreUserOnboardQuestionnaireSubmitEvent = 'tp.ui.onboard.questionnaire.submit',
  PreUserCompleteGoToDashboardClickEvent = 'tp.ui.onboard.completeGoToDashboard.click',

  PreUserRecoveryCodesContinueClickEvent = 'tp.ui.recoveryCodesContinue.click',
  PreUserRecoveryCodesCopyClickEvent = 'tp.ui.recoveryCodesCopy.click',
  PreUserRecoveryCodesPrintClickEvent = 'tp.ui.recoveryCodesPrint.click',
}

export enum IntegrationEnrollEvent {
  Started = 'tp.ui.integrationEnroll.start',
  Complete = 'tp.ui.integrationEnroll.complete',
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
}

export enum DiscoverEvent {
  Started = 'tp.ui.discover.started',
  ResourceSelection = 'tp.ui.discover.resourceSelection',
  IntegrationAWSOIDCConnectEvent = 'tp.ui.discover.integration.awsoidc.connect',
  DatabaseRDSEnrollEvent = 'tp.ui.discover.database.enroll.rds',
  DeployService = 'tp.ui.discover.deployService',
  DatabaseRegister = 'tp.ui.discover.database.register',
  DatabaseConfigureMTLS = 'tp.ui.discover.database.configure.mtls',
  DatabaseConfigureIAMPolicy = 'tp.ui.discover.database.configure.iampolicy',
  DesktopActiveDirectoryToolsInstall = 'tp.ui.discover.desktop.activeDirectory.tools.install',
  DesktopActiveDirectoryConfigure = 'tp.ui.discover.desktop.activeDirectory.configure',
  AutoDiscoveredResources = 'tp.ui.discover.autoDiscoveredResources',
  PrincipalsConfigure = 'tp.ui.discover.principals.configure',
  TestConnection = 'tp.ui.discover.testConnection',
  Completed = 'tp.ui.discover.completed',
}

// DiscoverResource represents a resource type.
export enum DiscoverEventResource {
  Server = 'DISCOVER_RESOURCE_SERVER',
  Kubernetes = 'DISCOVER_RESOURCE_KUBERNETES',
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
  WindowsDesktop = 'DISCOVER_RESOURCE_WINDOWS_DESKTOP',

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

export type IntegrationEnrollEventData = {
  id: string;
  kind: IntegrationEnrollKind;
};

export type IntegrationEnrollEventRequest = {
  event: IntegrationEnrollEvent;
  eventData: IntegrationEnrollEventData;
};

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

export enum CtaEvent {
  CTA_UNSPECIFIED = 0,
  CTA_AUTH_CONNECTOR = 1,
  CTA_ACTIVE_SESSIONS = 2,
  CTA_ACCESS_REQUESTS = 3,
  CTA_PREMIUM_SUPPORT = 4,
  CTA_TRUSTED_DEVICES = 5,
  CTA_UPGRADE_BANNER = 6,
  CTA_BILLING_SUMMARY = 7,
}
