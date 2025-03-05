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

import { formatDistanceStrict } from 'date-fns';

import { pluralize } from 'shared/utils/text';

import {
  Event,
  EventCode,
  eventCodes,
  Formatters,
  RawEvent,
  RawEvents,
} from './types';

const formatElasticsearchEvent: (
  json:
    | RawEvents[typeof eventCodes.ELASTICSEARCH_REQUEST]
    | RawEvents[typeof eventCodes.ELASTICSEARCH_REQUEST_FAILURE]
    | RawEvents[typeof eventCodes.OPENSEARCH_REQUEST]
    | RawEvents[typeof eventCodes.OPENSEARCH_REQUEST_FAILURE]
) => string = ({ category, code, db_service, path, query, target, user }) => {
  // local redefinition of enum from events.proto.
  // currently this matches both OpenSearchCategory and ElasticsearchCategory.
  enum Category {
    GENERAL = 0,
    SECURITY = 1,
    SEARCH = 2,
    SQL = 3,
  }

  const categoryString = Category[category] ?? 'UNKNOWN';

  let message = '';

  switch (code) {
    case eventCodes.ELASTICSEARCH_REQUEST:
    case eventCodes.OPENSEARCH_REQUEST:
      message += `User [${user}] has ran a [${categoryString}] query in [${db_service}], request path: [${path}]`;
      break;

    case eventCodes.ELASTICSEARCH_REQUEST_FAILURE:
    case eventCodes.OPENSEARCH_REQUEST_FAILURE:
      message += `User [${user}] has attempted to run a [${categoryString}] query in [${db_service}], request path: [${path}]`;
      break;
  }

  if (query) {
    message += `, query string: [${truncateStr(query, 80)}]`;
  }

  if (target) {
    message += `, target: [${target}]`;
  }

  return message;
};

const portForwardEventTypes = [
  'port',
  'port.local',
  'port.remote',
  'port.remote_conn',
] as const;
type PortForwardEventType = (typeof portForwardEventTypes)[number];
type PortForwardEvent =
  | RawEvents[typeof eventCodes.PORTFORWARD]
  | RawEvents[typeof eventCodes.PORTFORWARD_STOP]
  | RawEvents[typeof eventCodes.PORTFORWARD_FAILURE];

const getPortForwardEventName = (event: string): string => {
  let ev = event as PortForwardEventType;
  if (!portForwardEventTypes.includes(ev)) {
    ev = 'port'; // default to generic 'port' if the event type is unknown
  }

  switch (ev) {
    case 'port':
      return 'Port Forwarding';
    case 'port.local':
      return 'Local Port Forwarding';
    case 'port.remote':
      return 'Remote Port Forwarding';
    case 'port.remote_conn':
      return 'Remote Port Forwarded Connection';
  }
};

const formatPortForwardEvent = ({
  user,
  code,
  event,
}: PortForwardEvent): string => {
  const eventName = getPortForwardEventName(event).toLowerCase();

  switch (code) {
    case eventCodes.PORTFORWARD:
      return `User [${user}] started ${eventName}`;
    case eventCodes.PORTFORWARD_STOP:
      return `User [${user}] stopped ${eventName}`;
    case eventCodes.PORTFORWARD_FAILURE:
      return `User [${user}] failed ${eventName}`;
  }
};

const describePortForwardEvent = ({ code, event }: PortForwardEvent) => {
  const eventName = getPortForwardEventName(event);

  switch (code) {
    case eventCodes.PORTFORWARD:
      return `${eventName} Start`;
    case eventCodes.PORTFORWARD_STOP:
      return `${eventName} Stop`;
    case eventCodes.PORTFORWARD_FAILURE:
      return `${eventName} Failure`;
  }
};

export const formatters: Formatters = {
  [eventCodes.ACCESS_REQUEST_CREATED]: {
    type: 'access_request.create',
    desc: 'Access Request Created',
    format: ({ id, state }) =>
      `Access request [${id}] has been created and is ${state}`,
  },
  [eventCodes.ACCESS_REQUEST_UPDATED]: {
    type: 'access_request.update',
    desc: 'Access Request Updated',
    format: ({ id, state }) =>
      `Access request [${id}] has been updated to ${state}`,
  },
  [eventCodes.ACCESS_REQUEST_REVIEWED]: {
    type: 'access_request.review',
    desc: 'Access Request Reviewed',
    format: ({ id, reviewer, state }) => {
      return `User [${reviewer}] ${state.toLowerCase()} access request [${id}]`;
    },
  },
  [eventCodes.ACCESS_REQUEST_DELETED]: {
    type: 'access_request.delete',
    desc: 'Access Request Deleted',
    format: ({ id }) => `Access request [${id}] has been deleted`,
  },
  [eventCodes.ACCESS_REQUEST_RESOURCE_SEARCH]: {
    type: 'access_request.search',
    desc: 'Resource Access Search',
    format: ({ user, resource_type, search_as_roles }) =>
      `User [${user}] searched for resource type [${resource_type}] with role(s) [${truncateStr(search_as_roles.join(','), 80)}]`,
  },
  [eventCodes.SESSION_COMMAND]: {
    type: 'session.command',
    desc: 'Session Command',
    format: ({ program, sid }) =>
      `Program [${program}] has been executed within a session [${sid}]`,
  },
  [eventCodes.SESSION_DISK]: {
    type: 'session.disk',
    desc: 'Session File Access',
    format: ({ path, sid, program }) =>
      `Program [${program}] accessed a file [${path}] within a session [${sid}]`,
  },
  [eventCodes.SESSION_NETWORK]: {
    type: 'session.network',
    desc: 'Session Network Connection',
    format: ({ action, sid, program, src_addr, dst_addr, dst_port }) => {
      const a = action === 1 ? '[DENY]' : '[ALLOW]';
      const desc =
        action === 1 ? 'was prevented from opening' : 'successfully opened';
      return `${a} Program [${program}] ${desc} a connection [${src_addr} <-> ${dst_addr}:${dst_port}] within a session [${sid}]`;
    },
  },
  [eventCodes.SESSION_PROCESS_EXIT]: {
    type: 'session.process_exit',
    desc: 'Session Process Exit',
    format: ({ program, exit_status, sid }) =>
      `Program [${program}] has exited with status ${exit_status}, within a session [${sid}]`,
  },
  [eventCodes.SESSION_DATA]: {
    type: 'session.data',
    desc: 'Session Data',
    format: ({ sid }) =>
      `Usage report has been updated for session [${sid || ''}]`,
  },

  [eventCodes.USER_PASSWORD_CHANGED]: {
    type: 'user.password_change',
    desc: 'User Password Updated',
    format: ({ user }) => `User [${user}] has changed a password`,
  },

  [eventCodes.USER_UPDATED]: {
    type: 'user.update',
    desc: 'User Updated',
    format: ({ name }) => `User [${name}] has been updated`,
  },
  [eventCodes.RESET_PASSWORD_TOKEN_CREATED]: {
    type: 'reset_password_token.create',
    desc: 'Reset Password Token Created',
    format: ({ name, user }) =>
      `User [${user}] created a password reset token for user [${name}]`,
  },
  [eventCodes.AUTH_ATTEMPT_FAILURE]: {
    type: 'auth',
    desc: 'Auth Attempt Failed',
    format: ({ user, error }) => `User [${user}] failed auth attempt: ${error}`,
  },

  [eventCodes.CLIENT_DISCONNECT]: {
    type: 'client.disconnect',
    desc: 'Client Disconnected',
    format: ({ user, reason }) =>
      `User [${user}] has been disconnected: ${reason}`,
  },
  [eventCodes.EXEC]: {
    type: 'exec',
    desc: 'Command Execution',
    format: event => {
      const { proto, kubernetes_cluster, user = '' } = event;
      if (proto === 'kube') {
        if (!kubernetes_cluster) {
          return `User [${user}] executed a Kubernetes command`;
        }
        return `User [${user}] executed a command on Kubernetes cluster [${kubernetes_cluster}]`;
      }

      return `User [${user}] executed a command on node ${
        event['server_hostname'] || event['addr.local']
      }`;
    },
  },
  [eventCodes.EXEC_FAILURE]: {
    type: 'exec',
    desc: 'Command Execution Failed',
    format: ({ user, exitError, ...rest }) =>
      `User [${user}] command execution on node ${
        rest['server_hostname'] || rest['addr.local']
      } failed [${exitError}]`,
  },
  [eventCodes.GITHUB_CONNECTOR_CREATED]: {
    type: 'github.created',
    desc: 'GitHub Auth Connector Created',
    format: ({ user, name }) =>
      `User [${user}] created GitHub connector [${name}]`,
  },
  [eventCodes.GITHUB_CONNECTOR_DELETED]: {
    type: 'github.deleted',
    desc: 'GitHub Auth Connector Deleted',
    format: ({ user, name }) =>
      `User [${user}] deleted GitHub connector [${name}]`,
  },
  [eventCodes.GITHUB_CONNECTOR_UPDATED]: {
    type: 'github.updated',
    desc: 'GitHub Auth Connector Updated',
    format: ({ user, name }) =>
      `User [${user}] updated GitHub connector [${name}]`,
  },
  [eventCodes.OIDC_CONNECTOR_CREATED]: {
    type: 'oidc.created',
    desc: 'OIDC Auth Connector Created',
    format: ({ user, name }) =>
      `User [${user}] created OIDC connector [${name}]`,
  },
  [eventCodes.OIDC_CONNECTOR_DELETED]: {
    type: 'oidc.deleted',
    desc: 'OIDC Auth Connector Deleted',
    format: ({ user, name }) =>
      `User [${user}] deleted OIDC connector [${name}]`,
  },
  [eventCodes.OIDC_CONNECTOR_UPDATED]: {
    type: 'oidc.updated',
    desc: 'OIDC Auth Connector Updated',
    format: ({ user, name }) =>
      `User [${user}] updated OIDC connector [${name}]`,
  },
  [eventCodes.PORTFORWARD]: {
    type: 'port',
    desc: describePortForwardEvent,
    format: formatPortForwardEvent,
  },
  [eventCodes.PORTFORWARD_FAILURE]: {
    type: 'port',
    desc: describePortForwardEvent,
    format: formatPortForwardEvent,
  },
  [eventCodes.PORTFORWARD_STOP]: {
    type: 'port',
    desc: describePortForwardEvent,
    format: formatPortForwardEvent,
  },
  [eventCodes.SAML_CONNECTOR_CREATED]: {
    type: 'saml.created',
    desc: 'SAML Connector Created',
    format: ({ user, name }) =>
      `User [${user}] created SAML connector [${name}]`,
  },
  [eventCodes.SAML_CONNECTOR_DELETED]: {
    type: 'saml.deleted',
    desc: 'SAML Connector Deleted',
    format: ({ user, name }) =>
      `User [${user}] deleted SAML connector [${name}]`,
  },
  [eventCodes.SAML_CONNECTOR_UPDATED]: {
    type: 'saml.updated',
    desc: 'SAML Connector Updated',
    format: ({ user, name }) =>
      `User [${user}] updated SAML connector [${name}]`,
  },
  [eventCodes.SCP_DOWNLOAD]: {
    type: 'scp',
    desc: 'SCP Download',
    format: ({ user, path, ...rest }) =>
      `User [${user}] downloaded a file [${path}] from node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SCP_DOWNLOAD_FAILURE]: {
    type: 'scp',
    desc: 'SCP Download Failed',
    format: ({ exitError, ...rest }) =>
      `File download from node [${
        rest['server_hostname'] || rest['addr.local']
      }] failed [${exitError}]`,
  },
  [eventCodes.SCP_UPLOAD]: {
    type: 'scp',
    desc: 'SCP Upload',
    format: ({ user, path, ...rest }) =>
      `User [${user}] uploaded a file to [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SCP_UPLOAD_FAILURE]: {
    type: 'scp',
    desc: 'SCP Upload Failed',
    format: ({ exitError, ...rest }) =>
      `File upload to node [${
        rest['server_hostname'] || rest['addr.local']
      }] failed [${exitError}]`,
  },
  [eventCodes.SCP_DISALLOWED]: {
    type: 'scp',
    desc: 'SCP Disallowed',
    format: ({ user, ...rest }) =>
      `User [${user}] SCP file transfer on node [${
        rest['server_hostname'] || rest['addr.local']
      }] blocked`,
  },
  [eventCodes.SFTP_OPEN]: {
    type: 'sftp',
    desc: 'SFTP Open',
    format: ({ user, path, ...rest }) =>
      `User [${user}] opened file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_OPEN_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Open Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to open file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_SETSTAT]: {
    type: 'sftp',
    desc: 'SFTP Setstat',
    format: ({ user, path, ...rest }) =>
      `User [${user}] changed attributes of file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_SETSTAT_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Setstat Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to change attributes of file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_OPENDIR]: {
    type: 'sftp',
    desc: 'SFTP Opendir',
    format: ({ user, path, ...rest }) =>
      `User [${user}] opened directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_OPENDIR_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Opendir Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to open directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_READDIR]: {
    type: 'sftp',
    desc: 'SFTP Readdir',
    format: ({ user, path, ...rest }) =>
      `User [${user}] read directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_READDIR_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Readdir Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to read directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_REMOVE]: {
    type: 'sftp',
    desc: 'SFTP Remove',
    format: ({ user, path, ...rest }) =>
      `User [${user}] removed file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_REMOVE_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Remove Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to remove file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_MKDIR]: {
    type: 'sftp',
    desc: 'SFTP Mkdir',
    format: ({ user, path, ...rest }) =>
      `User [${user}] created directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_MKDIR_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Mkdir Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to create directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_RMDIR]: {
    type: 'sftp',
    desc: 'SFTP Rmdir',
    format: ({ user, path, ...rest }) =>
      `User [${user}] removed directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_RMDIR_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Rmdir Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to remove directory [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_RENAME]: {
    type: 'sftp',
    desc: 'SFTP Rename',
    format: ({ user, path, ...rest }) =>
      `User [${user}] renamed file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_RENAME_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Rename Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to rename file [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_SYMLINK]: {
    type: 'sftp',
    desc: 'SFTP Symlink',
    format: ({ user, path, ...rest }) =>
      `User [${user}] created symbolic link [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_SYMLINK_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Symlink Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to create symbolic link [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_LINK]: {
    type: 'sftp',
    desc: 'SFTP Link',
    format: ({ user, path, ...rest }) =>
      `User [${user}] created hard link [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SFTP_LINK_FAILURE]: {
    type: 'sftp',
    desc: 'SFTP Link Failed',
    format: ({ user, path, error, ...rest }) =>
      `User [${user}] failed to create hard link [${path}] on node [${
        rest['server_hostname'] || rest['addr.local']
      }]: [${error}]`,
  },
  [eventCodes.SFTP_DISALLOWED]: {
    type: 'sftp',
    desc: 'SFTP Disallowed',
    format: ({ user, ...rest }) =>
      `User [${user}] was blocked from creating an SFTP session on node [${
        rest['server_hostname'] || rest['addr.local']
      }]`,
  },
  [eventCodes.SESSION_JOIN]: {
    type: 'session.join',
    desc: 'User Joined',
    format: ({ user, sid }) => `User [${user}] has joined the session [${sid}]`,
  },
  [eventCodes.SESSION_END]: {
    type: 'session.end',
    desc: 'Session Ended',
    format: event => {
      const user = event.user || '';
      const node =
        event.server_hostname || event.server_addr || event.server_id;

      if (event.proto === 'kube') {
        if (!event.kubernetes_cluster) {
          return `User [${user}] has ended a Kubernetes session [${event.sid}]`;
        }
        return `User [${user}] has ended a session [${event.sid}] on Kubernetes cluster [${event.kubernetes_cluster}]`;
      }

      if (!event.interactive) {
        return `User [${user}] has ended a non-interactive session [${event.sid}] on node [${node}] `;
      }

      if (event.session_start && event.session_stop) {
        const start = new Date(event.session_start);
        const end = new Date(event.session_stop);
        const durationText = formatDistanceStrict(start, end);
        return `User [${user}] has ended an interactive session lasting ${durationText} [${event.sid}] on node [${node}]`;
      }

      return `User [${user}] has ended interactive session [${event.sid}] on node [${node}] `;
    },
  },
  [eventCodes.SESSION_REJECT]: {
    type: 'session.rejected',
    desc: 'Session Rejected',
    format: ({ user, login, server_id, reason }) =>
      `User [${user}] was denied access to [${login}@${server_id}] because [${reason}]`,
  },
  [eventCodes.SESSION_LEAVE]: {
    type: 'session.leave',
    desc: 'User Disconnected',
    format: ({ user, sid }) => `User [${user}] has left the session [${sid}]`,
  },
  [eventCodes.SESSION_START]: {
    type: 'session.start',
    desc: 'Session Started',
    format: event => {
      const user = event.user || '';

      if (event.proto === 'kube') {
        if (!event.kubernetes_cluster) {
          return `User [${user}] has started a Kubernetes session [${event.sid}]`;
        }
        return `User [${user}] has started a session [${event.sid}] on Kubernetes cluster [${event.kubernetes_cluster}]`;
      }

      const node =
        event.server_hostname || event.server_addr || event.server_id;
      return `User [${user}] has started a session [${event.sid}] on node [${node}] `;
    },
  },
  [eventCodes.SESSION_UPLOAD]: {
    type: 'session.upload',
    desc: 'Session Uploaded',
    format: ({ sid }) => `Recorded session [${sid}] has been uploaded`,
  },
  [eventCodes.APP_SESSION_START]: {
    type: 'app.session.start',
    desc: 'App Session Started',
    format: event => {
      const { user, app_name, aws_role_arn } = event;
      if (aws_role_arn) {
        return `User [${user}] has connected to AWS console [${app_name}]`;
      }
      return `User [${user}] has connected to application [${app_name}]`;
    },
  },
  [eventCodes.APP_SESSION_END]: {
    type: 'app.session.end',
    desc: 'App Session Ended',
    format: event => {
      const { user, app_name } = event;
      return `User [${user}] has disconnected from application [${app_name}]`;
    },
  },
  [eventCodes.APP_SESSION_CHUNK]: {
    type: 'app.session.chunk',
    desc: 'App Session Data',
    format: event => {
      const { user, app_name } = event;
      return `New session data chunk created for application [${app_name}] accessed by user [${user}]`;
    },
  },
  [eventCodes.APP_SESSION_DYNAMODB_REQUEST]: {
    type: 'app.session.dynamodb.request',
    desc: 'App Session DynamoDB Request',
    format: ({ user, app_name, target }) => {
      let message = `User [${user}] has made a request to application [${app_name}]`;
      if (target) {
        message += `, target: [${target}]`;
      }
      return message;
    },
  },
  [eventCodes.SUBSYSTEM]: {
    type: 'subsystem',
    desc: 'Subsystem Requested',
    format: ({ user, name }) => `User [${user}] requested subsystem [${name}]`,
  },
  [eventCodes.SUBSYSTEM_FAILURE]: {
    type: 'subsystem',
    desc: 'Subsystem Request Failed',
    format: ({ user, name, exitError }) =>
      `User [${user}] subsystem [${name}] request failed [${exitError}]`,
  },
  [eventCodes.TERMINAL_RESIZE]: {
    type: 'resize',
    desc: 'Terminal Resize',
    format: ({ user, sid }) =>
      `User [${user}] resized the session [${sid}] terminal`,
  },
  [eventCodes.USER_CREATED]: {
    type: 'user.create',
    desc: 'User Created',
    format: ({ name }) => `User [${name}] has been created`,
  },
  [eventCodes.USER_DELETED]: {
    type: 'user.delete',
    desc: 'User Deleted',
    format: ({ name }) => `User [${name}] has been deleted`,
  },
  [eventCodes.USER_LOCAL_LOGIN]: {
    type: 'user.login',
    desc: 'Local Login',
    format: ({ user }) => `Local user [${user}] successfully logged in`,
  },
  [eventCodes.USER_LOCAL_LOGINFAILURE]: {
    type: 'user.login',
    desc: 'Local Login Failed',
    format: ({ user, error }) => `Local user [${user}] login failed [${error}]`,
  },
  [eventCodes.USER_SSO_LOGIN]: {
    type: 'user.login',
    desc: 'SSO Login',
    format: ({ user }) => `SSO user [${user}] successfully logged in`,
  },
  [eventCodes.USER_SSO_LOGINFAILURE]: {
    type: 'user.login',
    desc: 'SSO Login Failed',
    format: ({ error }) => `SSO user login failed [${error}]`,
  },
  [eventCodes.USER_SSO_TEST_FLOW_LOGIN]: {
    type: 'user.login',
    desc: 'SSO Test Flow Login',
    format: ({ user }) =>
      `SSO Test Flow: user [${user}] successfully logged in`,
  },
  [eventCodes.USER_SSO_TEST_FLOW_LOGINFAILURE]: {
    type: 'user.login',
    desc: 'SSO Test Flow Login Failed',
    format: ({ error }) => `SSO Test flow: user login failed [${error}]`,
  },
  [eventCodes.USER_HEADLESS_LOGIN_REQUESTED]: {
    type: 'user.login',
    desc: 'Headless Login Requested',
    format: ({ user }) => `Headless login was requested for user [${user}]`,
  },
  [eventCodes.USER_HEADLESS_LOGIN_APPROVED]: {
    type: 'user.login',
    desc: 'Headless Login Approved',
    format: ({ user }) =>
      `User [${user}] successfully approved headless login request`,
  },
  [eventCodes.USER_HEADLESS_LOGIN_APPROVEDFAILURE]: {
    type: 'user.login',
    desc: 'Headless Login Failed',
    format: ({ user, error }) =>
      `User [${user}] tried to approve headless login request, but got an error [${error}]`,
  },
  [eventCodes.USER_HEADLESS_LOGIN_REJECTED]: {
    type: 'user.login',
    desc: 'Headless Login Rejected',
    format: ({ user }) => `User [${user}] rejected headless login request`,
  },
  [eventCodes.CREATE_MFA_AUTH_CHALLENGE]: {
    type: 'mfa_auth_challenge.create',
    desc: 'MFA Authentication Attempt',
    format: ({ user }) => {
      if (user) {
        return `User [${user}] requested an MFA authentication challenge`;
      }
      return `Passwordless user requested an MFA authentication challenge`;
    },
  },
  [eventCodes.VALIDATE_MFA_AUTH_RESPONSE]: {
    type: 'mfa_auth_challenge.validate',
    desc: 'MFA Authentication Success',
    format: ({ user }) => `User [${user}] completed MFA authentication`,
  },
  [eventCodes.VALIDATE_MFA_AUTH_RESPONSEFAILURE]: {
    type: 'mfa_auth_challenge.validate',
    desc: 'MFA Authentication Failure',
    format: ({ user }) => `User [${user}] failed MFA authentication`,
  },
  [eventCodes.ROLE_CREATED]: {
    type: 'role.created',
    desc: 'User Role Created',
    format: ({ user, name }) => `User [${user}] created a role [${name}]`,
  },
  [eventCodes.ROLE_DELETED]: {
    type: 'role.deleted',
    desc: 'User Role Deleted',
    format: ({ user, name }) => `User [${user}] deleted a role [${name}]`,
  },
  [eventCodes.ROLE_UPDATED]: {
    type: 'role.updated',
    desc: 'User Role Updated',
    format: ({ user, name }) => `User [${user}] updated a role [${name}]`,
  },
  [eventCodes.TRUSTED_CLUSTER_TOKEN_CREATED]: {
    type: 'trusted_cluster_token.create',
    desc: 'Trusted Cluster Token Created',
    format: ({ user }) => `User [${user}] has created a trusted cluster token`,
  },
  [eventCodes.TRUSTED_CLUSTER_CREATED]: {
    type: 'trusted_cluster.create',
    desc: 'Trusted Cluster Created',
    format: ({ user, name }) =>
      `User [${user}] has created a trusted relationship with cluster [${name}]`,
  },
  [eventCodes.PROVISION_TOKEN_CREATED]: {
    type: 'join_token.create',
    desc: 'Join Token Created',
    format: ({ user, roles, join_method }) =>
      `User [${user}] created a join token with role(s) [${roles}] and a join method [${join_method}]`,
  },
  [eventCodes.TRUSTED_CLUSTER_DELETED]: {
    type: 'trusted_cluster.delete',
    desc: 'Trusted Cluster Deleted',
    format: ({ user, name }) =>
      `User [${user}] has deleted a trusted relationship with cluster [${name}]`,
  },
  [eventCodes.KUBE_REQUEST]: {
    type: 'kube.request',
    desc: 'Kubernetes Request',
    format: ({ user, kubernetes_cluster, verb, request_path, response_code }) =>
      `User [${user}] received a [${response_code}] from a [${verb} ${request_path}] request to Kubernetes cluster [${kubernetes_cluster}]`,
  },
  [eventCodes.KUBE_CREATED]: {
    type: 'kube.create',
    desc: 'Kubernetes Created',
    format: ({ user, name }) =>
      `User [${user}] created Kubernetes cluster [${name}]`,
  },
  [eventCodes.KUBE_UPDATED]: {
    type: 'kube.update',
    desc: 'Kubernetes Updated',
    format: ({ user, name }) =>
      `User [${user}] updated Kubernetes cluster [${name}]`,
  },
  [eventCodes.KUBE_DELETED]: {
    type: 'kube.delete',
    desc: 'Kubernetes Deleted',
    format: ({ user, name }) =>
      `User [${user}] deleted Kubernetes cluster [${name}]`,
  },
  [eventCodes.DATABASE_SESSION_STARTED]: {
    type: 'db.session.start',
    desc: 'Database Session Started',
    format: ({ user, db_service, db_name, db_user, db_roles }) =>
      `User [${user}] has connected ${
        db_name ? `to database [${db_name}] ` : ''
      }as [${db_user}] ${
        db_roles ? `with roles [${db_roles}] ` : ''
      }on [${db_service}]`,
  },
  [eventCodes.DATABASE_SESSION_STARTED_FAILURE]: {
    type: 'db.session.start',
    desc: 'Database Session Denied',
    format: ({ user, db_service, db_name, db_user }) =>
      `User [${user}] was denied access to database [${db_name}] as [${db_user}] on [${db_service}]`,
  },
  [eventCodes.DATABASE_SESSION_ENDED]: {
    type: 'db.session.end',
    desc: 'Database Session Ended',
    format: ({ user, db_service, db_name }) =>
      `User [${user}] has disconnected ${
        db_name ? `from database [${db_name}] ` : ''
      }on [${db_service}]`,
  },
  [eventCodes.DATABASE_SESSION_QUERY]: {
    type: 'db.session.query',
    desc: 'Database Query',
    format: ({ user, db_service, db_name, db_query }) =>
      `User [${user}] has executed query [${truncateStr(
        db_query,
        80
      )}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.DATABASE_SESSION_QUERY_FAILURE]: {
    type: 'db.session.query.failed',
    desc: 'Database Query Failed',
    format: ({ user, db_service, db_name, db_query }) =>
      `User [${user}] query [${truncateStr(
        db_query,
        80
      )}] in database [${db_name}] on [${db_service}] failed`,
  },
  [eventCodes.DATABASE_SESSION_MALFORMED_PACKET]: {
    type: 'db.session.malformed_packet',
    desc: 'Database Malformed Packet',
    format: ({ user, db_service, db_name }) =>
      `Received malformed packet from [${user}] in [${db_name}] on database [${db_service}]`,
  },
  [eventCodes.DATABASE_SESSION_PERMISSIONS_UPDATE]: {
    type: 'db.session.permissions.update',
    desc: 'Database User Permissions Updated',
    format: ({ user, db_service, db_name, permission_summary }) => {
      if (!permission_summary) {
        return `Database user [${user}] permissions updated for database [${db_name}] on [${db_service}]`;
      }
      const summary = permission_summary
        .map(p => {
          const details = Object.entries(p.counts)
            .map(([key, value]) => `${key}:${value}`)
            .join(',');
          return `${p.permission}:${details}`;
        })
        .join('; ');
      return `Database user [${user}] permissions updated for database [${db_name}] on [${db_service}]: ${summary}`;
    },
  },
  [eventCodes.DATABASE_SESSION_USER_CREATE]: {
    type: 'db.session.user.create',
    desc: 'Database User Created',
    format: ev => {
      if (!ev.roles) {
        return `Database user [${ev.user}] created in database [${ev.db_service}]`;
      }
      return `Database user [${ev.user}] created in database [${ev.db_service}], roles: [${ev.roles}]`;
    },
  },
  [eventCodes.DATABASE_SESSION_USER_CREATE_FAILURE]: {
    type: 'db.session.user.create',
    desc: 'Database User Creation Failed',
    format: ev => {
      return `Failed to create database user [${ev.user}] in database [${ev.db_service}], error: [${ev.error}]`;
    },
  },
  [eventCodes.DATABASE_SESSION_USER_DEACTIVATE]: {
    type: 'db.session.user.deactivate',
    desc: 'Database User Deactivated',
    format: ev => {
      if (!ev.delete) {
        return `Database user [${ev.user}] disabled in database [${ev.db_service}]`;
      }
      return `Database user [${ev.user}] deleted in database [${ev.db_service}]`;
    },
  },
  [eventCodes.DATABASE_SESSION_USER_DEACTIVATE_FAILURE]: {
    type: 'db.session.user.deactivate',
    desc: 'Database User Deactivate Failure',
    format: ev => {
      return `Failed to disable database user [${ev.user}] in database [${ev.db_service}], error: [${ev.error}]`;
    },
  },
  [eventCodes.DATABASE_CREATED]: {
    type: 'db.create',
    desc: 'Database Created',
    format: ({ user, name }) => `User [${user}] created database [${name}]`,
  },
  [eventCodes.DATABASE_UPDATED]: {
    type: 'db.update',
    desc: 'Database Updated',
    format: ({ user, name }) => `User [${user}] updated database [${name}]`,
  },
  [eventCodes.DATABASE_DELETED]: {
    type: 'db.delete',
    desc: 'Database Deleted',
    format: ({ user, name }) => `User [${user}] deleted database [${name}]`,
  },
  [eventCodes.APP_CREATED]: {
    type: 'app.create',
    desc: 'Application Created',
    format: ({ user, name }) => `User [${user}] created application [${name}]`,
  },
  [eventCodes.APP_UPDATED]: {
    type: 'app.update',
    desc: 'Application Updated',
    format: ({ user, name }) => `User [${user}] updated application [${name}]`,
  },
  [eventCodes.APP_DELETED]: {
    type: 'app.delete',
    desc: 'Application Deleted',
    format: ({ user, name }) => `User [${user}] deleted application [${name}]`,
  },
  [eventCodes.POSTGRES_PARSE]: {
    type: 'db.session.postgres.statements.parse',
    desc: 'PostgreSQL Statement Parse',
    format: ({ user, db_service, statement_name, query }) =>
      `User [${user}] has prepared [${truncateStr(
        query,
        80
      )}] as statement [${statement_name}] on [${db_service}]`,
  },
  [eventCodes.POSTGRES_BIND]: {
    type: 'db.session.postgres.statements.bind',
    desc: 'PostgreSQL Statement Bind',
    format: ({ user, db_service, statement_name, portal_name }) =>
      `User [${user}] has readied statement [${statement_name}] for execution as portal [${portal_name}] on [${db_service}]`,
  },
  [eventCodes.POSTGRES_EXECUTE]: {
    type: 'db.session.postgres.statements.execute',
    desc: 'PostgreSQL Statement Execute',
    format: ({ user, db_service, portal_name }) =>
      `User [${user}] has executed portal [${portal_name}] on [${db_service}]`,
  },
  [eventCodes.POSTGRES_CLOSE]: {
    type: 'db.session.postgres.statements.close',
    desc: 'PostgreSQL Statement Close',
    format: e => {
      if (e.portal_name) {
        return `User [${e.user}] has closed portal [${e.portal_name}] on [${e.db_service}]`;
      }
      return `User [${e.user}] has closed statement [${e.statement_name}] on [${e.db_service}]`;
    },
  },
  [eventCodes.POSTGRES_FUNCTION_CALL]: {
    type: 'db.session.postgres.function',
    desc: 'PostgreSQL Function Call',
    format: ({ user, db_service, function_oid }) =>
      `User [${user}] has executed function with OID [${function_oid}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_STATEMENT_PREPARE]: {
    type: 'db.session.mysql.statements.prepare',
    desc: 'MySQL Statement Prepare',
    format: ({ user, db_service, db_name, query }) =>
      `User [${user}] has prepared [${truncateStr(
        query,
        80
      )}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_STATEMENT_EXECUTE]: {
    type: 'db.session.mysql.statements.execute',
    desc: 'MySQL Statement Execute',
    format: ({ user, db_service, db_name, statement_id }) =>
      `User [${user}] has executed statement [${statement_id}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_STATEMENT_SEND_LONG_DATA]: {
    type: 'db.session.mysql.statements.send_long_data',
    desc: 'MySQL Statement Send Long Data',
    format: ({
      user,
      db_service,
      db_name,
      statement_id,
      parameter_id,
      data_size,
    }) =>
      `User [${user}] has sent ${data_size} bytes of data to parameter [${parameter_id}] of statement [${statement_id}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_STATEMENT_CLOSE]: {
    type: 'db.session.mysql.statements.close',
    desc: 'MySQL Statement Close',
    format: ({ user, db_service, db_name, statement_id }) =>
      `User [${user}] has closed statement [${statement_id}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_STATEMENT_RESET]: {
    type: 'db.session.mysql.statements.reset',
    desc: 'MySQL Statement Reset',
    format: ({ user, db_service, db_name, statement_id }) =>
      `User [${user}] has reset statement [${statement_id}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_STATEMENT_FETCH]: {
    type: 'db.session.mysql.statements.fetch',
    desc: 'MySQL Statement Fetch',
    format: ({ user, db_service, db_name, rows_count, statement_id }) =>
      `User [${user}] has fetched ${rows_count} rows of statement [${statement_id}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_STATEMENT_BULK_EXECUTE]: {
    type: 'db.session.mysql.statements.bulk_execute',
    desc: 'MySQL Statement Bulk Execute',
    format: ({ user, db_service, db_name, statement_id }) =>
      `User [${user}] has executed statement [${statement_id}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_INIT_DB]: {
    type: 'db.session.mysql.init_db',
    desc: 'MySQL Change Database',
    format: ({ user, db_service, schema_name }) =>
      `User [${user}] has changed default database to [${schema_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_CREATE_DB]: {
    type: 'db.session.mysql.create_db',
    desc: 'MySQL Create Database',
    format: ({ user, db_service, schema_name }) =>
      `User [${user}] has created database [${schema_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_DROP_DB]: {
    type: 'db.session.mysql.drop_db',
    desc: 'MySQL Drop Database',
    format: ({ user, db_service, schema_name }) =>
      `User [${user}] has dropped database [${schema_name}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_SHUT_DOWN]: {
    type: 'db.session.mysql.shut_down',
    desc: 'MySQL Shut Down',
    format: ({ user, db_service }) =>
      `User [${user}] has attempted to shut down [${db_service}]`,
  },
  [eventCodes.MYSQL_PROCESS_KILL]: {
    type: 'db.session.mysql.process_kill',
    desc: 'MySQL Kill Process',
    format: ({ user, db_service, process_id }) =>
      `User [${user}] has attempted to kill process [${process_id}] on [${db_service}]`,
  },
  [eventCodes.MYSQL_DEBUG]: {
    type: 'db.session.mysql.debug',
    desc: 'MySQL Debug',
    format: ({ user, db_service }) =>
      `User [${user}] has asked [${db_service}] to dump debug information`,
  },
  [eventCodes.MYSQL_REFRESH]: {
    type: 'db.session.mysql.refresh',
    desc: 'MySQL Refresh',
    format: ({ user, db_service, subcommand }) =>
      `User [${user}] has sent command [${subcommand}] to [${db_service}]`,
  },
  [eventCodes.SQLSERVER_RPC_REQUEST]: {
    type: 'db.session.sqlserver.rpc_request',
    desc: 'SQLServer RPC Request',
    format: ({ user, db_service, db_name, proc_name }) =>
      `User [${user}] has sent RPC Request [${proc_name}] in database [${db_name}] on [${db_service}]`,
  },
  [eventCodes.CASSANDRA_BATCH_EVENT]: {
    type: 'db.session.cassandra.batch',
    desc: 'Cassandra Batch',
    format: ({ user, db_service }) =>
      `User [${user}] has sent Cassandra Batch to [${db_service}]`,
  },
  [eventCodes.CASSANDRA_PREPARE_EVENT]: {
    type: 'db.session.cassandra.prepare',
    desc: 'Cassandra Prepare Event',
    format: ({ user, db_service, query }) =>
      `User [${user}] has sent Cassandra Prepare [${truncateStr(
        query,
        80
      )}] to [${db_service}]`,
  },
  [eventCodes.CASSANDRA_EXECUTE_EVENT]: {
    type: 'db.session.cassandra.execute',
    desc: 'Cassandra Execute',
    format: ({ user, db_service }) =>
      `User [${user}] has sent Cassandra Execute to [${db_service}]`,
  },
  [eventCodes.CASSANDRA_REGISTER_EVENT]: {
    type: 'db.session.cassandra.register',
    desc: 'Cassandra Register',
    format: ({ user, db_service }) =>
      `User [${user}] has sent Cassandra Register to [${db_service}]`,
  },
  [eventCodes.ELASTICSEARCH_REQUEST]: {
    type: 'db.session.elasticsearch.request',
    desc: 'Elasticsearch Request',
    format: formatElasticsearchEvent,
  },
  [eventCodes.ELASTICSEARCH_REQUEST_FAILURE]: {
    type: 'db.session.elasticsearch.request',
    desc: 'Elasticsearch Request Failed',
    format: formatElasticsearchEvent,
  },
  [eventCodes.OPENSEARCH_REQUEST]: {
    type: 'db.session.opensearch.request',
    desc: 'OpenSearch Request',
    format: formatElasticsearchEvent,
  },
  [eventCodes.OPENSEARCH_REQUEST_FAILURE]: {
    type: 'db.session.opensearch.request',
    desc: 'OpenSearch Request Failed',
    format: formatElasticsearchEvent,
  },
  [eventCodes.DYNAMODB_REQUEST]: {
    type: 'db.session.dynamodb.request',
    desc: 'DynamoDB Request',
    format: ({ user, db_service, target }) => {
      let message = `User [${user}] has made a request to database [${db_service}]`;
      if (target) {
        message += `, target API: [${target}]`;
      }
      return message;
    },
  },
  [eventCodes.DYNAMODB_REQUEST_FAILURE]: {
    type: 'db.session.dynamodb.request',
    desc: 'DynamoDB Request Failed',
    format: ({ user, db_service, target }) => {
      let message = `User [${user}] failed to make a request to database  [${db_service}]`;
      if (target) {
        message += `, target API: [${target}]`;
      }
      return message;
    },
  },
  [eventCodes.MFA_DEVICE_ADD]: {
    type: 'mfa.add',
    desc: 'MFA Device Added',
    format: ({ user, mfa_device_name, mfa_device_type }) =>
      `User [${user}] added ${mfa_device_type} device [${mfa_device_name}]`,
  },
  [eventCodes.MFA_DEVICE_DELETE]: {
    type: 'mfa.delete',
    desc: 'MFA Device Deleted',
    format: ({ user, mfa_device_name, mfa_device_type }) =>
      `User [${user}] deleted ${mfa_device_type} device [${mfa_device_name}]`,
  },
  [eventCodes.BILLING_CARD_CREATE]: {
    type: 'billing.create_card',
    desc: 'Credit Card Added',
    format: ({ user }) => `User [${user}] has added a credit card`,
  },
  [eventCodes.BILLING_CARD_DELETE]: {
    type: 'billing.delete_card',
    desc: 'Credit Card Deleted',
    format: ({ user }) => `User [${user}] has deleted a credit card`,
  },
  [eventCodes.BILLING_CARD_UPDATE]: {
    type: 'billing.update_card',
    desc: 'Credit Card Updated',
    format: ({ user }) => `User [${user}] has updated a credit card`,
  },
  [eventCodes.BILLING_INFORMATION_UPDATE]: {
    type: 'billing.update_info',
    desc: 'Billing Information Updated',
    format: ({ user }) => `User [${user}] has updated the billing information`,
  },
  [eventCodes.LOCK_CREATED]: {
    type: 'lock.created',
    desc: 'Lock Created',
    format: ({ user, name }) => `Lock [${name}] was created by user [${user}]`,
  },
  [eventCodes.LOCK_DELETED]: {
    type: 'lock.deleted',
    desc: 'Lock Deleted',
    format: ({ user, name }) => `Lock [${name}] was deleted by user [${user}]`,
  },
  [eventCodes.PRIVILEGE_TOKEN_CREATED]: {
    type: 'privilege_token.create',
    desc: 'Privilege Token Created',
    format: ({ name }) => `Privilege token was created for user [${name}]`,
  },
  [eventCodes.RECOVERY_TOKEN_CREATED]: {
    type: 'recovery_token.create',
    desc: 'Recovery Token Created',
    format: ({ name }) => `Recovery token was created for user [${name}]`,
  },
  [eventCodes.RECOVERY_CODE_GENERATED]: {
    type: 'recovery_code.generated',
    desc: 'Recovery Codes Generated',
    format: ({ user }) =>
      `New recovery codes were generated for user [${user}]`,
  },
  [eventCodes.RECOVERY_CODE_USED]: {
    type: 'recovery_code.used',
    desc: 'Recovery Code Used',
    format: ({ user }) => `User [${user}] successfully used a recovery code`,
  },
  [eventCodes.RECOVERY_CODE_USED_FAILURE]: {
    type: 'recovery_code.used',
    desc: 'Recovery Code Use Failed',
    format: ({ user }) =>
      `User [${user}] failed an attempt to use a recovery code`,
  },
  [eventCodes.DESKTOP_SESSION_STARTED]: {
    type: 'windows.desktop.session.start',
    desc: 'Windows Desktop Session Started',
    format: ({ user, windows_domain, desktop_name, sid, windows_user }) => {
      let message = `User [${user}] started session ${sid} on Windows desktop [${windows_user}@${desktop_name}]`;
      if (windows_domain) {
        message += ` with domain [${windows_domain}]`;
      }
      return message;
    },
  },
  [eventCodes.DESKTOP_SESSION_STARTED_FAILED]: {
    type: 'windows.desktop.session.start',
    desc: 'Windows Desktop Session Denied',
    format: ({ user, windows_domain, desktop_name, windows_user }) => {
      let message = `User [${user}] was denied access to Windows desktop [${windows_user}@${desktop_name}]`;
      if (windows_domain) {
        message += ` with domain [${windows_domain}]`;
      }
      return message;
    },
  },
  [eventCodes.DESKTOP_SESSION_ENDED]: {
    type: 'windows.desktop.session.end',
    desc: 'Windows Desktop Session Ended',
    format: ({ user, windows_domain, desktop_name, sid, windows_user }) => {
      let desktopMessage = `[${windows_user}@${desktop_name}]`;
      if (windows_domain) {
        desktopMessage += ` with domain [${windows_domain}]`;
      }
      let message = `Session ${sid} for Windows desktop ${desktopMessage} has ended for user [${user}]`;
      return message;
    },
  },
  [eventCodes.DESKTOP_CLIPBOARD_RECEIVE]: {
    type: 'desktop.clipboard.receive',
    desc: 'Clipboard Data Received',
    format: ({ user, desktop_addr, length }) =>
      `User [${user}] received ${length} bytes of clipboard data from desktop [${desktop_addr}]`,
  },
  [eventCodes.DESKTOP_CLIPBOARD_SEND]: {
    type: 'desktop.clipboard.send',
    desc: 'Clipboard Data Sent',
    format: ({ user, desktop_addr, length }) =>
      `User [${user}] sent ${length} bytes of clipboard data to desktop [${desktop_addr}]`,
  },
  [eventCodes.DESKTOP_SHARED_DIRECTORY_START]: {
    type: 'desktop.directory.share',
    desc: 'Directory Sharing Started',
    format: ({ user, desktop_addr, directory_name }) =>
      `User [${user}] started sharing directory [${directory_name}] to desktop [${desktop_addr}]`,
  },
  [eventCodes.DESKTOP_SHARED_DIRECTORY_START_FAILURE]: {
    type: 'desktop.directory.share',
    desc: 'Directory Sharing Start Failed',
    format: ({ user, desktop_addr, directory_name }) =>
      `User [${user}] failed to start sharing directory [${directory_name}] to desktop [${desktop_addr}]`,
  },
  [eventCodes.DESKTOP_SHARED_DIRECTORY_READ]: {
    type: 'desktop.directory.read',
    desc: 'Directory Sharing Read',
    format: ({ user, desktop_addr, directory_name, file_path, length }) =>
      `User [${user}] read [${length}] bytes from file [${file_path}] in shared directory [${directory_name}] on desktop [${desktop_addr}]`,
  },
  [eventCodes.DESKTOP_SHARED_DIRECTORY_READ_FAILURE]: {
    type: 'desktop.directory.read',
    desc: 'Directory Sharing Read Failed',
    format: ({ user, desktop_addr, directory_name, file_path, length }) =>
      `User [${user}] failed to read [${length}] bytes from file [${file_path}] in shared directory [${directory_name}] on desktop [${desktop_addr}]`,
  },
  [eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE]: {
    type: 'desktop.directory.write',
    desc: 'Directory Sharing Write',
    format: ({ user, desktop_addr, directory_name, file_path, length }) =>
      `User [${user}] wrote [${length}] bytes to file [${file_path}] in shared directory [${directory_name}] on desktop [${desktop_addr}]`,
  },
  [eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE_FAILURE]: {
    type: 'desktop.directory.write',
    desc: 'Directory Sharing Write Failed',
    format: ({ user, desktop_addr, directory_name, file_path, length }) =>
      `User [${user}] failed to write [${length}] bytes to file [${file_path}] in shared directory [${directory_name}] on desktop [${desktop_addr}]`,
  },
  [eventCodes.DEVICE_CREATE]: {
    type: 'device.create',
    desc: 'Device Registered',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has registered a device`
        : `User [${user}] has failed to register a device`,
  },
  [eventCodes.DEVICE_DELETE]: {
    type: 'device.delete',
    desc: 'Device Deleted',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has deleted a device`
        : `User [${user}] has failed to delete a device`,
  },
  [eventCodes.DEVICE_AUTHENTICATE]: {
    type: 'device.authenticate',
    desc: 'Device Authenticated',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has successfully authenticated their device`
        : `User [${user}] has failed to authenticate their device`,
  },
  [eventCodes.DEVICE_ENROLL]: {
    type: 'device.enroll',
    desc: 'Device Enrolled',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has successfully enrolled their device`
        : `User [${user}] has failed to enroll their device`,
  },
  [eventCodes.DEVICE_ENROLL_TOKEN_CREATE]: {
    type: 'device.token.create',
    desc: 'Device Enroll Token Created',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] created a device enroll token`
        : `User [${user}] has failed to create a device enroll token`,
  },
  [eventCodes.DEVICE_ENROLL_TOKEN_SPENT]: {
    type: 'device.token.spent',
    desc: 'Device Enroll Token Spent',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has spent a device enroll token`
        : `User [${user}] has failed to spend a device enroll token`,
  },
  [eventCodes.DEVICE_UPDATE]: {
    type: 'device.update',
    desc: 'Device Updated',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has updated a device`
        : `User [${user}] has failed to update a device`,
  },
  [eventCodes.DEVICE_WEB_TOKEN_CREATE]: {
    type: 'device.webtoken.create',
    desc: 'Device Web Token Created',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has issued a device web token`
        : `User [${user}] has failed to issue a device web token`,
  },
  [eventCodes.DEVICE_AUTHENTICATE_CONFIRM]: {
    type: 'device.authenticate.confirm',
    desc: 'Device Web Authentication Confirmed',
    format: ({ user, status, success }) =>
      success || (status && status.success)
        ? `User [${user}] has confirmed device web authentication`
        : `User [${user}] has failed to confirm device web authentication`,
  },
  [eventCodes.X11_FORWARD]: {
    type: 'x11-forward',
    desc: 'X11 Forwarding Requested',
    format: ({ user }) =>
      `User [${user}] has requested x11 forwarding for a session`,
  },
  [eventCodes.X11_FORWARD_FAILURE]: {
    type: 'x11-forward',
    desc: 'X11 Forwarding Request Failed',
    format: ({ user }) =>
      `User [${user}] was denied x11 forwarding for a session`,
  },
  [eventCodes.SESSION_CONNECT]: {
    type: 'session.connect',
    desc: 'Session Connected',
    format: ({ server_addr }) => `Session connected to [${server_addr}]`,
  },
  [eventCodes.CERTIFICATE_CREATED]: {
    type: 'cert.create',
    desc: 'Certificate Issued',
    format: ({ cert_type, identity: { user } }) => {
      if (cert_type === 'user') {
        return `User certificate issued for [${user}]`;
      }
      return `Certificate of type [${cert_type}] issued for [${user}]`;
    },
  },
  [eventCodes.UPGRADE_WINDOW_UPDATED]: {
    type: 'upgradewindow.update',
    desc: 'Upgrade Window Start Updated',
    format: ({ user, upgrade_window_start }) => {
      return `Upgrade Window Start updated to [${upgrade_window_start}] by user [${user}]`;
    },
  },
  [eventCodes.SESSION_RECORDING_ACCESS]: {
    type: 'session.recording.access',
    desc: 'Session Recording Accessed',
    format: ({ sid, user }) => {
      return `User [${user}] accessed a session recording [${sid}]`;
    },
  },
  [eventCodes.SSMRUN_SUCCESS]: {
    type: 'ssm.run',
    desc: 'SSM Command Executed',
    format: ({ account_id, instance_id, region, command_id }) => {
      return `SSM Command with ID [${command_id}] was successfully executed on EC2 Instance [${instance_id}] on AWS Account [${account_id}] in [${region}]`;
    },
  },
  [eventCodes.SSMRUN_FAIL]: {
    type: 'ssm.run',
    desc: 'SSM Command Execution Failed',
    format: ({ account_id, instance_id, region, command_id }) => {
      return `SSM Command with ID [${command_id}] failed during execution on EC2 Instance [${instance_id}] on AWS Account [${account_id}] in [${region}]`;
    },
  },
  [eventCodes.BOT_JOIN]: {
    type: 'bot.join',
    desc: 'Bot Joined',
    format: ({ bot_name, method }) => {
      return `Bot [${bot_name}] joined the cluster using the [${method}] join method`;
    },
  },
  [eventCodes.BOT_JOIN_FAILURE]: {
    type: 'bot.join',
    desc: 'Bot Join Failed',
    format: ({ bot_name }) => {
      return `Bot [${bot_name || 'unknown'}] failed to join the cluster`;
    },
  },
  [eventCodes.INSTANCE_JOIN]: {
    type: 'instance.join',
    desc: 'Instance Joined',
    format: ({ node_name, role, method }) => {
      return `Instance [${node_name}] joined the cluster with the [${role}] role using the [${method}] join method`;
    },
  },
  [eventCodes.INSTANCE_JOIN_FAILURE]: {
    type: 'instance.join',
    desc: 'Instance Join Failed',
    format: ({ node_name }) => {
      return `Instance [${node_name || 'unknown'}] failed to join the cluster`;
    },
  },
  [eventCodes.BOT_CREATED]: {
    type: 'bot.create',
    desc: 'Bot Created',
    format: ({ user, name }) => {
      return `User [${user}] created a Bot [${name}]`;
    },
  },
  [eventCodes.BOT_UPDATED]: {
    type: 'bot.update',
    desc: 'Bot Updated',
    format: ({ user, name }) => {
      return `User [${user}] modified a Bot [${name}]`;
    },
  },
  [eventCodes.BOT_DELETED]: {
    type: 'bot.delete',
    desc: 'Bot Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted a Bot [${name}]`;
    },
  },
  [eventCodes.WORKLOAD_IDENTITY_CREATE]: {
    type: 'workload_identity.create',
    desc: 'Workload Identity Created',
    format: ({ user, name }) => {
      return `User [${user}] created a Workload Identity [${name}]`;
    },
  },
  [eventCodes.WORKLOAD_IDENTITY_UPDATE]: {
    type: 'workload_identity.update',
    desc: 'Workload Identity Updated',
    format: ({ user, name }) => {
      return `User [${user}] updated a Workload Identity [${name}]`;
    },
  },
  [eventCodes.WORKLOAD_IDENTITY_DELETE]: {
    type: 'workload_identity.delete',
    desc: 'Workload Identity Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted a Workload Identity [${name}]`;
    },
  },
  [eventCodes.LOGIN_RULE_CREATE]: {
    type: 'login_rule.create',
    desc: 'Login Rule Created',
    format: ({ user, name }) => `User [${user}] created a login rule [${name}]`,
  },
  [eventCodes.LOGIN_RULE_DELETE]: {
    type: 'login_rule.delete',
    desc: 'Login Rule Deleted',
    format: ({ user, name }) => `User [${user}] deleted a login rule [${name}]`,
  },
  [eventCodes.SAML_IDP_AUTH_ATTEMPT]: {
    type: 'saml.idp.auth',
    desc: 'SAML IdP authentication',
    format: ({
      user,
      success,
      service_provider_entity_id,
      service_provider_shortcut,
    }) => {
      const desc =
        success === false
          ? 'failed to authenticate'
          : 'successfully authenticated';
      const id = service_provider_entity_id
        ? `[${service_provider_entity_id}]`
        : `[${service_provider_shortcut}]`;
      return `User [${user}] ${desc} to SAML service provider ${id}`;
    },
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE]: {
    type: 'saml.idp.service.provider.create',
    desc: 'SAML IdP service provider created',
    format: ({ updated_by, name, service_provider_entity_id }) =>
      `User [${updated_by}] added service provider [${name}] with entity ID [${service_provider_entity_id}]`,
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE_FAILURE]: {
    type: 'saml.idp.service.provider.create',
    desc: 'SAML IdP service provider create failed',
    format: ({ updated_by, name, service_provider_entity_id }) =>
      `User [${updated_by}] failed to add service provider [${name}] with entity ID [${service_provider_entity_id}]`,
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE]: {
    type: 'saml.idp.service.provider.update',
    desc: 'SAML IdP service provider updated',
    format: ({ updated_by, name, service_provider_entity_id }) =>
      `User [${updated_by}] updated service provider [${name}] with entity ID [${service_provider_entity_id}]`,
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE_FAILURE]: {
    type: 'saml.idp.service.provider.update',
    desc: 'SAML IdP service provider update failed',
    format: ({ updated_by, name, service_provider_entity_id }) =>
      `User [${updated_by}] failed to update service provider [${name}] with entity ID [${service_provider_entity_id}]`,
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE]: {
    type: 'saml.idp.service.provider.delete',
    desc: 'SAML IdP service provider deleted',
    format: ({ updated_by, name, service_provider_entity_id }) =>
      `User [${updated_by}] deleted service provider [${name}] with entity ID [${service_provider_entity_id}]`,
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_FAILURE]: {
    type: 'saml.idp.service.provider.delete',
    desc: 'SAML IdP service provider delete failed',
    format: ({ updated_by, name, service_provider_entity_id }) =>
      `User [${updated_by}] failed to delete service provider [${name}] with entity ID [${service_provider_entity_id}]`,
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL]: {
    type: 'saml.idp.service.provider.delete_all',
    desc: 'All SAML IdP service provider deleted',
    format: ({ updated_by }) =>
      `User [${updated_by}] deleted all service providers`,
  },
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL_FAILURE]: {
    type: 'saml.idp.service.provider.delete',
    desc: 'SAML IdP service provider delete failed',
    format: ({ updated_by }) =>
      `User [${updated_by}] failed to delete all service providers`,
  },
  [eventCodes.OKTA_GROUPS_UPDATE]: {
    type: 'okta.groups.update',
    desc: 'Okta groups have been updated',
    format: ({ added, updated, deleted }) =>
      `[${added}] added, [${updated}] updated, [${deleted}] deleted`,
  },
  [eventCodes.OKTA_APPLICATIONS_UPDATE]: {
    type: 'okta.applications.update',
    desc: 'Okta applications have been updated',
    format: ({ added, updated, deleted }) =>
      `[${added}] added, [${updated}] updated, [${deleted}] deleted`,
  },
  [eventCodes.OKTA_SYNC_FAILURE]: {
    type: 'okta.sync.failure',
    desc: 'Okta synchronization failed',
    format: () => `Okta synchronization failed`,
  },
  [eventCodes.OKTA_ASSIGNMENT_PROCESS]: {
    type: 'okta.assignment.process',
    desc: 'Okta assignment has been processed',
    format: ({ name, source, user }) =>
      `Okta assignment [${name}], source [${source}], user [${user}] has been successfully processed`,
  },
  [eventCodes.OKTA_ASSIGNMENT_PROCESS_FAILURE]: {
    type: 'okta.assignment.process',
    desc: 'Okta assignment failed to process',
    format: ({ name, source, user }) =>
      `Okta assignment [${name}], source [${source}], user [${user}] processing has failed`,
  },
  [eventCodes.OKTA_ASSIGNMENT_CLEANUP]: {
    type: 'okta.assignment.cleanup',
    desc: 'Okta assignment has been cleaned up',
    format: ({ name, source, user }) =>
      `Okta assignment [${name}], source [${source}], user [${user}] has been successfully cleaned up`,
  },
  [eventCodes.OKTA_ASSIGNMENT_CLEANUP_FAILURE]: {
    type: 'okta.assignment.cleanup',
    desc: 'Okta assignment failed to clean up',
    format: ({ name, source, user }) =>
      `Okta assignment [${name}], source [${source}], user [${user}] cleanup has failed`,
  },
  [eventCodes.OKTA_USER_SYNC]: {
    type: 'okta.user.sync',
    desc: 'Okta user synchronization completed',
    format: ({ num_users_created, num_users_modified, num_users_deleted }) =>
      `[${num_users_created}] users added, [${num_users_modified}] users updated, [${num_users_deleted}] users deleted`,
  },
  [eventCodes.OKTA_USER_SYNC_FAILURE]: {
    type: 'okta.user.sync',
    desc: 'Okta user synchronization failed',
    format: () => `Okta user synchronization failed`,
  },
  [eventCodes.OKTA_ACCESS_LIST_SYNC]: {
    type: 'okta.access_list.sync',
    desc: 'Okta access list synchronization completed',
    format: () => `Okta access list synchronization successfully completed`,
  },
  [eventCodes.OKTA_ACCESS_LIST_SYNC_FAILURE]: {
    type: 'okta.access_list.sync',
    desc: 'Okta access list synchronization failed',
    format: () => `Okta access list synchronization failed`,
  },
  [eventCodes.ACCESS_LIST_CREATE]: {
    type: 'access_list.create',
    desc: 'Access list created',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] created access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_CREATE_FAILURE]: {
    type: 'access_list.create',
    desc: 'Access list create failed',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] failed to create access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_UPDATE]: {
    type: 'access_list.update',
    desc: 'Access list updated',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] updated access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_UPDATE_FAILURE]: {
    type: 'access_list.update',
    desc: 'Access list update failed',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] failed to update access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_DELETE]: {
    type: 'access_list.delete',
    desc: 'Access list deleted',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] deleted access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_DELETE_FAILURE]: {
    type: 'access_list.delete',
    desc: 'Access list delete failed',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] failed to delete access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_REVIEW]: {
    type: 'access_list.review',
    desc: 'Access list reviewed',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] reviewed access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_REVIEW_FAILURE]: {
    type: 'access_list.review',
    desc: 'Access list review failed',
    format: ({ name, updated_by }) =>
      `User [${updated_by}] failed to to review access list [${name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_CREATE]: {
    type: 'access_list.member.create',
    desc: 'Access list member added',
    format: ({ access_list_name, members, updated_by }) =>
      `User [${updated_by}] added ${formatMembers(
        members
      )} to access list [${access_list_name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_CREATE_FAILURE]: {
    type: 'access_list.member.create',
    desc: 'Access list member addition failure',
    format: ({ access_list_name, members, updated_by }) =>
      `User [${updated_by}] failed to add ${formatMembers(
        members
      )} to access list [${access_list_name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_UPDATE]: {
    type: 'access_list.member.update',
    desc: 'Access list member updated',
    format: ({ access_list_name, members, updated_by }) =>
      `User [${updated_by}] updated ${formatMembers(
        members
      )} in access list [${access_list_name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_UPDATE_FAILURE]: {
    type: 'access_list.member.update',
    desc: 'Access list member update failure',
    format: ({ access_list_name, members, updated_by }) =>
      `User [${updated_by}] failed to update ${formatMembers(
        members
      )} in access list [${access_list_name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_DELETE]: {
    type: 'access_list.member.delete',
    desc: 'Access list member removed',
    format: ({ access_list_name, members, updated_by }) =>
      `User [${updated_by}] removed ${formatMembers(
        members
      )} from access list [${access_list_name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_FAILURE]: {
    type: 'access_list.member.delete',
    desc: 'Access list member removal failure',
    format: ({ access_list_name, members, updated_by }) =>
      `User [${updated_by}] failed to remove ${formatMembers(
        members
      )} from access list [${access_list_name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST]: {
    type: 'access_list.member.delete_all_members',
    desc: 'All members removed from access list',
    format: ({ access_list_name, updated_by }) =>
      `User [${updated_by}] removed all members from access list [${access_list_name}]`,
  },
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST_FAILURE]: {
    type: 'access_list.member.delete_all_members',
    desc: 'Access list member delete all members failure',
    format: ({ access_list_name, updated_by }) =>
      `User [${updated_by}] failed to remove all members from access list [${access_list_name}]`,
  },
  [eventCodes.USER_LOGIN_INVALID_ACCESS_LIST]: {
    type: 'user_login.invalid_access_list',
    desc: 'Access list skipped.',
    format: ({ access_list_name, user, missing_roles }) =>
      `Access list [${access_list_name}] is invalid and was skipped for member [${user}] because it references non-existent role${missing_roles.length > 1 ? 's' : ''} [${missing_roles}]`,
  },
  [eventCodes.SECURITY_REPORT_AUDIT_QUERY_RUN]: {
    type: 'secreports.audit.query.run"',
    desc: 'Access Monitoring Query Executed',
    format: ({ user, query }) =>
      `User [${user}] executed Access Monitoring query [${truncateStr(
        query,
        80
      )}]`,
  },
  [eventCodes.SECURITY_REPORT_RUN]: {
    type: 'secreports.report.run""',
    desc: 'Access Monitoring Report Executed',
    format: ({ user, name }) =>
      `User [${user}] executed [${name}] access monitoring report`,
  },
  [eventCodes.EXTERNAL_AUDIT_STORAGE_ENABLE]: {
    type: 'external_audit_storage.enable',
    desc: 'External Audit Storage Enabled',
    format: ({ updated_by }) =>
      `User [${updated_by}] enabled External Audit Storage`,
  },
  [eventCodes.EXTERNAL_AUDIT_STORAGE_DISABLE]: {
    type: 'external_audit_storage.disable',
    desc: 'External Audit Storage Disabled',
    format: ({ updated_by }) =>
      `User [${updated_by}] disabled External Audit Storage`,
  },
  [eventCodes.SPIFFE_SVID_ISSUED]: {
    type: 'spiffe.svid.issued',
    desc: 'SPIFFE SVID Issued',
    format: ({ user, spiffe_id }) =>
      `User [${user}] issued SPIFFE SVID [${spiffe_id}]`,
  },
  [eventCodes.SPIFFE_SVID_ISSUED_FAILURE]: {
    type: 'spiffe.svid.issued',
    desc: 'SPIFFE SVID Issued Failure',
    format: ({ user, spiffe_id }) =>
      `User [${user}] failed to issue SPIFFE SVID [${spiffe_id}]`,
  },
  [eventCodes.AUTH_PREFERENCE_UPDATE]: {
    type: 'auth_preference.update',
    desc: 'Cluster Authentication Preferences Updated',
    format: ({ user }) =>
      `User [${user}] updated the cluster authentication preferences`,
  },
  [eventCodes.CLUSTER_NETWORKING_CONFIG_UPDATE]: {
    type: 'cluster_networking_config.update',
    desc: 'Cluster Networking Configuration Updated',
    format: ({ user }) =>
      `User [${user}] updated the cluster networking configuration`,
  },
  [eventCodes.SESSION_RECORDING_CONFIG_UPDATE]: {
    type: 'session_recording_config.update',
    desc: 'Session Recording Configuration Updated',
    format: ({ user }) =>
      `User [${user}] updated the cluster session recording configuration`,
  },
  [eventCodes.ACCESS_GRAPH_PATH_CHANGED]: {
    type: 'access_graph.path.changed',
    desc: 'Access Path Changed',
    format: ({
      affected_resource_kind,
      affected_resource_name,
      affected_resource_source,
    }) =>
      `Access path for ${affected_resource_kind || 'Node'} [${affected_resource_name}/${affected_resource_source}] changed`,
  },
  [eventCodes.SPANNER_RPC]: {
    type: 'db.session.spanner.rpc',
    desc: 'Spanner RPC',
    format: ({ args, user, procedure, db_name, db_service }) => {
      if (args.sql) {
        return `User [${user}] executed query [${truncateStr(
          args.sql,
          80
        )}] in database [${db_name}] on [${db_service}]`;
      }
      return `User [${user}] called [${procedure}] in database [${db_name}] on [${db_service}]`;
    },
  },
  [eventCodes.SPANNER_RPC_DENIED]: {
    type: 'db.session.spanner.rpc',
    desc: 'Spanner RPC Denied',
    format: ({ args, user, procedure, db_name, db_service }) => {
      if (args.sql) {
        return `User [${user}] attempted to execute query [${truncateStr(
          args.sql,
          80
        )}] in database [${db_name}] on [${db_service}]`;
      }
      return `User [${user}] attempted to call [${procedure}] in database [${db_name}] on [${db_service}]`;
    },
  },
  [eventCodes.DISCOVERY_CONFIG_CREATE]: {
    type: 'discovery_config.create',
    desc: 'Discovery Config Created',
    format: ({ user, name }) => {
      return `User [${user}] created a discovery config [${name}]`;
    },
  },
  [eventCodes.DISCOVERY_CONFIG_UPDATE]: {
    type: 'discovery_config.update',
    desc: 'Discovery Config Updated',
    format: ({ user, name }) => {
      return `User [${user}] updated a discovery config [${name}]`;
    },
  },
  [eventCodes.DISCOVERY_CONFIG_DELETE]: {
    type: 'discovery_config.delete',
    desc: 'Discovery Config Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted a discovery config [${name}]`;
    },
  },
  [eventCodes.DISCOVERY_CONFIG_DELETE_ALL]: {
    type: 'discovery_config.delete_all',
    desc: 'All Discovery Configs Deleted',
    format: ({ user }) => {
      return `User [${user}] deleted all discovery configs`;
    },
  },
  [eventCodes.INTEGRATION_CREATE]: {
    type: 'integration.create',
    desc: 'Integration Created',
    format: ({ user, name }) => {
      return `User [${user}] created an integration [${name}]`;
    },
  },
  [eventCodes.INTEGRATION_UPDATE]: {
    type: 'integration.update',
    desc: 'Integration Updated',
    format: ({ user, name }) => {
      return `User [${user}] updated an integration [${name}]`;
    },
  },
  [eventCodes.INTEGRATION_DELETE]: {
    type: 'integration.delete',
    desc: 'Integration Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted an integration [${name}]`;
    },
  },
  [eventCodes.STATIC_HOST_USER_CREATE]: {
    type: 'static_host_user.create',
    desc: 'Static Host User Created',
    format: ({ user, name }) => {
      return `User [${user}] created a static host user [${name}]`;
    },
  },
  [eventCodes.STATIC_HOST_USER_UPDATE]: {
    type: 'static_host_user.update',
    desc: 'Static Host User Updated',
    format: ({ user, name }) => {
      return `User [${user}] updated a static host user [${name}]`;
    },
  },
  [eventCodes.STATIC_HOST_USER_DELETE]: {
    type: 'static_host_user.delete',
    desc: 'Static Host User Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted a static host user [${name}]`;
    },
  },
  [eventCodes.CROWN_JEWEL_CREATE]: {
    type: 'access_graph.crown_jewel.create',
    desc: 'Crown Jewel Created',
    format: ({ user, name }) => {
      return `User [${user}] created a crown jewel [${name}]`;
    },
  },
  [eventCodes.CROWN_JEWEL_UPDATE]: {
    type: 'access_graph.crown_jewel.update',
    desc: 'Crown Jewel Updated',
    format: ({ user, name }) => {
      return `User [${user}] updated a crown jewel [${name}]`;
    },
  },
  [eventCodes.CROWN_JEWEL_DELETE]: {
    type: 'access_graph.crown_jewel.delete',
    desc: 'Crown Jewel Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted a crown jewel [${name}]`;
    },
  },
  [eventCodes.USER_TASK_CREATE]: {
    type: 'user_task.create',
    desc: 'User Task Created',
    format: ({ user, name }) => {
      return `User [${user}] created a user task [${name}]`;
    },
  },
  [eventCodes.USER_TASK_UPDATE]: {
    type: 'user_task.update',
    desc: 'User Task Updated',
    format: ({ user, name }) => {
      return `User [${user}] updated a user task [${name}]`;
    },
  },
  [eventCodes.USER_TASK_DELETE]: {
    type: 'user_task.delete',
    desc: 'User Task Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted a user task [${name}]`;
    },
  },
  [eventCodes.SFTP_SUMMARY]: {
    type: 'sftp_summary',
    desc: 'File Transfer Completed',
    format: ({ user, server_hostname }) => {
      return `User [${user}] completed a file transfer on [${server_hostname}]`;
    },
  },
  [eventCodes.PLUGIN_CREATE]: {
    type: 'plugin.create',
    desc: 'Plugin Created',
    format: ({ user, name, plugin_type }) => {
      return `User [${user}] created a plugin [${name}] of type [${plugin_type}]`;
    },
  },
  [eventCodes.PLUGIN_UPDATE]: {
    type: 'plugin.update',
    desc: 'Plugin Updated',
    format: ({ user, name, plugin_type }) => {
      return `User [${user}] updated a plugin [${name}] of type [${plugin_type}]`;
    },
  },
  [eventCodes.PLUGIN_DELETE]: {
    type: 'plugin.delete',
    desc: 'Plugin Deleted',
    format: ({ user, name }) => {
      return `User [${user}] deleted a plugin [${name}]`;
    },
  },
  [eventCodes.CONTACT_CREATE]: {
    type: 'contact.create',
    desc: 'Contact Created',
    format: ({ user, email, contact_type }) => {
      return `User [${user}] created a [${contactTypeStr(contact_type)}] contact [${email}]`;
    },
  },
  [eventCodes.CONTACT_DELETE]: {
    type: 'contact.delete',
    desc: 'Contact Deleted',
    format: ({ user, email, contact_type }) => {
      return `User [${user}] deleted a [${contactTypeStr(contact_type)}] contact [${email}]`;
    },
  },
  [eventCodes.UNKNOWN]: {
    type: 'unknown',
    desc: 'Unknown Event',
    format: ({ unknown_type, unknown_code }) =>
      `Unknown '${unknown_type}' event (${unknown_code})`,
  },
  [eventCodes.GIT_COMMAND]: {
    type: 'git.command',
    desc: 'Git Command',
    format: ({ user, service, path, actions }) => {
      // "git-upload-pack" are fetches like "git fetch", "git pull".
      if (service === 'git-upload-pack') {
        return `User [${user}] has fetched from [${path}]`;
      }
      // "git-receive-pack" are pushes. Usually it should have one action.
      if (service === 'git-receive-pack') {
        if (actions && actions.length == 1) {
          switch (actions[0].action) {
            case 'delete':
              return `User [${user}] has deleted [${actions[0].reference}] from [${path}]`;
            case 'create':
              return `User [${user}] has created [${actions[0].reference}] on [${path}]`;
            case 'update':
              return `User [${user}] has updated [${actions[0].reference}] to [${actions[0].new.substring(0, 7)}] on [${path}]`;
          }
        }
        return `User [${user}] has attempted a push to [${path}]`;
      }
      if (service && path) {
        return `User [${user}] has executed a Git Command [${service}] at [${path}]`;
      }
      return `User [${user}] has executed a Git Command`;
    },
  },
  [eventCodes.GIT_COMMAND_FAILURE]: {
    type: 'git.command',
    desc: 'Git Command Failed',
    format: ({ user, exitError, service, path }) => {
      return `User [${user}] Git Command [${service}] at [${path}] failed [${exitError}]`;
    },
  },
  [eventCodes.STABLE_UNIX_USER_CREATE]: {
    type: 'stable_unix_user.create',
    desc: 'Stable UNIX user created',
    format: ({ stable_unix_user: { username } }) => {
      return `Stable UNIX user for username [${username}] was created`;
    },
  },
  [eventCodes.AUTOUPDATE_CONFIG_CREATE]: {
    type: 'auto_update_config.create',
    desc: 'Automatic Update Config Created',
    format: ({ user }) => {
      return `User ${user} created the Automatic Update Config`;
    },
  },
  [eventCodes.AUTOUPDATE_CONFIG_UPDATE]: {
    type: 'auto_update_config.update',
    desc: 'Automatic Update Config Updated',
    format: ({ user }) => {
      return `User ${user} updated the Automatic Update Config`;
    },
  },
  [eventCodes.AUTOUPDATE_CONFIG_DELETE]: {
    type: 'auto_update_config.delete',
    desc: 'Automatic Update Config Deleted',
    format: ({ user }) => {
      return `User ${user} deleted the Automatic Update Config`;
    },
  },
  [eventCodes.AUTOUPDATE_VERSION_CREATE]: {
    type: 'auto_update_version.create',
    desc: 'Automatic Update Version Created',
    format: ({ user }) => {
      return `User ${user} created the Automatic Update Version`;
    },
  },
  [eventCodes.AUTOUPDATE_VERSION_UPDATE]: {
    type: 'auto_update_version.update',
    desc: 'Automatic Update Version Updated',
    format: ({ user }) => {
      return `User ${user} updated the Automatic Update Version`;
    },
  },
  [eventCodes.AUTOUPDATE_VERSION_DELETE]: {
    type: 'auto_update_version.delete',
    desc: 'Automatic Update Version Deleted',
    format: ({ user }) => {
      return `User ${user} deleted the Automatic Update Version`;
    },
  },
};

const unknownFormatter = {
  desc: 'Unknown',
  format: () => 'Unknown',
};

export default function makeEvent(json: any): Event {
  // lookup event formatter by code
  const formatter = formatters[json.code as EventCode] || unknownFormatter;
  return {
    codeDesc:
      typeof formatter.desc === 'function'
        ? formatter.desc(json)
        : formatter.desc,
    message: formatter.format(json as any),
    id: getId(json),
    code: json.code,
    user: json.user,
    time: new Date(json.time),
    raw: json,
  };
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

function formatMembers(members: { member_name: string }[]) {
  const memberNames = members.map(m => m.member_name);
  const memberNamesJoined = memberNames.join(', ');

  return `${pluralize(memberNames.length, 'member')} [${memberNamesJoined}]`;
}

function contactTypeStr(type: number): string {
  switch (type) {
    case 1:
      return 'Business';
    case 2:
      return 'Security';
    default:
      return `Unknown type: ${type}`;
  }
}
