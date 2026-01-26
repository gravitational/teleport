import { ResourceAccessKind } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';

import {
  PluginTypeAwsIdentityCenter,
  StandardRoleConditions,
} from '../role/conditions';
import { hasLabels, TeleportOriginLabelKey } from '../role/label';
import {
  DefinableResourceAccessFields,
  ListResourceAccessFields,
} from '../role/listaccess';
import { getGitHubOrgs } from '../role/resources/github';

export const awsIcSubKindPredicate = `resource.sub_kind == "aws_ic_account"`;

export function getPredicateExpression(
  props:
    | {
        // A user cannot affect the filter for aws IC apps.
        accessField: Extract<DefinableResourceAccessFields, 'awsIc'>;
        roleConditions?: never;
      }
    | {
        accessField: ListResourceAccessFields;
        roleConditions: StandardRoleConditions;
      }
) {
  if (props.accessField === 'awsIc') {
    return awsIcSubKindPredicate;
  }

  const { accessField, roleConditions } = props;

  let baseQuery = '';
  switch (accessField) {
    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
      const labelLookup = props.roleConditions[accessField];
      if (!hasLabels(labelLookup)) {
        break;
      }
      if (labelLookup['*']?.includes('*')) {
        return ''; // match by any labels
      }
      baseQuery = Object.keys(labelLookup || {})
        .map(labelKey => {
          let labelVals = labelLookup[labelKey];
          if (!Array.isArray(labelVals)) {
            labelVals = [labelVals];
          }

          if (labelVals.includes('*')) {
            // match by any value matching labelKey
            return `exists(labels["${labelKey}"])`;
          }

          // Multi value for a given label key is an OR operation.
          const multiValQuery = labelVals.map(
            val => `labels["${labelKey}"] == "${val}"`
          );
          return `(${multiValQuery.join(' || ')})`;
        })
        // Multi label is an AND operation.
        .join(' && ');
      break;

    case 'github_permissions':
      const ghPerms = roleConditions.github_permissions;
      if (!ghPerms.length) {
        break;
      }

      if (ghPerms.some(ghPerm => ghPerm.orgs.includes('*'))) {
        return ''; // match by any gh perms
      }

      baseQuery = getGitHubOrgs(ghPerms)
        .map(org => `search("${org}")`)
        .join(' || ');

      break;

    default:
      accessField satisfies never;
  }

  // Some resources need extra filtering.
  switch (accessField) {
    case 'app_labels':
      // app_labels from the "StandardRoleConditions" is referring to all apps
      // excluding special kinds like AWS IC apps (which have it's own condition
      // AwsIcRoleConditions)
      const withoutAwsIcApp = `labels["${TeleportOriginLabelKey}"] != "${PluginTypeAwsIdentityCenter}"`;
      if (baseQuery) {
        baseQuery = `(${baseQuery}) && ${withoutAwsIcApp}`;
      } else {
        baseQuery = withoutAwsIcApp;
      }
      break;

    case 'db_labels':
    case 'github_permissions':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
      break;

    default:
      accessField satisfies never;
  }

  return baseQuery;
}

/**
 * Returns the kind that is supported by the unified resources query API.
 */
export function getUnifiedResourceKind(
  kind: DefinableResourceAccessFields
): ResourceAccessKind {
  switch (kind) {
    // AWS IC is an application
    case 'awsIc':
    case 'app_labels':
      return 'app';

    case 'db_labels':
      return 'db';

    case 'github_permissions':
      return 'git_server';

    case 'kubernetes_labels':
      return 'kube_cluster';

    case 'node_labels':
      return 'node';

    case 'windows_desktop_labels':
      return 'windows_desktop';

    default:
      kind satisfies never;
  }
}
