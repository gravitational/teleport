import { FunctionComponent, JSX } from 'react';

import { ResourceIconName } from 'design/ResourceIcon';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { BaseView } from 'teleport/components/Wizard/flow';
import { PluginKind } from 'teleport/services/integrations';
import { IntegrationEnrollKind } from 'teleport/services/userEvent';

/**
 * View describes the UI for a single configuration
 * step in the plugin enrollment.
 */
export type View = BaseView<{
  title: string;
  /**
   * If the component is not defined,
   * then the component who is processing
   * this view, will have to provide it's own component.
   *
   * Eg: before multi-step support, all plugins had only
   * one step using `SubmittablePluginForm.tsx`.
   * Some plugins will have multi-step (eg. okta)
   * where the first step stays the same as before
   * (`SubmittablePluginForm.tsx`), but have more
   * steps afterwwards.
   */
  component: FunctionComponent;
}>;

/**
 * PluginBase describes base type for OAuth,
 * Cloud and Selfhosted plugins.
 */
export type PluginBase = {
  type: PluginKind;
  name: string;
  icon: ResourceIconName;
  url: string;

  // isOAuth describes a plugin that are authenticated
  // via OAuth.
  isOAuth?: boolean;

  /**
   * Describes whether the plugin can be hosted by Teleport Cloud.
   */
  cloudHostable: boolean;

  /**
   * Describes whether the plugin can be self hosted.
   */
  selfHostable: boolean;

  disabledIfNoMdmSupport?: boolean;
  // todo (michellescripts) replace with new entitlement `EntraIDSync`
  requiresIgs?: boolean;

  /**
   * views represents all the views for each step
   * in a multi step plugin enrollment eg: okta.
   * If empty, the plugin has only one step.
   */
  views?(): View[];
};

/**
 * SelfHostedPlugin describes a plugin that can be self hosted.
 */
export type SelfHostedPlugin = PluginBase & {
  cloudHostable: false;
  selfHostable: true;
  disabledIfNoMdmSupport?: true;
};

/**
 * CloudHostablePlugin describes a plugin that can be hosted by Teleport Cloud.
 * CloudHostablePlugins might also be self hostable, determined by the `selfHostable`
 * field.
 */
export type CloudHostablePlugin = PluginBase & {
  cloudHostable: true;
  /** selfHostable is true for cloud plugins that can be self hosted as well. */
  selfHostable: boolean;

  // For each hosted plugin, these describe additional elements in the enroll page.
  fullName: string;
  Description?: () => JSX.Element;
  Setup?: () => JSX.Element;
  FormMixin?: (props: { attempt: Attempt }) => JSX.Element;
  NextSteps?: (props: { successData?: EnrollSuccessResponse }) => JSX.Element;
  permissions?: CategoryPermissions[];
  disabledIfNoMdmSupport?: boolean;
};

// EnrollSuccessResponse contains the necessary data to guide the user
// after the plugin is connected (in `NextSteps` component).
// This is equivalent to `pluginOnboardingCookieNonSensitiveData` in e/lib/web/plugins.go
export type EnrollSuccessResponse = {
  slack?: {
    fallback_channel: string;
  };
};

type CategoryPermissions = {
  category: string;
  permissions: Permission[];
};

type Permission = {
  title: string;
  description?: string;
};

export type PluginConfigOktaGroup = {
  // name is the name of the group.
  name: string;
  // description is the description of the group.
  description: string;
};

export type PluginConfigOktaApp = {
  name: string;
};

export function pluginTypeToIntegrationEnrollKind(
  p: PluginKind
): IntegrationEnrollKind {
  switch (p) {
    case 'discord':
      return IntegrationEnrollKind.Discord;
    case 'email':
      return IntegrationEnrollKind.Email;
    case 'jira':
      return IntegrationEnrollKind.Jira;
    case 'mattermost':
      return IntegrationEnrollKind.Mattermost;
    case 'msteams':
      return IntegrationEnrollKind.MsTeams;
    case 'pagerduty':
      return IntegrationEnrollKind.PagerDuty;
    case 'slack':
      return IntegrationEnrollKind.Slack;
    case 'okta':
      return IntegrationEnrollKind.Okta;
    case 'servicenow':
      return IntegrationEnrollKind.ServiceNow;
    case 'jamf':
      return IntegrationEnrollKind.Jamf;
    case 'opsgenie':
      return IntegrationEnrollKind.OpsGenie;
    case 'entra-id':
      return IntegrationEnrollKind.EntraId;
    case 'datadog':
      return IntegrationEnrollKind.DatadogIncidentManagement;
    default:
      return IntegrationEnrollKind.Unspecified;
  }
}

/**
 * PluginConfigBase defines base configuration field names required to
 * create plugin. Name format is an exact representation of the form
 * names defined in the backend.
 */
export enum PluginConfigBase {
  Name = 'name',
  Type = 'type',
  CSRFToken = 'csrf_token',
}

/**
 * PluginConfigAwsIc defines configuration field names used to
 * create AWS IAM Identity Center plugin. Name format is an exact
 * representation of the form names defined in the backend.
 */
export enum PluginConfigAwsIc {
  PluginName = 'aws-identity-center',
  OidcIntegrationName = 'oidcIntegrationName',
  InstanceArn = 'arn',
  InstanceRegion = 'region',
  AccessListDefaultOwners = 'accessListDefaultOwners',
  SamlServiceProviderMetadata = 'samlServiceProviderMetadata',
  SamlServiceProviderName = 'samlServiceProviderName',
  ScimBaseURL = 'scimBaseURL',
  ScimAccessToken = 'scimAccessToken',
  ResourceToValidate = 'resourceToValidate',
  ValidateSaml = 'validateSAML',
  ValidateScim = 'validateSCIM',
}

/**
 * AwsIcResourceTypes defines names for AWS Identity
 * Center resource types
 */
export enum AwsIcResourceTypes {
  Accounts = 'accounts',
  GroupsWithAssignments = 'groupsWithAssignments',
  PermissionSets = 'permissionSets',
  PermissionAssignments = 'permissionAssignments',
}

/**
 * AwsIcAccounts defines account fields that
 * are shown in the import resources accounts table.
 */
export type AwsIcAccounts = {
  name: string;
  arn: string;
  id: string;
  permissionSets: AwsIcPermissionSets[];
};

/**
 * AwsIcGroupsWithAssignment defines
 * AWS IAM Identity Center user groups with respective account
 * assignments.
 */
export type AwsIcGroupsWithAssignment = {
  name: string;
  assignments: AwsIcPermissionAssignments[];
};

/**
 * Assignments is an AWS IAM Identity Center account assignment.
 */
export type AwsIcPermissionAssignments = {
  permissionSetName: string;
  accountName: string;
};

/**
 * AwsIcPermissionSets is an AWS ideneity
 * center permission set.
 */
export type AwsIcPermissionSets = {
  name: string;
  description: string;
  arn: string;
};
