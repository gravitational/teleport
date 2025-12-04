import {
  AccessRequestMatchCondition,
  accessRequestMatchConditionOptions,
  convertRuleConditionToPredicateExpression,
  getNotificationRuleCondition,
  getReviewRuleCondition,
  RuleCondition,
} from './rulecondition';

describe('getNotificationRuleCondition', () => {
  const predicates: {
    name: string;
    predicate: string;
    cond: RuleCondition;
  }[] = [
    {
      name: 'valid empty predicate, returns default',
      predicate: '',
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAllRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [],
        },
        traitsCondition: null,
      },
    },
    {
      name: 'valid is_empty predicate',
      predicate: '!is_empty(access_request.spec.roles)',
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.AnyRoles
            ).label,
            value: AccessRequestMatchCondition.AnyRoles,
          },
          values: undefined,
        },
        traitsCondition: null,
      },
    },
    {
      name: 'valid contains_any predicate',
      predicate:
        'contains_any(access_request.spec.roles, set("access","editor"))',
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAnyRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAnyRoles,
          },
          values: [
            { label: 'access', value: 'access' },
            { label: 'editor', value: 'editor' },
          ],
        },
        traitsCondition: null,
      },
    },
    {
      name: 'valid contains_all predicate',
      predicate:
        'contains_all(set("access","editor"), access_request.spec.roles)',
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAllRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [
            { label: 'access', value: 'access' },
            { label: 'editor', value: 'editor' },
          ],
        },
        traitsCondition: null,
      },
    },
  ];
  test.each(predicates)('$name', ({ predicate, cond }) => {
    const got = getNotificationRuleCondition(predicate);
    expect(got).toEqual(cond);
  });
});

describe('getNotificationRuleCondition invalid inputs', () => {
  const invalidPredicates: { name: string; str: string }[] = [
    {
      name: 'does not match "contains_any" template',
      str: `'contains_any(access.spec.roles, set("access","editor"))'`,
    },
    {
      name: 'does not match "is_empty" template',
      str: `'is_empty(access_request.spec.roles)'`,
    },
    {
      name: 'no roles found',
      str: 'contains_any(access_request.spec.roles, set())',
    },
    {
      name: 'no named group "set" found',
      str: `'contains_any(access_request.spec.roles, list("access","editor"))'`,
    },
    {
      name: 'does not match "contains_all" template',
      str: `'contains_all(set("access","editor"), access.spec.roles))'`,
    },
    {
      name: 'no roles found (contains_all)',
      str: `'contains_all(set(), access_request.spec.roles))'`,
    },
    {
      name: 'no named group "set" found (contains_all)',
      str: `'contains_all(list("access","editor"), access_request.spec.roles))'`,
    },
    {
      name: 'invalid roles condition',
      str: `contains_any(access_request.spec.roles, ("access", "editor"))`,
    },
    {
      name: 'invalid function',
      str: `contains__any(access_request.spec.roles, set("access", "editor"))`,
    },
    {
      name: 'traits condition not supported',
      str: `|-
          contains_any(access_request.spec.roles, set("access", "editor")) &&
          contains_any(user.traits["team"], set("Cloud"))`,
    },
  ];
  test.each(invalidPredicates)('$name', ({ str }) => {
    const got = getNotificationRuleCondition(str);
    expect(got).toBeNull();
  });
});

describe('getReviewRuleCondition', () => {
  const predicates: {
    name: string;
    predicate: string;
    cond: RuleCondition;
  }[] = [
    {
      name: 'valid empty predicate, returns default',
      predicate: '',
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAllRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [],
        },
        traitsCondition: null,
        resourcesCondition: null,
      },
    },
    {
      name: 'single trait',
      predicate: `|-
        contains_all(set("access"), access_request.spec.roles) &&
        contains_any(user.traits["level"], set("L1"))`,
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAllRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [{ label: 'access', value: 'access' }],
        },
        traitsCondition: [
          {
            traitKey: { label: 'level', value: 'level' },
            traitValues: [{ label: 'L1', value: 'L1' }],
          },
        ],
        resourcesCondition: [],
      },
    },
    {
      name: 'single trait with multiple values',
      predicate: `
        contains_all(set("access"), access_request.spec.roles) &&
        contains_any(user.traits["level"], set("L1", "L2"))`,
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAllRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [{ label: 'access', value: 'access' }],
        },
        traitsCondition: [
          {
            traitKey: { label: 'level', value: 'level' },
            traitValues: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
        ],
        resourcesCondition: [],
      },
    },
    {
      name: 'multiple traits',
      predicate: `
        contains_all(set("access"), access_request.spec.roles) &&
        contains_any(user.traits["level"], set("L1", "L2")) &&
        contains_any(user.traits["team"], set("Cloud"))`,
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAllRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [{ label: 'access', value: 'access' }],
        },
        traitsCondition: [
          {
            traitKey: { label: 'level', value: 'level' },
            traitValues: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
          {
            traitKey: { label: 'team', value: 'team' },
            traitValues: [{ label: 'Cloud', value: 'Cloud' }],
          },
        ],
        resourcesCondition: [],
      },
    },
    {
      name: 'resource labels condition',
      predicate: `
        contains_all(set("access"), access_request.spec.roles) &&
        access_request.spec.resource_labels_intersection["env"].contains("dev") &&
        access_request.spec.resource_labels_intersection["service"].contains("test")`,
      cond: {
        rolesCondition: {
          field: {
            label: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.MatchAllRoles
            ).label,
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [{ label: 'access', value: 'access' }],
        },
        traitsCondition: [],
        resourcesCondition: [
          {
            name: 'env',
            value: 'dev',
          },
          {
            name: 'service',
            value: 'test',
          },
        ],
      },
    },
  ];
  test.each(predicates)('$name', ({ predicate, cond }) => {
    const got = getReviewRuleCondition(predicate);
    expect(got).toEqual(cond);
  });
});

describe('getReviewRuleCondition invalid inputs', () => {
  const invalidPredicates: { name: string; str: string }[] = [
    {
      name: '!is_empty is not supported',
      str: '!is_empty(access_request.spec.roles)',
    },
    {
      name: 'does not match "contains_all" template',
      str: 'contains_all(access_request.spec.roles, set("access"))',
    },
    {
      name: 'no roles found',
      str: `
        contains_all(set(), access_request.spec.roles) &&
        contains_any(user.traits["team"], set("test"))`,
    },
    {
      name: 'no traits found',
      str: `
        contains_all(set("access"), access_request.spec.roles) &&
        contains_any(user.traits["team"], set())`,
    },
    {
      name: 'no named group "set" found',
      str: `
        contains_all(list("access"), access_request.spec.roles) &&
        contains_any(user.traits["team"], set("test"))`,
    },
    {
      name: 'missing roles condition',
      str: `
        contains_any(user.traits["level"], set("L1", "L2")) &&
        contains_any(user.traits["team"], set("Cloud"))`,
    },
  ];

  test.each(invalidPredicates)('$name', ({ str }) => {
    const got = getReviewRuleCondition(str);
    expect(got).toBeNull();
  });
});

describe('convertRuleConditionToPredicateExpression', () => {
  const ruleConditions: {
    name: string;
    cond: RuleCondition | null;
    exp: string;
  }[] = [
    {
      name: 'null returns empty string',
      cond: null,
      exp: '',
    },
    {
      name: 'field is empty (unlikely, but test it anyways)',
      cond: {
        rolesCondition: { field: null, values: [] },
      },
      exp: '',
    },
    {
      name: 'match condition roles, null values',
      cond: {
        rolesCondition: {
          field: {
            label: '',
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: null,
        },
      },
      exp: '',
    },
    {
      name: 'match condition roles, without any values',
      cond: {
        rolesCondition: {
          field: {
            label: '',
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [],
        },
      },
      exp: '',
    },
    {
      name: 'match condition contains_any roles',
      cond: {
        rolesCondition: {
          field: {
            label: '',
            value: AccessRequestMatchCondition.MatchAnyRoles,
          },
          values: [
            { value: 'access', label: 'access' },
            { value: 'editor', label: 'editor' },
          ],
        },
      },
      exp: 'contains_any(access_request.spec.roles, set("access", "editor"))',
    },
    {
      name: 'match condition contains_all roles',
      cond: {
        rolesCondition: {
          field: {
            label: '',
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [
            { value: 'access', label: 'access' },
            { value: 'editor', label: 'editor' },
          ],
        },
      },
      exp: 'contains_all(set("access", "editor"), access_request.spec.roles)',
    },
    {
      name: 'match condition ANY roles',
      cond: {
        rolesCondition: {
          field: { label: '', value: AccessRequestMatchCondition.AnyRoles },
          values: [],
        },
      },
      exp: '!is_empty(access_request.spec.roles)',
    },
    {
      name: 'match condition traits',
      cond: {
        rolesCondition: {
          field: {
            label: '',
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [
            { value: 'access', label: 'access' },
            { value: 'editor', label: 'editor' },
          ],
        },
        traitsCondition: [
          {
            traitKey: { label: '', value: 'level' },
            traitValues: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
          {
            traitKey: { label: '', value: 'team' },
            traitValues: [{ label: 'Cloud', value: 'Cloud' }],
          },
        ],
      },
      exp: `contains_all(set("access", "editor"), access_request.spec.roles) &&
contains_any(user.traits["level"], set("L1", "L2")) &&
contains_any(user.traits["team"], set("Cloud"))`,
    },
    {
      name: 'match resource traits',
      cond: {
        rolesCondition: {
          field: {
            label: '',
            value: AccessRequestMatchCondition.MatchAllRoles,
          },
          values: [
            { value: 'access', label: 'access' },
            { value: 'editor', label: 'editor' },
          ],
        },
        resourcesCondition: [{ name: 'env', value: 'dev' }],
      },
      exp: `contains_all(set("access", "editor"), access_request.spec.roles) &&
access_request.spec.resource_labels_intersection["env"].contains("dev")`,
    },
  ];

  test.each(ruleConditions)('$name', ({ cond, exp }) => {
    const got = convertRuleConditionToPredicateExpression(cond);
    expect(got).toBe(exp);
  });
});
