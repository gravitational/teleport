/**
 * An enum of the features a user can have access to
 *
 * @remarks
 * This enum is 1:1 with entitlements/entitlements.go
 *
 */
export enum Entitlement {
  Hsm = 'hsm',
  Oidc = 'oidc',
  Assist = 'assist',
  App = 'app',
  Db = 'db',
  Desktop = 'desktop',
  AccessMonitoring = 'accessMonitoring',
  FeatureHiding = 'featureHiding',
  Policy = 'policy',
  Identity = 'identity',
  DeviceTrust = 'deviceTrust',
  K8s = 'k8s',
  UsageReporting = 'usageReporting',
  UpsellAlert = 'upsellAlert',
  Saml = 'saml',
  CloudAuditLogRetention = 'cloudAuditLogRetention',
  ExternalAuditStorage = 'externalAuditStorage',
  JoinActiveSessions = 'joinActiveSessions',
  MobileDeviceManagement = 'mobileDeviceManagement',
  AccessRequests = 'accessRequests',
  AccessLists = 'accessLists',
  SessionLocks = 'sessionLocks',
  OktaUserSync = 'oktaUserSync',
  OktaSCIM = 'oktaSCIM',
}
