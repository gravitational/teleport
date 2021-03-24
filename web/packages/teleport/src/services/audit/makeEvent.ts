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

import moment from 'moment';
import { Event, CodeEnum, RawEvent, Formatters } from './types';

export const formatters: Formatters = {
  [CodeEnum.ACCESS_REQUEST_CREATED]: {
    desc: 'Access Request Created',
    format: ({ id, state }) =>
      `Access request [${id}] has been created and is ${state}`,
  },
  [CodeEnum.ACCESS_REQUEST_UPDATED]: {
    desc: 'Access Request Updated',
    format: ({ id, state }) =>
      `Access request [${id}] has been updated to ${state}`,
  },
  [CodeEnum.SESSION_COMMAND]: {
    desc: 'Session Command',
    format: ({ program, sid }) =>
      `Program [${program}] has been executed within a session [${sid}]`,
  },
  [CodeEnum.SESSION_DISK]: {
    desc: 'Session File Access',
    format: ({ path, sid, program }) =>
      `Program [${program}] accessed a file [${path}] within a session [${sid}]`,
  },
  [CodeEnum.SESSION_NETWORK]: {
    desc: 'Session Network Connection',
    format: ({ sid, program, src_addr, dst_addr, dst_port }) =>
      `Program [${program}] opened a connection [${src_addr} <-> ${dst_addr}:${dst_port}] within a session [${sid}]`,
  },
  [CodeEnum.SESSION_DATA]: {
    desc: 'Session Data',
    format: ({ sid }) =>
      `Usage report has been updated for session [${sid || ''}]`,
  },

  [CodeEnum.USER_PASSWORD_CHANGED]: {
    desc: 'User Password Updated',
    format: ({ user }) => `User [${user}] has changed a password`,
  },

  [CodeEnum.USER_UPDATED]: {
    desc: 'User Updated',
    format: ({ name }) => `User [${name}] has been updated`,
  },
  [CodeEnum.RESET_PASSWORD_TOKEN_CREATED]: {
    desc: 'Reset Password Token Created',
    format: ({ name, user }) =>
      `User [${user}] created a password reset token for user [${name}]`,
  },
  [CodeEnum.AUTH_ATTEMPT_FAILURE]: {
    desc: 'Auth Attempt Failed',
    format: ({ user, error }) => `User [${user}] failed auth attempt: ${error}`,
  },

  [CodeEnum.CLIENT_DISCONNECT]: {
    desc: 'Client Disconnected',
    format: ({ user, reason }) =>
      `User [${user}] has been disconnected: ${reason}`,
  },
  [CodeEnum.EXEC]: {
    desc: 'Command Execution',
    format: event => {
      const { proto, kubernetes_cluster, user = '' } = event;
      if (proto === 'kube') {
        if (!kubernetes_cluster) {
          return `User [${user}] executed a kubernetes command`;
        }
        return `User [${user}] executed a command on kubernetes cluster [${kubernetes_cluster}]`;
      }

      return `User [${user}] executed a command on node ${event['addr.local']}`;
    },
  },
  [CodeEnum.EXEC_FAILURE]: {
    desc: 'Command Execution Failed',
    format: ({ user, exitError, ...rest }) =>
      `User [${user}] command execution on node ${rest['addr.local']} failed [${exitError}]`,
  },
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: {
    desc: 'GITHUB Auth Connector Created',
    format: ({ user, name }) =>
      `User [${user}] created Github connector [${name}] has been created`,
  },
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: {
    desc: 'GITHUB Auth Connector Deleted',
    format: ({ user, name }) =>
      `User [${user}] deleted Github connector [${name}]`,
  },
  [CodeEnum.OIDC_CONNECTOR_CREATED]: {
    desc: 'OIDC Auth Connector Created',
    format: ({ user, name }) =>
      `User [${user}] created OIDC connector [${name}]`,
  },
  [CodeEnum.OIDC_CONNECTOR_DELETED]: {
    desc: 'OIDC Auth Connector Deleted',
    format: ({ user, name }) =>
      `User [${user}] deleted OIDC connector [${name}]`,
  },
  [CodeEnum.PORTFORWARD]: {
    desc: 'Port Forwarding Started',
    format: ({ user }) => `User [${user}] started port forwarding`,
  },
  [CodeEnum.PORTFORWARD_FAILURE]: {
    desc: 'Port Forwarding Failed',
    format: ({ user, error }) =>
      `User [${user}] port forwarding request failed: ${error}`,
  },
  [CodeEnum.SAML_CONNECTOR_CREATED]: {
    desc: 'SAML Connector Created',
    format: ({ user, name }) =>
      `User [${user}] created SAML connector [${name}]`,
  },
  [CodeEnum.SAML_CONNECTOR_DELETED]: {
    desc: 'SAML Connector Deleted',
    format: ({ user, name }) =>
      `User [${user}] deleted SAML connector [${name}]`,
  },
  [CodeEnum.SCP_DOWNLOAD]: {
    desc: 'SCP Download',
    format: ({ user, path, ...rest }) =>
      `User [${user}] downloaded a file [${path}] from node [${rest['addr.local']}]`,
  },
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: {
    desc: 'SCP Download Failed',
    format: ({ exitError, ...rest }) =>
      `File download from node [${rest['addr.local']}] failed [${exitError}]`,
  },
  [CodeEnum.SCP_UPLOAD]: {
    desc: 'SCP Upload',
    format: ({ user, path, ...rest }) =>
      `User [${user}] uploaded a file [${path}] to node [${rest['addr.local']}]`,
  },
  [CodeEnum.SCP_UPLOAD_FAILURE]: {
    desc: 'SCP Upload Failed',
    format: ({ exitError, ...rest }) =>
      `File upload to node [${rest['addr.local']}] failed [${exitError}]`,
  },
  [CodeEnum.SESSION_JOIN]: {
    desc: 'User Joined',
    format: ({ user, sid }) => `User [${user}] has joined the session [${sid}]`,
  },
  [CodeEnum.SESSION_END]: {
    desc: 'Session Ended',
    format: event => {
      const user = event.user || '';
      const node =
        event.server_hostname || event.server_addr || event.server_id;

      if (event.proto === 'kube') {
        if (!event.kubernetes_cluster) {
          return `User [${user}] has ended a kubernetes session [${event.sid}]`;
        }
        return `User [${user}] has ended a session [${event.sid}] on kubernetes cluster [${event.kubernetes_cluster}]`;
      }

      if (!event.interactive) {
        return `User [${user}] has ended a non-interactive session [${event.sid}] on node [${node}] `;
      }

      if (event.session_start && event.session_stop) {
        const duration = moment(event.session_stop).diff(event.session_start);
        const durationText = moment.duration(duration).humanize();
        return `User [${user}] has ended an interactive session lasting ${durationText} [${event.sid}] on node [${node}]`;
      }

      return `User [${user}] has ended interactive session [${event.sid}] on node [${node}] `;
    },
  },
  [CodeEnum.SESSION_REJECT]: {
    desc: 'Session Rejected',
    format: ({ user, login, server_id, reason }) =>
      `User [${user}] was denied access to [${login}@${server_id}] because [${reason}]`,
  },
  [CodeEnum.SESSION_LEAVE]: {
    desc: 'User Disconnected',
    format: ({ user, sid }) => `User [${user}] has left the session [${sid}]`,
  },
  [CodeEnum.SESSION_START]: {
    desc: 'Session Started',
    format: ({ user, sid }) => `User [${user}] has started a session [${sid}]`,
  },
  [CodeEnum.SESSION_UPLOAD]: {
    desc: 'Session Uploaded',
    format: () => `Recorded session has been uploaded`,
  },
  [CodeEnum.APP_SESSION_START]: {
    desc: 'App Session Started',
    format: ({ user, sid }) =>
      `User [${user}] has started an app session [${sid}]`,
  },
  [CodeEnum.APP_SESSION_CHUNK]: {
    desc: 'App Session Data',
    format: ({ sid }) => `New app session data created [${sid}]`,
  },
  [CodeEnum.SUBSYSTEM]: {
    desc: 'Subsystem Requested',
    format: ({ user, name }) => `User [${user}] requested subsystem [${name}]`,
  },
  [CodeEnum.SUBSYSTEM_FAILURE]: {
    desc: 'Subsystem Request Failed',
    format: ({ user, name, exitError }) =>
      `User [${user}] subsystem [${name}] request failed [${exitError}]`,
  },
  [CodeEnum.TERMINAL_RESIZE]: {
    desc: 'Terminal Resize',
    format: ({ user, sid }) =>
      `User [${user}] resized the session [${sid}] terminal`,
  },
  [CodeEnum.USER_CREATED]: {
    desc: 'User Created',
    format: ({ name }) => `User [${name}] has been created`,
  },
  [CodeEnum.USER_DELETED]: {
    desc: 'User Deleted',
    format: ({ name }) => `User [${name}] has been deleted`,
  },
  [CodeEnum.USER_LOCAL_LOGIN]: {
    desc: 'Local Login',
    format: ({ user }) => `Local user [${user}] successfully logged in`,
  },
  [CodeEnum.USER_LOCAL_LOGINFAILURE]: {
    desc: 'Local Login Failed',
    format: ({ user, error }) => `Local user [${user}] login failed [${error}]`,
  },
  [CodeEnum.USER_SSO_LOGIN]: {
    desc: 'SSO Login',
    format: ({ user }) => `SSO user [${user}] successfully logged in`,
  },
  [CodeEnum.USER_SSO_LOGINFAILURE]: {
    desc: 'SSO Login Failed',
    format: ({ error }) => `SSO user login failed [${error}]`,
  },
  [CodeEnum.ROLE_CREATED]: {
    desc: 'User Role Created',
    format: ({ user, name }) => `User [${user}] created a role [${name}]`,
  },
  [CodeEnum.ROLE_DELETED]: {
    desc: 'User Role Deleted',
    format: ({ user, name }) => `User [${user}] deleted a role [${name}]`,
  },
  [CodeEnum.TRUSTED_CLUSTER_TOKEN_CREATED]: {
    desc: 'Trusted Cluster Token Created',
    format: ({ user }) => `User [${user}] has created a trusted cluster token`,
  },
  [CodeEnum.TRUSTED_CLUSTER_CREATED]: {
    desc: 'Trusted Cluster Created',
    format: ({ user, name }) =>
      `User [${user}] has created a trusted relationship with cluster [${name}]`,
  },
  [CodeEnum.TRUSTED_CLUSTER_DELETED]: {
    desc: 'Trusted Cluster Deleted',
    format: ({ user, name }) =>
      `User [${user}] has deleted a trusted relationship with cluster [${name}]`,
  },
  [CodeEnum.KUBE_REQUEST]: {
    desc: 'Kubernetes Request',
    format: ({ user, kubernetes_cluster }) =>
      `User [${user}] made a request to kubernetes cluster [${kubernetes_cluster}]`,
  },
  [CodeEnum.DATABASE_SESSION_STARTED]: {
    desc: 'Database Session Started',
    format: ({ user, db_service, db_name, db_user }) =>
      `User [${user}] has connected to database [${db_name}] as [${db_user}] on [${db_service}]`,
  },
  [CodeEnum.DATABASE_SESSION_STARTED_FAILURE]: {
    desc: 'Database Session Denied',
    format: ({ user, db_service, db_name, db_user }) =>
      `User [${user}] was denied access to database [${db_name}] as [${db_user}] on [${db_service}]`,
  },
  [CodeEnum.DATABASE_SESSION_ENDED]: {
    desc: 'Database Session Ended',
    format: ({ user, db_service, db_name }) =>
      `User [${user}] has disconnected from database [${db_name}] on [${db_service}]`,
  },
  [CodeEnum.DATABASE_SESSION_QUERY]: {
    desc: 'Database Query',
    format: ({ user, db_service, db_name, db_query }) =>
      `User [${user}] has executed query [${truncateStr(
        db_query,
        80
      )}] in database [${db_name}] on [${db_service}]`,
  },
  [CodeEnum.MFA_DEVICE_ADD]: {
    desc: 'MFA Device Added',
    format: ({ user, mfa_device_name, mfa_device_type }) =>
      `User [${user}] added ${mfa_device_type} device [${mfa_device_name}]`,
  },
  [CodeEnum.MFA_DEVICE_DELETE]: {
    desc: 'MFA Device Deleted',
    format: ({ user, mfa_device_name, mfa_device_type }) =>
      `User [${user}] deleted ${mfa_device_type} device [${mfa_device_name}]`,
  },
  [CodeEnum.BILLING_CARD_CREATE]: {
    desc: 'Credit Card Added',
    format: ({ user }) => `User [${user}] has added a credit card`,
  },
  [CodeEnum.BILLING_CARD_DELETE]: {
    desc: 'Credit Card Deleted',
    format: ({ user }) => `User [${user}] has deleted a credit card`,
  },
  [CodeEnum.BILLING_CARD_UPDATE]: {
    desc: 'Credit Card Updated',
    format: ({ user }) => `User [${user}] has updated a credit card`,
  },
  [CodeEnum.BILLING_ACCOUNT_UPDATE]: {
    desc: 'Billing Information Updated',
    format: ({ user }) => `User [${user}] has updated the billing information`,
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

function truncateStr(str: string, len: number): string {
  if (str.length <= len) {
    return str;
  }
  return str.substring(0, len - 3) + '...';
}
