import {
  AccessRequestMatchCondition,
  accessRequestMatchConditionOptions,
  convertRuleConditionToPredicateExpression,
  getRuleCondition,
  RuleCondition,
} from './rulecondition';

describe('getRuleCondition', () => {
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
        errors: [],
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
        errors: [],
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
        errors: [],
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
        errors: [],
      },
    },
    {
      name: 'single trait',
      predicate: `
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
            field: { label: 'level', value: 'level' },
            values: [{ label: 'L1', value: 'L1' }],
          },
        ],
        errors: [],
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
            field: { label: 'level', value: 'level' },
            values: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
        ],
        errors: [],
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
            field: { label: 'level', value: 'level' },
            values: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
          {
            field: { label: 'team', value: 'team' },
            values: [{ label: 'Cloud', value: 'Cloud' }],
          },
        ],
        errors: [],
      },
    },
    {
      name: 'valid contains_all roles and traits in single line',
      predicate: `contains_all(set("access", "editor"), access_request.spec.roles) && contains_any(user.traits["level"], set("L1", "L2")) && contains_any(user.traits["team"], set("Cloud"))`,
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
        traitsCondition: [
          {
            field: { label: 'level', value: 'level' },
            values: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
          {
            field: { label: 'team', value: 'team' },
            values: [{ label: 'Cloud', value: 'Cloud' }],
          },
        ],
        errors: [],
      },
    },
    {
      name: 'valid contains_any roles and traits in single line',
      predicate: `contains_any(access_request.spec.roles, set("access", "editor")) && contains_any(user.traits["level"], set("L1", "L2")) && contains_any(user.traits["team"], set("Cloud"))`,
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
        traitsCondition: [
          {
            field: { label: 'level', value: 'level' },
            values: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
          {
            field: { label: 'team', value: 'team' },
            values: [{ label: 'Cloud', value: 'Cloud' }],
          },
        ],
        errors: [],
      },
    },
  ];
  predicates.forEach(test => {
    it(`${test.name}`, () => {
      const got = getRuleCondition(test.predicate);
      expect(got).toEqual(test.cond);
    });
  });
});

describe('getRuleCondition invalid inputs', () => {
  const invalidPredicates: { name: string; str: string; errors: string[] }[] = [
    {
      name: 'does not match "contains_any" template',
      str: `'contains_any(access.spec.roles, set("access","editor"))'`,
      errors: ['Role Match Condition is required'],
    },
    {
      name: 'does not match "is_empty" template',
      str: `'is_empty(access_request.spec.roles)'`,
      errors: ['Role Match Condition is required'],
    },
    {
      name: 'no roles found',
      str: 'contains_any(access_request.spec.roles, set())',
      errors: ['Role Match Condition is required'],
    },
    {
      name: 'no named group "set" found',
      str: `'contains_any(access_request.spec.roles, list("access","editor"))'`,
      errors: [`Unknown function "list"`, 'Role Match Condition is required'],
    },
    {
      name: 'does not match "contains_all" template',
      str: `'contains_all(set("access","editor"), access.spec.roles))'`,
      errors: ['Role Match Condition is required'],
    },
    {
      name: 'no roles found (contains_all)',
      str: `'contains_all(set(), access_request.spec.roles))'`,
      errors: ['Role Match Condition is required'],
    },
    {
      name: 'no named group "set" found (contains_all)',
      str: `'contains_all(list("access","editor"), access_request.spec.roles))'`,
      errors: [`Unknown function "list"`, `Role Match Condition is required`],
    },
    {
      name: 'invalid roles condition',
      str: `contains_any(access_request.spec.roles, ("access", "editor"))`,
      errors: ['Role Match Condition is required'],
    },
    {
      name: 'invalid function',
      str: `contains__any(access_request.spec.roles, set("access", "editor"))`,
      errors: [
        `Unknown function "contains__any"`,
        'Role Match Condition is required',
      ],
    },
    {
      name: 'missing roles condition',
      str: `
          contains_any(user.traits["level"], set("L1", "L2")) &&
          contains_any(user.traits["team"], set("Cloud"))`,

      errors: ['Role Match Condition is required'],
    },
  ];

  invalidPredicates.forEach(test => {
    it(`${test.name}`, () => {
      const got = getRuleCondition(test.str);
      expect(got.errors).toEqual(test.errors);
    });
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
            field: { label: '', value: 'level' },
            values: [
              { label: 'L1', value: 'L1' },
              { label: 'L2', value: 'L2' },
            ],
          },
          {
            field: { label: '', value: 'team' },
            values: [{ label: 'Cloud', value: 'Cloud' }],
          },
        ],
      },
      exp: `contains_all(set("access", "editor"), access_request.spec.roles) &&
contains_any(user.traits["level"], set("L1", "L2")) &&
contains_any(user.traits["team"], set("Cloud"))`,
    },
  ];

  ruleConditions.forEach(test => {
    it(`${test.name}`, () => {
      const got = convertRuleConditionToPredicateExpression(test.cond);
      expect(got).toBe(test.exp);
    });
  });
});
