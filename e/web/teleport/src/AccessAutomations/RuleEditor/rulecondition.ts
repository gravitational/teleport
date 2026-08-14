import { Option } from 'shared/components/Select';
import { TraitsOption } from 'shared/components/TraitsEditor';
import { RequestState } from 'shared/services/accessRequests';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { parseQuotedWordsDelimitedByComma } from 'shared/utils/parseString';

import { Label } from 'teleport/types';

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
  traitsCondition?: TraitsOption[];
  resourcesCondition?: Label[];
};

export type RolesCondition = {
  field: AccessRequestMatchConditionOption;
  values: Option[];
};

/**
 * Tries to parse the provided notification rule predicate expression (condition)
 * to see if it conforms to what the web UI expects.
 * If it doesn't exactly conform, returns null to mean
 * it couldn't be parsed.
 *
 * Some examples of parsable predicate expression:
 * - !is_empty(access_request.spec.roles)
 * - contains_any(access_request.spec.roles, set("access","editor"))
 * - contains_all(set("access","editor"), access_request.spec.roles)
 */
export function getNotificationRuleCondition(condition: string): RuleCondition {
  let rolesCondition: RolesCondition;

  // Default to role condition.
  if (!condition) {
    return {
      rolesCondition: {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAllRoles
        ),
        values: [],
      },
      traitsCondition: null,
    };
  }

  if (condition === `!is_empty(${ACCESS_REQUEST_SPEC_ROLES})`) {
    rolesCondition = {
      field: accessRequestMatchConditionOptions.find(
        a => a.value === AccessRequestMatchCondition.AnyRoles
      ),
      values: undefined, // there are no values for `AnyRoles`
    };
  }

  const containsAnyRolesRegex =
    /^contains_any\(access_request.spec.roles, set\((?<set>"[^)]+")\)\)$/;
  const templateMatchContainsAnyRoles = containsAnyRolesRegex.exec(condition);
  const gotContainsAnyRoles = templateMatchContainsAnyRoles?.groups?.set;
  if (gotContainsAnyRoles) {
    const roles = parseQuotedWordsDelimitedByComma(gotContainsAnyRoles);
    if (roles.length !== 0) {
      rolesCondition = {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAnyRoles
        ),
        values: roles.map(v => ({ label: v, value: v })),
      };
    }
  }

  const containsAllRolesRegex =
    /^contains_all\(set\((?<set>"[^)]+")\), access_request.spec.roles\)$/;
  const templateMatchContainsAllRoles = containsAllRolesRegex.exec(condition);
  const gotContainsAllRoles = templateMatchContainsAllRoles?.groups?.set;
  if (gotContainsAllRoles) {
    const roles = parseQuotedWordsDelimitedByComma(gotContainsAllRoles);
    if (roles.length !== 0) {
      rolesCondition = {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAllRoles
        ),
        values: roles.map(v => ({ label: v, value: v })),
      };
    }
  }

  // Return null if we couldn't parse the condition.
  if (!rolesCondition) {
    return null;
  }

  return {
    rolesCondition,
    traitsCondition: null,
  };
}

/**
 * Tries to parse the provided review rule predicate expression (condition)
 * to see if it conforms to what the web UI expects.
 * If it doesn't exactly conform, returns null to mean
 * it couldn't be parsed. The predicate expression is expected to contain
 * one matching roles expression and at least one matching traits expression.
 *
 * Example of a parsable predicate expression:
 * - `contains_all(set("access","editor"), access_request.spec.roles) &&
 *   contains_any(user.traits["department"], set("engineering","sales"))`
 */
export function getReviewRuleCondition(condition: string): RuleCondition {
  let rolesCondition: RolesCondition;
  let traitsCondition: TraitsOption[] = [];
  let resourcesCondition: Label[] = [];

  // Default to role condition.
  if (!condition) {
    return {
      rolesCondition: {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAllRoles
        ),
        values: [],
      },
      traitsCondition: null,
      resourcesCondition: null,
    };
  }

  // Trim and split on &&
  const normalizedInput = condition.replace('|-', '').trim();
  const expressions = normalizedInput.split(/\s*&&\s*/);

  // Expect roles condition to be the first expression.
  const rolesConditionExpr = expressions[0] || '';
  const containsAllRolesRegex =
    /^contains_all\(set\((?<set>"[^)]+")\), access_request.spec.roles\)$/;
  const templateMatchContainsAllRoles =
    containsAllRolesRegex.exec(rolesConditionExpr);
  const gotContainsAllRoles = templateMatchContainsAllRoles?.groups?.set;
  if (gotContainsAllRoles) {
    const roles = parseQuotedWordsDelimitedByComma(gotContainsAllRoles);
    if (roles.length !== 0) {
      rolesCondition = {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAllRoles
        ),
        values: roles.map(v => ({ label: v, value: v })),
      };
    }
  }

  // Return null if we couldn't parse the roles condition.
  if (!rolesCondition) {
    return null;
  }

  // Parse remaining expressions for traits condition.
  const remainingExpr = expressions.slice(1);

  const containsTraitsRegex =
    /^contains_any\(user.traits\["(?<trait>[^"]+)"\], set\((?<set>"[^)]+")\)\)$/;

  remainingExpr.forEach(expr => {
    const templateMatchContainsTraits = containsTraitsRegex.exec(expr);
    const gotContainsTraits = templateMatchContainsTraits?.groups;
    if (gotContainsTraits) {
      const trait = gotContainsTraits.trait;
      const traitValues = parseQuotedWordsDelimitedByComma(
        gotContainsTraits.set
      );
      if (traitValues.length !== 0) {
        traitsCondition.push({
          traitKey: { label: trait, value: trait },
          traitValues: traitValues.map(v => ({ label: v, value: v })),
        });
      }
    }
  });

  const resourceLabelsRegex =
    /^access_request.spec.resource_labels_intersection\["(?<key>[^"]+)"\].contains\("(?<val>[^"]+)"\)$/;

  remainingExpr.forEach(expr => {
    const templateMatchResourceLabels = resourceLabelsRegex.exec(expr);
    const gotResourceLabels = templateMatchResourceLabels?.groups;
    if (gotResourceLabels) {
      resourcesCondition.push({
        name: gotResourceLabels.key,
        value: gotResourceLabels.val,
      });
    }
  });

  // Return null if we couldn't parse any of the remainig expressions.
  if (
    traitsCondition.length + resourcesCondition.length !==
    remainingExpr.length
  ) {
    return null;
  }

  return {
    rolesCondition,
    traitsCondition,
    resourcesCondition,
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
      if (!rolesCondition.values || rolesCondition.values.length === 0) {
        return ``;
      }
      const joinedRoles = rolesCondition.values
        .map(v => `"${v.value}"`)
        .join(', ');
      return `contains_all(set(${joinedRoles}), ${ACCESS_REQUEST_SPEC_ROLES})`;
    }
    case AccessRequestMatchCondition.MatchAnyRoles: {
      if (!rolesCondition.values || rolesCondition.values.length === 0) {
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
  traitsCondition?: TraitsOption[]
) {
  if (!traitsCondition || traitsCondition.length === 0) {
    return ``;
  }

  // Ignore empty traits
  traitsCondition = traitsCondition.filter(
    v => !!v.traitKey && v.traitValues.length > 0
  );

  const expressions = traitsCondition.map(v => {
    const joinedTraits = v.traitValues.map(v => `"${v.value}"`).join(', ');
    return `contains_any(${USER_TRAITS}["${v.traitKey.value}"], set(${joinedTraits}))`;
  });

  return expressions.join(` &&\n`);
}

function convertResourcesConditionToPredicateExpression(
  resourcesCondition?: Label[]
) {
  if (!resourcesCondition || resourcesCondition.length === 0) {
    return ``;
  }

  const expressions = resourcesCondition.map(label => {
    return `access_request.spec.resource_labels_intersection["${label.name}"].contains("${label.value}")`;
  });

  return expressions.join(` &&\n`);
}

export function convertRuleConditionToPredicateExpression(
  ruleCondition: RuleCondition
) {
  if (!ruleCondition) {
    return '';
  }

  const rolesExpression = convertRolesConditionToPredicateExpression(
    ruleCondition.rolesCondition
  );
  const traitsExpression = convertTraitsConditionToPredicateExpression(
    ruleCondition.traitsCondition
  );
  const resourcesExpression = convertResourcesConditionToPredicateExpression(
    ruleCondition.resourcesCondition
  );

  const expressions = [rolesExpression, traitsExpression, resourcesExpression]
    .filter(str => str !== '')
    .join(` &&\n`);
  return expressions;
}
