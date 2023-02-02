export enum CaptureEvent {
  // UserEvent types
  BannerClickEvent = 'tp.ui.banner.click',
  OnboardAddFirstResourceClickEvent = 'tp.ui.onboard.addFirstResource.click',
  OnboardAddFirstResourceLaterClickEvent = 'tp.ui.onboard.addFirstResourceLater.click',

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

export enum DiscoverEvent {
  Started = 'tp.ui.discover.started',
  ResourceSelection = 'tp.ui.discover.resourceSelection',
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

  ApplicationHttp = 'DISCOVER_RESOURCE_APPLICATION_HTTP',
  ApplicationTcp = 'DISCOVER_RESOURCE_APPLICATION_TCP',
  WindowsDesktop = 'DISCOVER_RESOURCE_WINDOWS_DESKTOP',
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

export type PreUserEvent = UserEvent & {
  username: string;
  mfaType?: string;
  loginFlow?: string;
};

export type DiscoverEventRequest = Omit<UserEvent, 'event'> & {
  event: DiscoverEvent;
  eventData: DiscoverEventData;
};

export type DiscoverEventData = DiscoverEventStepStatus & {
  id: string;
  resource: DiscoverEventResource;
  // AutoDiscoverResourcesCount is the number of
  // auto-discovered resources in the Auto Discovering resources screen.
  // This value is only considered for the 'tp.ui.discover.autoDiscoveredResources'.
  autoDiscoverResourcesCount?: number;
};

export type DiscoverEventStepStatus = {
  stepStatus: DiscoverEventStatus;
  stepStatusError?: string;
};
