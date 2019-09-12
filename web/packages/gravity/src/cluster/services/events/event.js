/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

export const CodeEnum = {
  // Teleport
  AUTH_ATTEMPT_FAILURE: 'T3007W',
  CLIENT_DISCONNECT: 'T3006I',
  EXEC_FAILURE: 'T3002E',
  EXEC: 'T3002I',
  PORTFORWARD_FAILURE: 'T3003E',
  PORTFORWARD: 'T3003I',
  SCP_DOWNLOAD_FAILURE: 'T3004E',
  SCP_DOWNLOAD: 'T3004I',
  SCP_UPLOAD_FAILURE: 'T3005E',
  SCP_UPLOAD: 'T3005I',
  SESSION_END: 'T2004I',
  SESSION_JOIN: 'T2001I',
  SESSION_LEAVE: 'T2003I',
  SESSION_START: 'T2000I',
  SESSION_UPLOAD: 'T2005I',
  SUBSYSTEM_FAILURE: 'T3001E',
  SUBSYSTEM: 'T3001I',
  TERMINAL_RESIZE: 'T2002I',
  USER_LOCAL_LOGIN: 'T1000I',
  USER_LOCAL_LOGINFAILURE: 'T1000W',
  USER_SSO_LOGIN: 'T1001I',
  USER_SSO_LOGINFAILURE: 'T1001W',
  // Gravity Oss
  ALERT_CREATED: 'G1007I',
  ALERT_DELETED: 'G2007I',
  ALERT_TARGET_CREATED: 'G1008I',
  ALERT_TARGET_DELETED: 'G2008I',
  APPLICATION_INSTALL: 'G4000I',
  APPLICATION_ROLLBACK: 'G4002I',
  APPLICATION_UNINSTALL: 'G4003I',
  APPLICATION_UPGRADE: 'G4001I',
  AUTHGATEWAY_UPDATED: 'G1009I',
  AUTHPREFERENCE_UPDATED: 'G1005I',
  CLUSTER_HEALTHY: 'G3001I',
  CLUSTER_UNHEALTHY: 'G3000W',
  GITHUB_CONNECTOR_CREATED: 'G1002I',
  GITHUB_CONNECTOR_DELETED: 'G2002I',
  LOGFORWARDER_CREATED: 'G1003I',
  LOGFORWARDER_DELETED: 'G2003I',
  OPERATION_CONFIG_COMPLETE: 'G0016I',
  OPERATION_CONFIG_FAILURE: 'G0016E',
  OPERATION_CONFIG_START: 'G0015I',
  OPERATION_ENV_COMPLETE: 'G0014I',
  OPERATION_ENV_FAILURE: 'G0014E',
  OPERATION_ENV_START: 'G0013I',
  OPERATION_EXPAND_COMPLETE: 'G0004I',
  OPERATION_EXPAND_FAILURE: 'G0004E',
  OPERATION_EXPAND_START: 'G0003I',
  OPERATION_GC_COMPLETE: 'G0012I',
  OPERATION_GC_FAILURE: 'G0012E',
  OPERATION_GC_START: 'G0011I',
  OPERATION_INSTALL_COMPLETE: 'G0002I',
  OPERATION_INSTALL_FAILURE: 'G0002E',
  OPERATION_INSTALL_START: 'G0001I',
  OPERATION_SHRINK_COMPLETE: 'G0006I',
  OPERATION_SHRINK_FAILURE: 'G0006E',
  OPERATION_SHRINK_START: 'G0005I',
  OPERATION_UNINSTALL_COMPLETE: 'G0010I',
  OPERATION_UNINSTALL_FAILURE: 'G0010E',
  OPERATION_UNINSTALL_START: 'G0009I',
  OPERATION_UPDATE_COMPLETE: 'G0008I',
  OPERATION_UPDATE_FAILURE: 'G0008E',
  OPERATION_UPDATE_START: 'G0007I',
  ROLE_CREATED: 'GE1000I',
  ROLE_DELETED: 'GE2000I',
  SMTPCONFIG_CREATED: 'G1006I',
  SMTPCONFIG_DELETED: 'G2006I',
  TLSKEYPAIR_CREATED: 'G1004I',
  TLSKEYPAIR_DELETED: 'G2004I',
  TOKEN_CREATED: 'G1001I',
  TOKEN_DELETED: 'G2001I',
  USER_CREATED: 'G1000I',
  USER_DELETED: 'G2000I',
  USER_INVITE_CREATED: 'G1010I',
  // Gravity E
  ENDPOINTS_UPDATED: 'GE1003I',
  LICENSE_EXPIRED: 'GE3003I',
  LICENSE_GENERATED: 'GE3002I',
  LICENSE_UPDATED: 'GE3004I',
  OIDC_CONNECTOR_CREATED: 'GE1001I',
  OIDC_CONNECTOR_DELETED: 'GE2001I',
  REMOTE_SUPPORT_DISABLED: 'GE3001I',
  REMOTE_SUPPORT_ENABLED: 'GE3000I',
  SAML_CONNECTOR_CREATED: 'GE1002I',
  SAML_CONNECTOR_DELETED: 'GE2002I',
  UPDATES_DISABLED: 'GE3006I',
  UPDATES_DOWNLOADED: 'GE3007I',
  UPDATES_ENABLED: 'GE3005I',
}

export const eventConfig = {
  [CodeEnum.ALERT_CREATED]: {
    desc: 'Alert Created',
    formatter: ({ user, name }) => `User ${user} created monitoring alert ${name}`,
  },
  [CodeEnum.ALERT_DELETED]: {
    desc: 'Alert Deleted',
    formatter: ({ user, name }) => `User ${user} deleted monitoring alert ${name}`,
  },
  [CodeEnum.ALERT_TARGET_CREATED]: {
    desc: 'Alert Target Created',
    formatter: ({ user }) => `User ${user} updated monitoring alert target`,
  },
  [CodeEnum.ALERT_TARGET_DELETED]: {
    desc: 'Alert Target Deleted',
    formatter: ({ user }) => `User ${user} deleted monitoring alert target`,
  },
  [CodeEnum.APPLICATION_INSTALL]: {
    desc: 'Application Installed',
    formatter: ({ releaseName, name, version }) => `Application release ${releaseName} (${name}:${version}) has been installed`,
  },
  [CodeEnum.APPLICATION_UPGRADE]: {
    desc: 'Application Upgraded',
    formatter: ({ releaseName, name, version }) => `Application release ${releaseName} has been upgraded to ${name}:${version}`,
  },
  [CodeEnum.APPLICATION_ROLLBACK]: {
    desc: 'Application Rolledbacked',
    formatter: ({ releaseName, name, version }) => `Application release ${releaseName} has been rolled back to ${name}:${version}`,
  },
  [CodeEnum.APPLICATION_UNINSTALL]: {
    desc: 'Application Uninstalled',
    formatter: ({ releaseName, name, version }) => `Applicaiton release ${releaseName} (${name}:${version}) has been uninstalled`,
  },
  [CodeEnum.AUTH_ATTEMPT_FAILURE]: {
    desc: 'Auth Attempt Failed',
    formatter: ({ user, error }) => `User ${user} failed auth attempt: ${error}`,
  },
  [CodeEnum.AUTHGATEWAY_UPDATED]: {
    desc: 'Auth Gateway Updated',
    formatter: ({ user }) => `User ${user} updated cluster authentication gateway settings`,
  },
  [CodeEnum.AUTHPREFERENCE_UPDATED]: {
    desc: 'Auth Preferences Updated',
    formatter: ({ user }) => `User ${user} updated cluster authentication preference`,
  },
  [CodeEnum.CLIENT_DISCONNECT]: {
    desc: 'Client Disconnected',
    formatter: ({ user, reason }) => `User ${user} has been disconnected: ${reason}`
  },
  [CodeEnum.CLUSTER_HEALTHY]: {
    desc: 'Cluster Healthy',
    formatter: () => `Cluster has become healthy`,
  },
  [CodeEnum.CLUSTER_UNHEALTHY]: {
    desc: 'Cluster Unhealthy',
    formatter: ({ reason }) => `Cluster is degraded: ${reason}`,
  },
  [CodeEnum.ENDPOINTS_UPDATED]: {
    desc: 'Endpoints Updated',
    formatter: ({ user }) => `User ${user} updated Ops Center endpoints`,
  },
  [CodeEnum.EXEC]: {
    desc: 'Command Execution',
    formatter: ({ user, ...rest }) => `User ${user} executed a command on node ${rest["addr.local"]}`,
  },
  [CodeEnum.EXEC_FAILURE]: {
    desc: 'Command Execution Failed',
    formatter: ({ user, exitError, ...rest }) => `User ${user} command execution on node ${rest["addr.local"]} failed: ${exitError}`,
  },
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: {
    desc: 'GITHUB Auth Connector Created',
    formatter: ({ user, name }) => `User ${user} created Github connector ${name}`,
  },
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: {
    desc: 'GITHUB Auth Connector Deleted',
    formatter: ({ user, name }) => `User ${user} deleted Github connector ${name}`,
  },
  [CodeEnum.LICENSE_GENERATED]: {
    desc: 'Cluster License Generated',
    formatter: ({ maxNodes }) => `License for max nodes ${maxNodes} has been generated`,
  },
  [CodeEnum.LICENSE_EXPIRED]: {
    desc: 'Cluster License Expired',
    formatter: () => `Cluster license has expired`,
  },
  [CodeEnum.LICENSE_UPDATED]: {
    desc: 'Cluster License Updated',
    formatter: () => `Cluster license has been updated`,
  },
  [CodeEnum.LOGFORWARDER_CREATED]: {
    desc: 'Log Forwarder Created',
    formatter: ({ user, name }) => `User ${user} created log forwarder ${name}`,
  },
  [CodeEnum.LOGFORWARDER_DELETED]: {
    desc: 'Log Forwarder Deleted',
    formatter: ({ user, name }) => `User ${user} deleted log forwarder ${name}`,
  },
  [CodeEnum.OIDC_CONNECTOR_CREATED]: {
    desc: 'OIDC Auth Connector Created',
    formatter: ({ user, name }) => `User ${user} created OIDC connector ${name}`
  },
  [CodeEnum.OIDC_CONNECTOR_DELETED]: {
    desc: 'OIDC Auth Connector Deleted',
    formatter: ({ user, name }) => `User ${user} deleted OIDC connector ${name}`
  },
  [CodeEnum.OPERATION_CONFIG_COMPLETE]: {
    desc: 'Cluster Configuration Completed',
    formatter: () => `Cluster configuration has been updated`,
  },
  [CodeEnum.OPERATION_CONFIG_FAILURE]: {
    desc: 'Cluster Configuration Failed',
    formatter: () => `Failed to update the cluster configuration`,
  },
  [CodeEnum.OPERATION_CONFIG_START]: {
    desc: 'Cluster Configuration Started',
    formatter: () => `Updating the cluster configuration`
  },
  [CodeEnum.OPERATION_ENV_COMPLETE]: {
    desc: 'Environment Update Completed',
    formatter: () => `Cluster runtime environment has been updated`
  },
  [CodeEnum.OPERATION_ENV_FAILURE]: {
    desc: 'Environment Update Failed',
    formatter: () => `Failed to update the cluster runtime environment`
  },
  [CodeEnum.OPERATION_ENV_START]: {
    desc: 'Environment Update Started',
    formatter: () => `Updating the cluster runtime environment`
  },
  [CodeEnum.OPERATION_EXPAND_START]: {
    desc: 'Cluster Expand Started',
    formatter: ({ hostname, ip, role }) => `Node ${hostname} (${ip}) with role ${role} is joining the cluster`
  },
  [CodeEnum.OPERATION_EXPAND_COMPLETE]: {
    desc: 'Cluster Expand Completed',
    formatter: ({ hostname, ip, role }) => `Node ${hostname} (${ip}) with role ${role} has joined the cluster`
  },
  [CodeEnum.OPERATION_EXPAND_FAILURE]: {
    desc: 'Cluster Expand Failed',
    formatter: ({ hostname, ip, role }) => `Node ${hostname} (${ip}) with role ${role} has failed to join the cluster`
  },
  [CodeEnum.OPERATION_GC_START]: {
    desc: 'GC Started',
    formatter: () => 'Running garbage collection on the cluster'
  },
  [CodeEnum.OPERATION_GC_COMPLETE]: {
    desc: 'GC Completed',
    formatter: () => 'Garbage collection on the cluster has finished'
  },
  [CodeEnum.OPERATION_GC_FAILURE]: {
    desc: 'GC Failed',
    formatter: () => 'Garbage collection on the cluster has failed'
  },
  [CodeEnum.OPERATION_INSTALL_START]: {
    desc: 'Cluster Install Started',
    formatter: ({ cluster }) => `Cluster ${cluster} is being installed`
  },
  [CodeEnum.OPERATION_INSTALL_COMPLETE]: {
    desc: 'Cluster Install Completed',
    formatter: ({ cluster }) => `Cluster ${cluster} has been installed`
  },
  [CodeEnum.OPERATION_INSTALL_FAILURE]: {
    desc: 'Cluster Install Failed',
    formatter: ({ cluster }) => `Cluster ${cluster} install has failed`
  },
  [CodeEnum.OPERATION_SHRINK_START]: {
    desc: 'Cluster Shrink Started',
    formatter: ({ hostname, ip, role }) => `Node ${hostname} (${ip}) with role ${role} is leaving the cluster`
  },
  [CodeEnum.OPERATION_SHRINK_COMPLETE]: {
    desc: 'Cluster Shrink Completed',
    formatter: ({ hostname, ip, role }) => `Node ${hostname} (${ip}) with role ${role} has left the cluster`
  },
  [CodeEnum.OPERATION_SHRINK_FAILURE]: {
    desc: 'Cluster Shrink Failed',
    formatter: ({ hostname, ip, role }) => `Node ${hostname} (${ip}) with role ${role} has failed to leave the cluster`
  },
  [CodeEnum.OPERATION_UNINSTALL_START]: {
    desc: 'Cluster Uninstall Started',
    formatter: () => `Cluster is being uninstalled`
  },
  [CodeEnum.OPERATION_UNINSTALL_COMPLETE]: {
    desc: 'Cluster Uninstall Completed',
    formatter: () => `Cluster has been uninstalled`
  },
  [CodeEnum.OPERATION_UNINSTALL_FAILURE]: {
    desc: 'Cluster Uninstall Failed',
    formatter: () => `Cluster uninstall has failed`
  },
  [CodeEnum.OPERATION_UPDATE_COMPLETE]: {
    desc: 'Cluster Update Completed',
    formatter: ({ version }) => `Cluster has been updated to version ${version}`
  },
  [CodeEnum.OPERATION_UPDATE_FAILURE]: {
    desc: 'Cluster Update Failed',
    formatter: ({ version }) => `Cluster has failed to update to version ${version}`
  },
  [CodeEnum.OPERATION_UPDATE_START]: {
    desc: 'Cluster Update Started',
    formatter: ({ version }) => `Cluster update to version ${version} has started`
  },
  [CodeEnum.PORTFORWARD]: {
    desc: 'Port Forwarding Started',
    formatter: ({ user }) => `User ${user} started port forwarding`
  },
  [CodeEnum.PORTFORWARD_FAILURE]: {
    desc: 'Port Forwarding Failed',
    formatter: ({ user, error }) => `User ${user} port forwarding request failed: ${error}`
  },
  [CodeEnum.REMOTE_SUPPORT_ENABLED]: {
    desc: 'Remote Support Enabled',
    formatter: ({ user, hub }) => `User ${user} enabled remote support with Gravity Hub ${hub}`
  },
  [CodeEnum.REMOTE_SUPPORT_DISABLED]: {
    desc: 'Remote Support Disabled',
    formatter: ({ user, hub }) => `User ${user} disabled remote support with Gravity Hub ${hub}`
  },
  [CodeEnum.ROLE_CREATED]: {
    desc: 'Role Created',
    formatter: ({ user, name }) => `User ${user} created role ${name}`
  },
  [CodeEnum.ROLE_DELETED]: {
    desc: 'Role Deleted',
    formatter: ({ user, name }) => `User ${user} deleted role ${name}`
  },
  [CodeEnum.SAML_CONNECTOR_CREATED]: {
    desc: 'SAML Connector Created',
    formatter: ({ user, name }) => `User ${user} created SAML connector ${name}`
  },
  [CodeEnum.SAML_CONNECTOR_DELETED]: {
    desc: 'SAML Connector Deleted',
    formatter: ({ user, name }) => `User ${user} deleted SAML connector ${name}`
  },
  [CodeEnum.SCP_DOWNLOAD]: {
    desc: 'SCP Download',
    formatter: ({ user, path, ...rest }) => `User ${user} downloaded a file ${path} from node ${rest["addr.local"]}`
  },
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: {
    desc: 'SCP Download Failed',
    formatter: ({ exitError, ...rest }) => `File download from node ${rest["addr.local"]} failed: ${exitError}`
  },
  [CodeEnum.SCP_UPLOAD]: {
    desc: 'SCP Upload',
    formatter: ({ user, path, ...rest }) => `User ${user} uploaded a file ${path} to node ${rest["addr.local"]}`
  },
  [CodeEnum.SCP_UPLOAD_FAILURE]: {
    desc: 'SCP Upload Failed',
    formatter: ({ exitError, ...rest }) => `File upload to node ${rest["addr.local"]} failed: ${exitError}`
  },
  [CodeEnum.SESSION_JOIN]: {
    desc: 'User Joined',
    formatter: ({ user, sid }) => `User ${user} has joined the session ${sid}`
  },
  [CodeEnum.SESSION_END]: {
    desc: 'Session Ended',
    formatter: ({ user, sid }) => `User ${user} has ended the session ${sid}`
  },
  [CodeEnum.SESSION_LEAVE]: {
    desc: 'User Disconnected',
    formatter: ({ user, sid }) => `User ${user} has left the session ${sid}`
  },
  [CodeEnum.SESSION_START]: {
    desc: 'Session Started',
    formatter: ({ user, sid }) => `User ${user} has started a session ${sid}`
  },
  [CodeEnum.SESSION_UPLOAD]: {
    desc: 'Session Uploaded',
    formatter: () => `Recorded session has been uploaded`
  },
  [CodeEnum.SMTPCONFIG_CREATED]: {
    desc: 'SMTP Config Created',
    formatter: ({ user }) => `User ${user} updated cluster SMTP configuration`
  },
  [CodeEnum.SMTPCONFIG_DELETED]: {
    desc: 'SMTP Config Deleted',
    formatter: ({ user }) => `User ${user} deleted cluster SMTP configuration`
  },
  [CodeEnum.SUBSYSTEM]: {
    desc: 'Subsystem Requested',
    formatter: ({ user, name }) => `User ${user} requested subsystem ${name}`
  },
  [CodeEnum.SUBSYSTEM_FAILURE]: {
    desc: 'Subsystem Request Failed',
    formatter: ({ user, name, exitError }) => `User ${user} subsystem ${name} request failed: ${exitError}`
  },
  [CodeEnum.TERMINAL_RESIZE]: {
    desc: 'Terminal Resize',
    formatter: ({ user }) => `User ${user} resized the terminal`
  },
  [CodeEnum.TLSKEYPAIR_CREATED]: {
    desc: 'TLS Keypair Created',
    formatter: ({ user }) => `User ${user} installed cluster web certificate`
  },
  [CodeEnum.TLSKEYPAIR_DELETED]: {
    desc: 'TLS Keypair Deleted',
    formatter: ({ user }) => `User ${user} deleted cluster web certificate`
  },
  [CodeEnum.TOKEN_CREATED]: {
    desc: 'User Token Created',
    formatter: ({ user, owner }) => `User ${user} created token for user ${owner}`
  },
  [CodeEnum.TOKEN_DELETED]: {
    desc: 'User Token Deleted',
    formatter: ({ user, owner }) => `User ${user} deleted token for user ${owner}`
  },
  [CodeEnum.UPDATES_ENABLED]: {
    desc: 'Periodic Updates Enabled',
    formatter: ({ user, hub }) => `User ${user} enabled periodic updates with Gravity Hub ${hub}`
  },
  [CodeEnum.UPDATES_DISABLED]: {
    desc: 'Periodic Updates Disabled',
    formatter: ({ user, hub }) => `User ${user} disabled periodic updates with Gravity Hub ${hub}`
  },
  [CodeEnum.UPDATES_DOWNLOADED]: {
    desc: 'Update Downloaded',
    formatter: ({ hub, name, version }) => `Downloaded new version ${name}:${version} from Gravity Hub ${hub}`
  },
  [CodeEnum.USER_CREATED]: {
    desc: 'User Created',
    formatter: ({ user, name }) => `User ${user} created user ${name}`
  },
  [CodeEnum.USER_DELETED]: {
    desc: 'User Deleted',
    formatter: ({ user, name }) => `User ${user} deleted user ${name}`
  },
  [CodeEnum.USER_INVITE_CREATED]: {
    desc: 'Invite Created',
    formatter: ({ user, name, roles }) => `User ${user} invited user ${name} with roles ${roles}`
  },
  [CodeEnum.USER_LOCAL_LOGIN]: {
    desc: 'Local Login',
    formatter: ({ user }) => `Local user ${user} successfully logged in`
  },
  [CodeEnum.USER_LOCAL_LOGINFAILURE]: {
    desc: 'Local Login Failed',
    formatter: ({ user, error }) => `Local user ${user} login failed: ${error}`
  },
  [CodeEnum.USER_SSO_LOGIN]: {
    desc: 'SSO Login',
    formatter: ({ user }) => `SSO user ${user} successfully logged in`
  },
  [CodeEnum.USER_SSO_LOGINFAILURE]: {
    desc: 'SSO Login Failed',
    formatter: ({ error }) => `SSO user login failed: ${error}`
  },
  [CodeEnum.ROLE_CREATED]: {
    desc: 'User Role Created',
    formatter: ({ user, name }) => `User ${user} created role ${name}`
  },
  [CodeEnum.ROLE_DELETED]: {
    desc: 'User Role Deleted',
    formatter: ({ user, name }) => `User ${user} deleted role ${name}`
  },
}

const unknownEvent = {
  desc: 'Unknown',
  formatter: () => 'Unknown'
}

export const SeverityEnum = {
  WARNING: 'warning',
  ERROR: 'error',
  INFO: 'info'
}

class Event {
  id = ''
  type = ''
  typeDesc = ''
  time = ''
  user = ''
  desc = ''
  details = {}

  constructor(json) {
    const cfg = eventConfig[json.code] || unknownEvent;
    this.id = getId(json);
    this.code = json.code;
    this.codeDesc = cfg.desc;
    this.message = cfg.formatter(json);
    this.user = json.user;
    this.time = new Date(json.time);
    this.details = json;
  }
}

// older events might not have an uid field.
// in this case compose it from other fields.
function getId(json) {
  const { uid, event, time } = json;
  if (uid) {
    return uid;
  }

  return `${event}:${time}`;
}

export default Event;