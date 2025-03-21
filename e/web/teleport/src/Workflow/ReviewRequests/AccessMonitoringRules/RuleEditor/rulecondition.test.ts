import {
  AccessRequestMatchCondition,
  accessRequestMatchConditionOptions,
  convertRuleConditionToPredicateExpression,
  getRuleCondition,
  RuleCondition,
} from './rulecondition';

test('getRuleCondition: valid is_empty predicate', () => {
  const predicate = '!is_empty(access_request.spec.roles)';
  const got = getRuleCondition(predicate);

  expect(got.field).toEqual(
    accessRequestMatchConditionOptions.find(
      a => a.value === AccessRequestMatchCondition.AnyRoles
    )
  );
  expect(got.values).toBeUndefined();
});

test('getRuleCondition: valid contains_any predicate', () => {
  const predicate =
    'contains_any(access_request.spec.roles, set("access","editor"))';
  const got = getRuleCondition(predicate);

  expect(got.field).toEqual(
    accessRequestMatchConditionOptions.find(
      a => a.value === AccessRequestMatchCondition.Roles
    )
  );
  expect(got.values).toEqual([
    { label: 'access', value: 'access' },
    { label: 'editor', value: 'editor' },
  ]);
});

test('getRuleCondition: valid empty predicate, returns default', () => {
  const got = getRuleCondition('');

  expect(got.field).toEqual(
    accessRequestMatchConditionOptions.find(
      a => a.value === AccessRequestMatchCondition.Roles
    )
  );
  expect(got.values).toEqual([]);
});

describe('getRuleCondition invalid inputs', () => {
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
  ];

  invalidPredicates.forEach(test => {
    it(`${test.name}`, () => {
      const got = getRuleCondition(test.str);
      expect(got).toBeNull();
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
      cond: { field: null, values: [] },
      exp: '',
    },
    {
      name: 'match condition roles, without any values',
      cond: {
        field: { label: '', value: AccessRequestMatchCondition.Roles },
        values: [],
      },
      exp: '',
    },
    {
      name: 'match condition roles',
      cond: {
        field: { label: '', value: AccessRequestMatchCondition.Roles },
        values: [
          { value: 'access', label: 'access' },
          { value: 'editor', label: 'editor' },
        ],
      },
      exp: 'contains_any(access_request.spec.roles, set("access", "editor"))',
    },
    {
      name: 'match condition ANY roles',
      cond: {
        field: { label: '', value: AccessRequestMatchCondition.AnyRoles },
        values: [],
      },
      exp: '!is_empty(access_request.spec.roles)',
    },
  ];

  ruleConditions.forEach(test => {
    it(`${test.name}`, () => {
      const got = convertRuleConditionToPredicateExpression(test.cond);
      expect(got).toBe(test.exp);
    });
  });
});
