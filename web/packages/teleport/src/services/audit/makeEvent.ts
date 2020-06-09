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

import { Event, CodeEnum, RawEvent, Formatters } from './types';

export const formatters: Formatters = {
  [CodeEnum.ACCESS_REQUEST_CREATED]: {
    desc: 'Access Request Created',
    format: ({ id, state }) =>
      `Access request "${id}" has been created and is ${state}`,
  },

  [CodeEnum.ACCESS_REQUEST_UPDATED]: {
    desc: 'Access Request Updated',
    format: ({ id, state }) =>
      `Access request "${id}" has been updated to ${state}`,
  },

  [CodeEnum.G_SAML_CONNECTOR_CREATED]: {
    desc: 'SAML Connector Created',
    format: ({ user, name }) =>
      `User "${user}" created SAML connector "${name}"`,
  },

  [CodeEnum.G_SAML_CONNECTOR_DELETED]: {
    desc: 'SAML Connector Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted SAML connector "${name}"`,
  },

  [CodeEnum.SESSION_COMMAND]: {
    desc: 'Session Command',
    format: ({ program }) => `${program} has been executed`,
  },

  [CodeEnum.SESSION_DISK]: {
    desc: 'Session File Access',
    format: ({ path }) => `${path}`,
  },

  [CodeEnum.SESSION_NETWORK]: {
    desc: 'Session Network Connection',
    format: ({ sid, program, src_addr, dst_addr, dst_port }) =>
      `Session "${sid}" opened a connection ${program}: ${src_addr} <-> ${dst_addr}:${dst_port}`,
  },

  [CodeEnum.SESSION_DATA]: {
    desc: 'Session Report',
    format: ({ sid }) => `Report has been created for session "${sid}"`,
  },

  [CodeEnum.USER_PASSWORD_CHANGED]: {
    desc: 'User Password Updated',
    format: ({ user }) => `User "${user}" has changed a password`,
  },

  [CodeEnum.USER_UPDATED]: {
    desc: 'User Updated',
    format: ({ entity }) => `User ${entity} has been updated`,
  },

  [CodeEnum.RESET_PASSWORD_TOKEN_CREATED]: {
    desc: 'Reset Password Token Created',
    format: ({ entity, ttl, user }) =>
      `User "${user}" created a password reset token for user "${entity}" which expires in ${ttl}`,
  },

  [CodeEnum.G_ALERT_CREATED]: {
    desc: 'Alert Created',
    format: ({ user, name }) =>
      `User "${user}" created Monitoring Alert "${name}"`,
  },
  [CodeEnum.G_ALERT_DELETED]: {
    desc: 'Alert Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted monitoring alert "${name}"`,
  },
  [CodeEnum.G_ALERT_TARGET_CREATED]: {
    desc: 'Alert Target Created',
    format: ({ user }) => `User "${user}" updated monitoring alert target`,
  },
  [CodeEnum.G_ALERT_TARGET_DELETED]: {
    desc: 'Alert Target Deleted',
    format: ({ user }) => `User "${user}" deleted monitoring alert target`,
  },
  [CodeEnum.G_APPLICATION_INSTALL]: {
    desc: 'Application Installed',
    format: ({ releaseName, name, version }) =>
      `Application release ${releaseName} ("${name}":${version}) has been installed`,
  },
  [CodeEnum.G_APPLICATION_UPGRADE]: {
    desc: 'Application Upgraded',
    format: ({ releaseName, name, version }) =>
      `Application release ${releaseName} has been upgraded to "${name}":${version}`,
  },
  [CodeEnum.G_APPLICATION_ROLLBACK]: {
    desc: 'Application Rolledbacked',
    format: ({ releaseName, name, version }) =>
      `Application release ${releaseName} has been rolled back to "${name}":${version}`,
  },
  [CodeEnum.G_APPLICATION_UNINSTALL]: {
    desc: 'Application Uninstalled',
    format: ({ releaseName, name, version }) =>
      `Applicaiton release ${releaseName} ("${name}":${version}) has been uninstalled`,
  },
  [CodeEnum.AUTH_ATTEMPT_FAILURE]: {
    desc: 'Auth Attempt Failed',
    format: ({ user, error }) => `User "${user}" failed auth attempt: ${error}`,
  },
  [CodeEnum.G_AUTHGATEWAY_UPDATED]: {
    desc: 'Auth Gateway Updated',
    format: ({ user }) =>
      `User "${user}" updated cluster authentication gateway settings`,
  },
  [CodeEnum.G_AUTHPREFERENCE_UPDATED]: {
    desc: 'Auth Preferences Updated',
    format: ({ user }) =>
      `User "${user}" updated cluster authentication preference`,
  },

  [CodeEnum.CLIENT_DISCONNECT]: {
    desc: 'Client Disconnected',
    format: ({ user, reason }) =>
      `User "${user}" has been disconnected: ${reason}`,
  },
  [CodeEnum.G_CLUSTER_HEALTHY]: {
    desc: 'Cluster Healthy',
    format: () => `Cluster has become healthy`,
  },
  [CodeEnum.G_CLUSTER_UNHEALTHY]: {
    desc: 'Cluster Unhealthy',
    format: ({ reason }) => `Cluster is degraded: ${reason}`,
  },
  [CodeEnum.G_ENDPOINTS_UPDATED]: {
    desc: 'Endpoints Updated',
    format: ({ user }) => `User "${user}" updated Ops Center endpoints`,
  },
  [CodeEnum.EXEC]: {
    desc: 'Command Execution',
    format: ({ user, ...rest }) =>
      `User "${user}" executed a command on node ${rest['addr.local']}`,
  },
  [CodeEnum.EXEC_FAILURE]: {
    desc: 'Command Execution Failed',
    format: ({ user, exitError, ...rest }) =>
      `User "${user}" command execution on node ${rest['addr.local']} failed: ${exitError}`,
  },
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: {
    desc: 'GITHUB Auth Connector Created',
    format: ({ user, name }) =>
      `User "${user}" created Github connector "${name}" has been created`,
  },
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: {
    desc: 'GITHUB Auth Connector Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted Github connector "${name}"`,
  },
  [CodeEnum.G_GITHUB_CONNECTOR_CREATED]: {
    desc: 'GITHUB Auth Connector Created',
    format: ({ user, name }) =>
      `User "${user}" created Github connector "${name}"`,
  },
  [CodeEnum.G_GITHUB_CONNECTOR_DELETED]: {
    desc: 'GITHUB Auth Connector Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted Github connector "${name}"`,
  },
  [CodeEnum.G_LICENSE_GENERATED]: {
    desc: 'Cluster License Generated',
    format: ({ maxNodes }) =>
      `License for max nodes ${maxNodes} has been generated`,
  },
  [CodeEnum.G_LICENSE_EXPIRED]: {
    desc: 'Cluster License Expired',
    format: () => `Cluster license has expired`,
  },
  [CodeEnum.G_LICENSE_UPDATED]: {
    desc: 'Cluster License Updated',
    format: () => `Cluster license has been updated`,
  },
  [CodeEnum.G_LOGFORWARDER_CREATED]: {
    desc: 'Log Forwarder Created',
    format: ({ user, name }) =>
      `User "${user}" created log forwarder "${name}"`,
  },
  [CodeEnum.G_LOGFORWARDER_DELETED]: {
    desc: 'Log Forwarder Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted log forwarder "${name}"`,
  },
  [CodeEnum.OIDC_CONNECTOR_CREATED]: {
    desc: 'OIDC Auth Connector Created',
    format: ({ user, name }) =>
      `User "${user}" created OIDC connector "${name}"`,
  },
  [CodeEnum.OIDC_CONNECTOR_DELETED]: {
    desc: 'OIDC Auth Connector Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted OIDC connector "${name}"`,
  },
  [CodeEnum.G_OIDC_CONNECTOR_CREATED]: {
    desc: 'OIDC Auth Connector Created',
    format: ({ user, name }) =>
      `User "${user}" created OIDC connector "${name}"`,
  },
  [CodeEnum.G_OIDC_CONNECTOR_DELETED]: {
    desc: 'OIDC Auth Connector Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted OIDC connector "${name}"`,
  },
  [CodeEnum.G_OPERATION_CONFIG_COMPLETE]: {
    desc: 'Cluster Configuration Completed',
    format: () => `Cluster configuration has been updated`,
  },
  [CodeEnum.G_OPERATION_CONFIG_FAILURE]: {
    desc: 'Cluster Configuration Failed',
    format: () => `Failed to update the cluster configuration`,
  },
  [CodeEnum.G_OPERATION_CONFIG_START]: {
    desc: 'Cluster Configuration Started',
    format: () => `Updating the cluster configuration`,
  },
  [CodeEnum.G_OPERATION_ENV_COMPLETE]: {
    desc: 'Environment Update Completed',
    format: () => `Cluster runtime environment has been updated`,
  },
  [CodeEnum.G_OPERATION_ENV_FAILURE]: {
    desc: 'Environment Update Failed',
    format: () => `Failed to update the cluster runtime environment`,
  },
  [CodeEnum.G_OPERATION_ENV_START]: {
    desc: 'Environment Update Started',
    format: () => `Updating the cluster runtime environment`,
  },
  [CodeEnum.G_OPERATION_EXPAND_START]: {
    desc: 'Cluster Expand Started',
    format: ({ hostname, ip, role }) =>
      `Node ${hostname} (${ip}) with role ${role} is joining the cluster`,
  },
  [CodeEnum.G_OPERATION_EXPAND_COMPLETE]: {
    desc: 'Cluster Expand Completed',
    format: ({ hostname, ip, role }) =>
      `Node ${hostname} (${ip}) with role ${role} has joined the cluster`,
  },
  [CodeEnum.G_OPERATION_EXPAND_FAILURE]: {
    desc: 'Cluster Expand Failed',
    format: ({ hostname, ip, role }) =>
      `Node ${hostname} (${ip}) with role ${role} has failed to join the cluster`,
  },
  [CodeEnum.G_OPERATION_GC_START]: {
    desc: 'GC Started',
    format: () => 'Running garbage collection on the cluster',
  },
  [CodeEnum.G_OPERATION_GC_COMPLETE]: {
    desc: 'GC Completed',
    format: () => 'Garbage collection on the cluster has finished',
  },
  [CodeEnum.G_OPERATION_GC_FAILURE]: {
    desc: 'GC Failed',
    format: () => 'Garbage collection on the cluster has failed',
  },
  [CodeEnum.G_OPERATION_INSTALL_START]: {
    desc: 'Cluster Install Started',
    format: ({ cluster }) => `Cluster ${cluster} is being installed`,
  },
  [CodeEnum.G_OPERATION_INSTALL_COMPLETE]: {
    desc: 'Cluster Install Completed',
    format: ({ cluster }) => `Cluster ${cluster} has been installed`,
  },
  [CodeEnum.G_OPERATION_INSTALL_FAILURE]: {
    desc: 'Cluster Install Failed',
    format: ({ cluster }) => `Cluster ${cluster} install has failed`,
  },
  [CodeEnum.G_OPERATION_SHRINK_START]: {
    desc: 'Cluster Shrink Started',
    format: ({ hostname, ip, role }) =>
      `Node ${hostname} (${ip}) with role ${role} is leaving the cluster`,
  },
  [CodeEnum.G_OPERATION_SHRINK_COMPLETE]: {
    desc: 'Cluster Shrink Completed',
    format: ({ hostname, ip, role }) =>
      `Node ${hostname} (${ip}) with role ${role} has left the cluster`,
  },
  [CodeEnum.G_OPERATION_SHRINK_FAILURE]: {
    desc: 'Cluster Shrink Failed',
    format: ({ hostname, ip, role }) =>
      `Node ${hostname} (${ip}) with role ${role} has failed to leave the cluster`,
  },
  [CodeEnum.G_OPERATION_UNINSTALL_START]: {
    desc: 'Cluster Uninstall Started',
    format: () => `Cluster is being uninstalled`,
  },
  [CodeEnum.G_OPERATION_UNINSTALL_COMPLETE]: {
    desc: 'Cluster Uninstall Completed',
    format: () => `Cluster has been uninstalled`,
  },
  [CodeEnum.G_OPERATION_UNINSTALL_FAILURE]: {
    desc: 'Cluster Uninstall Failed',
    format: () => `Cluster uninstall has failed`,
  },
  [CodeEnum.G_OPERATION_UPDATE_COMPLETE]: {
    desc: 'Cluster Update Completed',
    format: ({ version }) => `Cluster has been updated to version ${version}`,
  },
  [CodeEnum.G_OPERATION_UPDATE_FAILURE]: {
    desc: 'Cluster Update Failed',
    format: ({ version }) =>
      `Cluster has failed to update to version ${version}`,
  },
  [CodeEnum.G_OPERATION_UPDATE_START]: {
    desc: 'Cluster Update Started',
    format: ({ version }) => `Cluster update to version ${version} has started`,
  },
  [CodeEnum.PORTFORWARD]: {
    desc: 'Port Forwarding Started',
    format: ({ user }) => `User "${user}" started port forwarding`,
  },
  [CodeEnum.PORTFORWARD_FAILURE]: {
    desc: 'Port Forwarding Failed',
    format: ({ user, error }) =>
      `User "${user}" port forwarding request failed: ${error}`,
  },
  [CodeEnum.G_REMOTE_SUPPORT_ENABLED]: {
    desc: 'Remote Support Enabled',
    format: ({ user, hub }) =>
      `User "${user}" enabled remote support with Gravity Hub ${hub}`,
  },
  [CodeEnum.G_REMOTE_SUPPORT_DISABLED]: {
    desc: 'Remote Support Disabled',
    format: ({ user, hub }) =>
      `User "${user}" disabled remote support with Gravity Hub ${hub}`,
  },
  [CodeEnum.G_ROLE_CREATED]: {
    desc: 'Role Created',
    format: ({ user, name }) => `User "${user}" created role "${name}"`,
  },
  [CodeEnum.G_ROLE_DELETED]: {
    desc: 'Role Deleted',
    format: ({ user, name }) => `User "${user}" deleted role "${name}"`,
  },
  [CodeEnum.SAML_CONNECTOR_CREATED]: {
    desc: 'SAML Connector Created',
    format: ({ user, name }) =>
      `User "${user}" created SAML connector "${name}"`,
  },
  [CodeEnum.SAML_CONNECTOR_DELETED]: {
    desc: 'SAML Connector Deleted',
    format: ({ user, name }) =>
      `User "${user}" deleted SAML connector "${name}"`,
  },
  [CodeEnum.SCP_DOWNLOAD]: {
    desc: 'SCP Download',
    format: ({ user, path, ...rest }) =>
      `User "${user}" downloaded a file ${path} from node ${rest['addr.local']}`,
  },
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: {
    desc: 'SCP Download Failed',
    format: ({ exitError, ...rest }) =>
      `File download from node ${rest['addr.local']} failed: ${exitError}`,
  },
  [CodeEnum.SCP_UPLOAD]: {
    desc: 'SCP Upload',
    format: ({ user, path, ...rest }) =>
      `User "${user}" uploaded a file ${path} to node ${rest['addr.local']}`,
  },
  [CodeEnum.SCP_UPLOAD_FAILURE]: {
    desc: 'SCP Upload Failed',
    format: ({ exitError, ...rest }) =>
      `File upload to node ${rest['addr.local']} failed: ${exitError}`,
  },
  [CodeEnum.SESSION_JOIN]: {
    desc: 'User Joined',
    format: ({ user, sid }) => `User "${user}" has joined the session "${sid}"`,
  },
  [CodeEnum.SESSION_END]: {
    desc: 'Session Ended',
    format: ({ user, sid }) => `User "${user}" has ended the session "${sid}"`,
  },
  [CodeEnum.SESSION_LEAVE]: {
    desc: 'User Disconnected',
    format: ({ user, sid }) => `User "${user}" has left the session "${sid}"`,
  },
  [CodeEnum.SESSION_START]: {
    desc: 'Session Started',
    format: ({ user, sid }) => `User "${user}" has started a session "${sid}"`,
  },
  [CodeEnum.SESSION_UPLOAD]: {
    desc: 'Session Uploaded',
    format: () => `Recorded session has been uploaded`,
  },
  [CodeEnum.G_SMTPCONFIG_CREATED]: {
    desc: 'SMTP Config Created',
    format: ({ user }) => `User "${user}" updated cluster SMTP configuration`,
  },
  [CodeEnum.G_SMTPCONFIG_DELETED]: {
    desc: 'SMTP Config Deleted',
    format: ({ user }) => `User "${user}" deleted cluster SMTP configuration`,
  },
  [CodeEnum.SUBSYSTEM]: {
    desc: 'Subsystem Requested',
    format: ({ user, name }) => `User "${user}" requested subsystem "${name}"`,
  },
  [CodeEnum.SUBSYSTEM_FAILURE]: {
    desc: 'Subsystem Request Failed',
    format: ({ user, name, exitError }) =>
      `User "${user}" subsystem "${name}" request failed: ${exitError}`,
  },
  [CodeEnum.TERMINAL_RESIZE]: {
    desc: 'Terminal Resize',
    format: ({ user }) => `User "${user}" resized the terminal`,
  },
  [CodeEnum.G_TLSKEYPAIR_CREATED]: {
    desc: 'TLS Keypair Created',
    format: ({ user }) => `User "${user}" installed cluster web certificate`,
  },
  [CodeEnum.G_TLSKEYPAIR_DELETED]: {
    desc: 'TLS Keypair Deleted',
    format: ({ user }) => `User "${user}" deleted cluster web certificate`,
  },
  [CodeEnum.G_TOKEN_CREATED]: {
    desc: 'User Token Created',
    format: ({ user, owner }) =>
      `User "${user}" created token for user ${owner}`,
  },
  [CodeEnum.G_TOKEN_DELETED]: {
    desc: 'User Token Deleted',
    format: ({ user, owner }) =>
      `User "${user}" deleted token for user ${owner}`,
  },
  [CodeEnum.G_UPDATES_ENABLED]: {
    desc: 'Periodic Updates Enabled',
    format: ({ user, hub }) =>
      `User "${user}" enabled periodic updates with Gravity Hub ${hub}`,
  },
  [CodeEnum.G_UPDATES_DISABLED]: {
    desc: 'Periodic Updates Disabled',
    format: ({ user, hub }) =>
      `User "${user}" disabled periodic updates with Gravity Hub ${hub}`,
  },
  [CodeEnum.G_UPDATES_DOWNLOADED]: {
    desc: 'Update Downloaded',
    format: ({ hub, name, version }) =>
      `Downloaded new version "${name}":${version} from Gravity Hub ${hub}`,
  },
  [CodeEnum.USER_CREATED]: {
    desc: 'User Created',
    format: ({ entity }) => `User ${entity} has been created`,
  },
  [CodeEnum.USER_DELETED]: {
    desc: 'User Deleted',
    format: ({ entity }) => `User ${entity} has been deleted`,
  },
  [CodeEnum.G_USER_CREATED]: {
    desc: 'User Created',
    format: ({ user, name }) => `User "${user}" created user "${name}"`,
  },
  [CodeEnum.G_USER_DELETED]: {
    desc: 'User Deleted',
    format: ({ user, name }) => `User "${user}" deleted user "${name}"`,
  },
  [CodeEnum.G_USER_INVITE_CREATED]: {
    desc: 'Invite Created',
    format: ({ user, name, roles }) =>
      `User "${user}" invited user "${name}" with roles ${roles}`,
  },
  [CodeEnum.USER_LOCAL_LOGIN]: {
    desc: 'Local Login',
    format: ({ user }) => `Local user "${user}" successfully logged in`,
  },
  [CodeEnum.USER_LOCAL_LOGINFAILURE]: {
    desc: 'Local Login Failed',
    format: ({ user, error }) => `Local user "${user}" login failed: ${error}`,
  },
  [CodeEnum.USER_SSO_LOGIN]: {
    desc: 'SSO Login',
    format: ({ user }) => `SSO user "${user}" successfully logged in`,
  },
  [CodeEnum.USER_SSO_LOGINFAILURE]: {
    desc: 'SSO Login Failed',
    format: ({ error }) => `SSO user login failed: ${error}`,
  },
  [CodeEnum.ROLE_CREATED]: {
    desc: 'User Role Created',
    format: ({ user, name }) => `User "${user}" created role "${name}"`,
  },
  [CodeEnum.ROLE_DELETED]: {
    desc: 'User Role Deleted',
    format: ({ user, name }) => `User "${user}" deleted role "${name}"`,
  },
};

const unknownFormatter = {
  desc: 'Unknown',
  format: () => 'Unknown',
};

export default function makeEvent(json: any): Event {
  // lookup event formatter by code
  const formatter = formatters[json.code] || unknownFormatter;
  const event = {
    codeDesc: formatter.desc,
    message: formatter.format(json as any),
    id: getId(json),
    code: json.code,
    user: json.user,
    time: new Date(json.time),
    raw: json,
  };

  return event;
}

// older events might not have an uid field.
// in this case compose it from other fields.
function getId(json: RawEvent<any>) {
  const { uid, event, time } = json;
  if (uid) {
    return uid;
  }

  return `${event}:${time}`;
}
