import { Option } from 'shared/components/Select';
import { RequestState } from 'shared/services/accessRequests';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { parseQuotedWordsDelimitedByComma } from 'shared/utils/parseString';

const ACCESS_REQUEST_SPEC_ROLES = 'access_request.spec.roles';

export enum AccessRequestMatchCondition {
  Roles = ACCESS_REQUEST_SPEC_ROLES,
  /**
   * AnyXXX means user wants to define a notification
   * where any XXX types trigger a notification.
   * eg: AnyRoles means any role requested will trigger
   * a notification.
   */
  AnyRoles = 'any-roles',
}

export type AccessRequestMatchConditionOption = {
  value: AccessRequestMatchCondition;
  label: string;
};

export const accessRequestMatchConditionOptions: AccessRequestMatchConditionOption[] =
  [
    {
      value: AccessRequestMatchCondition.Roles,
      label: 'Notify on specific roles requested',
    },
    {
      value: AccessRequestMatchCondition.AnyRoles,
      label: 'Notify on any roles requested',
    },
  ];

export type AccessRequestStateOption = {
  value: RequestState;
  label: RequestState;
};

export type RuleCondition = {
  field: AccessRequestMatchConditionOption;
  values: Option[];
};

/**
 * Tries to parse the provided predicate expression (condition)
 * to see if it conforms to what the web UI expects.
 * If it doesn't exactly conform, returns null to mean
 * it couldn't be parsed.
 *
 * Some examples of parsable predicate expression:
 *  - contains_any(access_request.spec.roles, set("access","editor"))
 *  - !is_empty(access_request.spec.roles)
 */
export function getRuleCondition(condition: string): RuleCondition | null {
  // Default to role condition.
  if (!condition) {
    return {
      field: accessRequestMatchConditionOptions.find(
        a => a.value === AccessRequestMatchCondition.Roles
      ),
      values: [],
    };
  }

  // Parse the provided predicate condition to see if
  // it conforms to what the web UI expects.
  // If it doesn't exactly conform, return null to mean
  // it couldn't be parsed.

  if (condition === `!is_empty(${ACCESS_REQUEST_SPEC_ROLES})`) {
    return {
      field: accessRequestMatchConditionOptions.find(
        a => a.value === AccessRequestMatchCondition.AnyRoles
      ),
      values: undefined, // there are no values for `AnyRoles`
    };
  }

  const containsRolesRegex =
    /^contains_any\(access_request.spec.roles, set\((?<set>".*")\){2}$/;

  const templateMatch = containsRolesRegex.exec(condition);
  const gotRoles = templateMatch?.groups?.set;
  if (!gotRoles) {
    return null;
  }

  const roles = parseQuotedWordsDelimitedByComma(gotRoles);
  if (roles.length === 0) {
    return null;
  }

  return {
    field: accessRequestMatchConditionOptions.find(
      a => a.value === AccessRequestMatchCondition.Roles
    ),
    values: roles.map(v => ({
      label: v,
      value: v,
    })),
  };
}

export function convertRuleConditionToPredicateExpression(
  ruleCondition: RuleCondition
) {
  if (!ruleCondition || !ruleCondition.field) {
    return ``;
  }

  switch (ruleCondition.field.value) {
    case AccessRequestMatchCondition.Roles: {
      if (ruleCondition.values.length === 0) {
        return ``;
      }
      const joinedRoles = ruleCondition.values
        .map(v => `"${v.value}"`)
        .join(', ');
      return `contains_any(${ACCESS_REQUEST_SPEC_ROLES}, set(${joinedRoles}))`;
    }
    case AccessRequestMatchCondition.AnyRoles: {
      return `!is_empty(${ACCESS_REQUEST_SPEC_ROLES})`;
    }
    default:
      assertUnreachable(ruleCondition.field.value);
  }
}
