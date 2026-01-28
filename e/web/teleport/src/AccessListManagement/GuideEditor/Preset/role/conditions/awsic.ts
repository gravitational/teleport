import { AccountAssignment, Labels, Role } from 'teleport/services/resources';

import { TeleportOriginLabelKey } from '../label';

export const PluginTypeAwsIdentityCenter = 'aws-identity-center';

/**
 * Const label automatically assigned to AWS IC apps by Teleport.
 * Users cannot modify this label or add their own labels on AWS IC apps.
 */
export const AwsIcAppLabel = {
  [TeleportOriginLabelKey]: PluginTypeAwsIdentityCenter,
} as const;

/**
 * Role conditions for AWS Identity Center apps.
 *
 * AWS IC requires a specific app_label (see AwsIcAppLabel) + selecting from
 * "known" account_assignments in the role. Because of this unique constraint,
 * AWS IC gets its own separate role rather than being included in the
 * standard role (see StandardRoleConditions).
 *
 * FYI: multi labels for a resource type is an AND operation in the role,
 * so app_labels for a AWS IC app and other applications cannot be combined.
 */
export type AwsIcRoleConditions = {
  /**
   * Maps to role field "app_labels".
   * Always set to AwsIcAppLabel const - users cannot modify.
   */
  labels: Labels;
  /**
   * Maps to role field "account_assignments" with friendly names.
   * An account ID can have access to multiple ARNs.
   */
  account: AwsAccountMap;
};

/**
 * Key is the AWS account ID and is also the name of its app resource.
 *
 * Value holds a list of allowed ARNs for this account.
 */
export type AwsAccountMap = Map<string, Set<string>>;

/**
 * Describes a AWS identity center app.
 * Type that is used to render rows of AWS IC apps.
 */
export type AwsIcApp = {
  /**
   * AWS account id, also the app "name"
   */
  accountId: string;
  /**
   * Friendly name of the AWS account id.
   * Field is not defined until the app is queried.
   */
  friendlyAccountName: string | undefined;
  /**
   * Map of allowed arns.
   *
   * Key is the ARN value.
   * Value is it's friendly name which will be undefined until
   * the app is queried.
   */
  arnMap: Map<string, string | undefined>;
};

export const defaultAwsIcRoleConditions = (): AwsIcRoleConditions => ({
  labels: AwsIcAppLabel,
  account: new Map(),
});

/**
 * Converts web version of "account_assignments" role field to the type
 * that the role resource accepts.
 */
export function convertAwsAccountMapToRoleType(
  account: AwsAccountMap
): AccountAssignment[] {
  if (account.size === 0) {
    return undefined;
  }

  const assignments: AccountAssignment[] = [];

  for (const [awsAccount, arns] of account) {
    arns.forEach(arn => {
      assignments.push({
        account: awsAccount,
        permission_set: arn,
      });
    });
  }

  return assignments;
}

/**
 * Converts role field "account_assignments" to the type that
 * the web UI expects.
 */
export function extractAwsIcRoleConditionsFromRole(
  role: Role
): AwsIcRoleConditions {
  const allow = role.spec.allow;
  const account: AwsAccountMap = new Map();

  allow.account_assignments.forEach(aws => {
    if (!account.has(aws.account)) {
      account.set(aws.account, new Set());
    }
    account.get(aws.account).add(aws.permission_set);
  });

  return {
    labels: allow.app_labels,
    account,
  };
}
