export enum HeadlessAuthenticationType {
  UNSPECIFIED = 0,
  HEADLESS = 1,
  BROWSER = 2,
  SESSION = 3,
}

export function getHeadlessAuthTypeLabel(type: number): string {
  switch (type) {
    case HeadlessAuthenticationType.HEADLESS:
      return 'headless';
    case HeadlessAuthenticationType.BROWSER:
      return 'browser';
    case HeadlessAuthenticationType.SESSION:
      return 'session';
    default:
      return 'unknown';
  }
}

export function getActionFromAuthType(authType: HeadlessAuthenticationType): string {
  switch (authType) {
    case HeadlessAuthenticationType.BROWSER:
      return 'login';
    case HeadlessAuthenticationType.HEADLESS:
    case HeadlessAuthenticationType.SESSION:
      return 'command';
    default:
      return 'unknown';
  }
}