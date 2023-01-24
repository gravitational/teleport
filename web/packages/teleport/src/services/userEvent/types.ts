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

export type UserEvent = {
  event: CaptureEvent;
  alert?: string;
};

export type PreUserEvent = UserEvent & {
  username: string;
  mfaType?: string;
  loginFlow?: string;
};

export enum DiscoverEvent {
  Started = 'tp.ui.discover.started.click',
  ResourceSelection = 'tp.ui.discover.resourceSelection.click',

  // TODO(lisa): replace with actual event names as they get implemented:
  DeployService = 'deploy_service',
  ConfigureRegisterDatabase = 'register_database',
  ConfigureDatabaseMTLS = 'mtls',
  ConfigureDatabaseIAMPolicy = 'iam_policy',
  SetUpAccess = 'setup_access',
  TestConnection = 'test_connection',
  Completed = 'completed',
  InstallActiveDirectory = 'install_active_directory',
  ConfigureActiveDirectory = 'configure_active_directory',
  DiscoverDesktops = 'discover_desktops',
}

export type DiscoverEventRequest = Omit<UserEvent, 'event'> & {
  event: DiscoverEvent;
  eventData: DiscoverEventData;
};

export type DiscoverEventData = DiscoverEventStepStatus & {
  id: string;
  resource: DiscoverEventResource;
};

export type DiscoverEventStepStatus = {
  stepStatus: DiscoverEventStatus;
  stepStatusError?: string;
};

export enum DiscoverEventResource {
  Server = 'DISCOVER_RESOURCE_SERVER',
  Kubernetes = 'DISCOVER_RESOURCE_KUBERNETES',
  DatabasePostgresSelfHosted = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_SELF_HOSTED',
  DatabaseMysqlSelfHosted = 'DISCOVER_RESOURCE_DATABASE_MYSQL_SELF_HOSTED',
  DatabaseMongodbSelfHosted = 'DISCOVER_RESOURCE_DATABASE_MONGODB_SELF_HOSTED',
  DatabasePostgresRds = 'DISCOVER_RESOURCE_DATABASE_POSTGRES_RDS',
  DatabaseMysqlRds = 'DISCOVER_RESOURCE_DATABASE_MYSQL_RDS',
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
