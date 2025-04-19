import { Option } from 'shared/components/Select';
import { RequestState } from 'shared/services/accessRequests';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { parseQuotedWordsDelimitedByComma } from 'shared/utils/parseString';

const ACCESS_REQUEST_SPEC_ROLES = 'access_request.spec.roles';
const USER_TRAITS = 'user.traits';

export enum AccessRequestMatchCondition {
  /**
   * MatchAllRoles indicates that all requested roles must match
   * the specified list of roles.
   */
  MatchAllRoles = 'match-all-roles',

  /**
   * MatchAnyRoles indicates that there must be at least one requested
   * role that matches the specified list of roles.
   */
  MatchAnyRoles = 'match-any-roles',

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
      value: AccessRequestMatchCondition.MatchAllRoles,
      label: 'Notify when all requested roles match',
    },
    {
      value: AccessRequestMatchCondition.MatchAnyRoles,
      label: 'Notify when any requested roles match',
    },
    {
      value: AccessRequestMatchCondition.AnyRoles,
      label: 'Notify when any roles requested',
    },
  ];

export type AccessRequestStateOption = {
  value: RequestState;
  label: RequestState;
};

export type RuleCondition = {
  rolesCondition?: RolesCondition;
  traitsCondition?: TraitsCondition[];
  errors?: string[];
};

export type RolesCondition = {
  field: AccessRequestMatchConditionOption;
  values: Option[];
};

export type TraitsCondition = {
  field: Option<string>;
  values: Option[];
};

function getRolesCondition(condition: string): RolesCondition | null {
  // Default to role condition.
  if (!condition) {
    return {
      field: accessRequestMatchConditionOptions.find(
        a => a.value === AccessRequestMatchCondition.MatchAllRoles
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

  const containsAllRolesRegex =
    /contains_all\(set\((?<set>"[^)]+")\), access_request.spec.roles\)/;

  const templateMatchContainsAllRoles = containsAllRolesRegex.exec(condition);
  const gotContainsAllRoles = templateMatchContainsAllRoles?.groups?.set;
  if (gotContainsAllRoles) {
    const roles = parseQuotedWordsDelimitedByComma(gotContainsAllRoles);
    if (roles.length !== 0) {
      return {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAllRoles
        ),
        values: roles.map(v => ({
          label: v,
          value: v,
        })),
      };
    }
  }

  const containsAnyRolesRegex =
    /contains_any\(access_request.spec.roles, set\((?<set>"[^)]+")\)/;

  const templateMatchContainsAnyRoles = containsAnyRolesRegex.exec(condition);
  const gotContainsAnyRoles = templateMatchContainsAnyRoles?.groups?.set;
  if (gotContainsAnyRoles) {
    const roles = parseQuotedWordsDelimitedByComma(gotContainsAnyRoles);

    if (roles.length !== 0) {
      return {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAnyRoles
        ),
        values: roles.map(v => ({
          label: v,
          value: v,
        })),
      };
    }
  }

  return null;
}

function getTraitsCondition(condition: string): TraitsCondition[] | null {
  const containsTraitsRegex =
    /contains_any\(user.traits\["(?<trait>[^"]+)"\], set\((?<set>"[^)]+")\)/g;

  let match;
  const traitsCondition = [];

  while ((match = containsTraitsRegex.exec(condition)) !== null) {
    const traitLabel = match?.groups?.trait;
    const traitValueSet = match?.groups?.set;
    if (traitLabel && traitValueSet) {
      const traitValues = parseQuotedWordsDelimitedByComma(traitValueSet);
      traitsCondition.push({
        field: { label: traitLabel, value: traitLabel },
        values: traitValues.map(v => ({ label: v, value: v })),
      });
    }
  }

  if (traitsCondition.length === 0) {
    return null;
  }

  return traitsCondition;
}

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
  const errors: string[] = [];

  // Verify allowed functions
  const allowedFunctions = new Set([
    'contains_any',
    'contains_all',
    'set',
    'is_empty',
  ]);
  const functionRegex = /(?<func>[a-zA-Z_]\w*)\s*\(/g;

  condition?.matchAll(functionRegex).forEach(match => {
    const fnName = match?.groups?.func;
    if (!allowedFunctions.has(fnName)) {
      errors.push(`Unknown function "${fnName}"`);
    }
  });

  // Verify roles condition is parsed
  const rolesCondition = getRolesCondition(condition);
  if (rolesCondition == null) {
    errors.push('Role Match Condition is required');
  }

  const traitsCondition = getTraitsCondition(condition);

  if (rolesCondition === null && traitsCondition === null && errors === null) {
    return null;
  }

  return {
    rolesCondition,
    traitsCondition,
    errors,
  };
}

function convertRolesConditionToPredicateExpression(
  rolesCondition: RolesCondition
) {
  if (!rolesCondition || !rolesCondition.field) {
    return ``;
  }

  switch (rolesCondition.field.value) {
    case AccessRequestMatchCondition.MatchAllRoles: {
      if (rolesCondition.values.length === 0) {
        return ``;
      }
      const joinedRoles = rolesCondition.values
        .map(v => `"${v.value}"`)
        .join(', ');
      return `contains_all(set(${joinedRoles}), ${ACCESS_REQUEST_SPEC_ROLES})`;
    }
    case AccessRequestMatchCondition.MatchAnyRoles: {
      if (rolesCondition.values.length === 0) {
        return ``;
      }
      const joinedRoles = rolesCondition.values
        .map(v => `"${v.value}"`)
        .join(', ');
      return `contains_any(${ACCESS_REQUEST_SPEC_ROLES}, set(${joinedRoles}))`;
    }
    case AccessRequestMatchCondition.AnyRoles: {
      return `!is_empty(${ACCESS_REQUEST_SPEC_ROLES})`;
    }
    default:
      assertUnreachable(rolesCondition.field.value);
  }
}

function convertTraitsConditionToPredicateExpression(
  traitsCondition?: TraitsCondition[]
) {
  if (!traitsCondition || traitsCondition.length === 0) {
    return ``;
  }

  const expressions = traitsCondition.map(v => {
    const joinedTraits = v.values.map(v => `"${v.value}"`).join(', ');
    return `contains_any(${USER_TRAITS}["${v.field.value}"], set(${joinedTraits}))`;
  });

  return expressions.join(` &&\n`);
}

export function convertRuleConditionToPredicateExpression(
  ruleCondition: RuleCondition
) {
  if (!ruleCondition) {
    return ``;
  }

  const rolesExpression = convertRolesConditionToPredicateExpression(
    ruleCondition.rolesCondition
  );
  const traitsExpression = convertTraitsConditionToPredicateExpression(
    ruleCondition.traitsCondition
  );

  if (rolesExpression !== '' && traitsExpression !== '') {
    return `${rolesExpression} &&\n${traitsExpression}`;
  }

  if (rolesExpression !== '') {
    return rolesExpression;
  }

  if (traitsExpression !== '') {
    return `${traitsExpression}`;
  }

  return ``;
}
