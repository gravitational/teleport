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

export const SystemRoleEnum = {
  TELE_ADMIN: '@teleadmin'
}

export const ResourceEnum = {
  SAML: 'saml',
  OIDC: 'oidc',
  ROLE: 'role',
  AUTH_CONNECTORS: 'auth_connector',
  TRUSTED_CLUSTER: 'trusted_cluster',
  LOG_FWRD: 'logforwarder'
}

export const AuthTypeEnum = {
  LOCAL: 'local',
  SSO: 'sso'
}

export const Auth2faTypeEnum = {
  UTF: 'u2f',
  OTP: 'otp',
  DISABLED: 'off'
}

export const AuthProviderTypeEnum = {
  OIDC: 'oidc',
  SAML: 'saml'
}

export const UserTokenTypeEnum = {
  RESET: 'reset',
  INVITE: 'invite'
}

export const RepositoryEnum = {
  SYSTEM: 'gravitational.io'
}

export const RemoteAccessEnum = {
  ON: 'on',
  OFF: 'off',
  NA: 'n/a'
}

export const RestRespCodeEnum = {
  FORBIDDEN: 403
}

export const ExpandPolicyEnum = {
  FIXED: 'fixed',
}

export const UserStatusEnum = {
  INVITED: 'invited',
  ACTIVE: 'active'
}

export const SiteReasonEnum = {
  INVALID_LICENSE: 'license_invalid'
}

export const ServerVarEnums = {
  INTERFACE: 'interface',
  MOUNT: 'mount',
  DOCKER_DISK: 'docker_device',
  GRAVITY_DISK: 'system_device'
}

export const OpTypeEnum = {
  OPERATION_UPDATE: 'operation_update',
  OPERATION_INSTALL: 'operation_install',
  OPERATION_EXPAND: 'operation_expand',
  OPERATION_UNINSTALL: 'operation_uninstall',
  OPERATION_SHRINK: 'operation_shrink'
}

export const OpStateEnum = {
  FAILED: 'failed',
  CREATED: 'created',
  COMPLETED: 'completed',
  READY: 'ready',
  INSTALL_PRECHECKS: 'install_prechecks',
  INSTALL_INITIATED: 'install_initiated',
  INSTALL_SETTING_CLUSTER_PLAN: 'install_setting_plan',
  INSTALL_PROVISIONING: 'install_provisioning',
  INSTALL_DEPLOYING: 'install_deploying',
  UNINSTALL_IN_PROGRESS: 'uninstall_in_progress',
  EXPAND_PRECHECKS: 'expand_prechecks',
  EXPAND_INITIATED: 'expand_initiated',
  EXPAND_SETTING_PLAN: 'expand_setting_plan',
  EXPAND_PLANSET: 'expand_plan_set',
  EXPAND_PROVISIONING: 'expand_provisioning',
  EXPAND_DEPLOYING: 'expand_deploying',
  SHRINK_IN_PROGRESS: 'shrink_in_progress',
  UPDATE_IN_PROGRESS: 'update_in_progress'
}

export const SiteStateEnum = {
  ACTIVE: 'active',
  FAILED: 'failed',
  DEGRADED: 'degraded',
  NOT_INSTALLED: 'not_installed',
  INSTALLING: 'installing',
  UPDATING: 'updating',
  SHRINKING: 'shrinking',
  EXPANDING: 'expanding',
  UNINSTALLING: 'uninstalling',
  OFFLINE: 'offline'
}

export const ProviderEnum = {
  ONPREM: 'onprem',
}

export const K8sPodPhaseEnum = {
  SUCCEEDED: 'Succeeded',
  RUNNING: 'Running',
  PENDING: 'Pending',
  FAILED: 'Failed',
  UNKNOWN: 'Unknown'
}

export const K8sPodDisplayStatusEnum = {
  ...K8sPodPhaseEnum,
  TERMINATED: 'Terminated'
}