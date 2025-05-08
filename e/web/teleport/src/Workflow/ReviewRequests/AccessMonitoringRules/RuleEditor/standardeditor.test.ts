import {
  AccessMonitoringRule,
  AccessMonitoringRuleSubject,
  AccessMonitoringRuleVersion,
} from 'e-teleport/services/accessmonitoringrule/types';

import {
  AccessRequestMatchCondition,
  accessRequestMatchConditionOptions,
} from './rulecondition';
import {
  buildRuleFromStandardEditor,
  ConfigurableFieldsForStandardEditor,
  getConfigurableFieldsForStandardEditor,
  hasModifiedFields,
  newAccessMonitoringRule,
} from './standardeditor';

test('buildRuleFromStandardEditor: empty fields', () => {
  const emptyRule = newAccessMonitoringRule();
  expect(emptyRule).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: '',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '',
      notification: {
        name: '',
        recipients: [],
      },
    },
  });

  const got = buildRuleFromStandardEditor({
    rule: emptyRule,
    ...getConfigurableFieldsForStandardEditor(null, []),
    isDirty: false,
  });

  expect(got).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: '',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '',
      notification: {
        name: '',
        recipients: [],
      },
    },
  });
});

test('buildRuleFromStandardEditor: empty rule with configurable fields defined', () => {
  const emptyRule = newAccessMonitoringRule();

  const got = buildRuleFromStandardEditor({
    rule: emptyRule,
    ruleName: 'rule-name',
    pluginOption: { value: 'slack', label: 'slack' },
    recipients: [{ value: 'llama', label: 'llama' }],
    ruleCondition: {
      rolesCondition: {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.AnyRoles
        ),
        values: [],
      },
    },
    isDirty: false,
  });

  expect(got).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: 'rule-name',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '!is_empty(access_request.spec.roles)',
      notification: {
        name: 'slack',
        recipients: ['llama'],
      },
    },
  });
});

test('buildRuleFromStandardEditor: partial configurable fields defined', () => {
  const rule = newAccessMonitoringRule();
  rule.metadata.name = 'some-name';
  rule.spec.notification.name = 'slack';
  rule.spec.notification.recipients = ['llama'];

  const cfg = getConfigurableFieldsForStandardEditor(rule, []);
  expect(cfg).toStrictEqual({
    ruleName: 'some-name',
    pluginOption: { value: 'slack', label: 'slack' },
    recipients: [{ value: 'llama', label: 'llama' }],
    ruleCondition: {
      rolesCondition: {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAllRoles
        ),
        values: [],
      },
      traitsCondition: null,
    },
    errors: [],
  });

  const got = buildRuleFromStandardEditor({
    rule,
    ...cfg,
    pluginOption: { value: 'mattermost', label: 'mattermost' },
    ruleCondition: null,
    isDirty: false,
  });

  expect(got).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: 'some-name',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '',
      notification: {
        name: 'mattermost',
        recipients: ['llama'],
      },
    },
  });
});

describe('getConfigurableFieldsForStandardEditor unsupported fields', () => {
  const rule = newAccessMonitoringRule();
  const cases: {
    name: string;
    rule: AccessMonitoringRule;
    errors: string[];
  }[] = [
    {
      name: 'desired_state is not supported',
      rule: {
        ...rule,
        spec: {
          ...rule.spec,
          desired_state: 'reviewed',
        },
      },
      errors: ['Unsupported field: desired_state'],
    },
    {
      name: 'automatic_review is not supported',
      rule: {
        ...rule,
        spec: {
          ...rule.spec,
          automatic_review: {
            integration: 'builtin',
            decision: 'APPROVED',
          },
        },
      },
      errors: ['Unsupported field: automatic_review'],
    },
    {
      name: 'invalid condition',
      rule: {
        ...rule,
        spec: {
          ...rule.spec,
          condition: 'invalid',
        },
      },
      errors: ['Unsupported condition: invalid'],
    },
  ];

  test.each(cases)('$name', ({ rule, errors }) => {
    const cfg = getConfigurableFieldsForStandardEditor(rule, []);
    expect(cfg.errors).toStrictEqual(errors);
  });
});

test('hasModifiedFields: no modified fields', () => {
  const originalRule: AccessMonitoringRule = {
    kind: 'access_monitoring_rule',
    version: AccessMonitoringRuleVersion.V1,
    metadata: {
      name: 'rule-name',
    },
    spec: {
      subjects: [],
      condition: 'condition',
      notification: {
        name: 'slack',
        recipients: ['apple'],
      },
    },
  };

  const modified = hasModifiedFields(
    getConfigurableFieldsForStandardEditor(originalRule, []),
    originalRule,
    false /* yamlModified */
  );

  expect(modified).toBe(false);
});

test('hasModifiedFields: if yaml is modified, always return true', () => {
  const originalRule: AccessMonitoringRule = {
    kind: 'access_monitoring_rule',
    version: AccessMonitoringRuleVersion.V1,
    metadata: {
      name: 'rule-name',
    },
    spec: {
      subjects: [],
      condition: 'condition',
      notification: {
        name: 'slack',
        recipients: ['apple'],
      },
    },
  };

  const modified = hasModifiedFields(
    getConfigurableFieldsForStandardEditor(originalRule, []),
    originalRule,
    true /* yamlModified */
  );

  expect(modified).toBe(true);
});

describe('hasModifiedFields', () => {
  const originalRule: AccessMonitoringRule = {
    kind: 'access_monitoring_rule',
    version: AccessMonitoringRuleVersion.V1,
    metadata: {
      name: 'rule-name',
    },
    spec: {
      subjects: [],
      condition: 'condition',
      notification: {
        name: 'slack',
        recipients: ['apple'],
      },
    },
  };

  const cfg = getConfigurableFieldsForStandardEditor(originalRule, []);

  const cases: { name: string; cfg: ConfigurableFieldsForStandardEditor }[] = [
    {
      name: 'modify name',
      cfg: { ...cfg, ruleName: 'modified-name' },
    },
    {
      name: 'modify notification name',
      cfg: {
        ...cfg,
        pluginOption: { value: 'mattermost', label: 'mattermost' },
      },
    },
    {
      name: 'modify recipients',
      cfg: { ...cfg, recipients: [{ value: 'llama', label: 'llama' }] },
    },
    {
      name: 'modify role condition',
      cfg: {
        ...cfg,
        ruleCondition: {
          rolesCondition: {
            field: accessRequestMatchConditionOptions.find(
              a => a.value === AccessRequestMatchCondition.AnyRoles
            ),
            values: [],
          },
        },
      },
    },
    {
      name: 'modify traits condition',
      cfg: {
        ...cfg,
        ruleCondition: {
          traitsCondition: [
            {
              field: { label: 'trait-key', value: 'trait-key' },
              values: [{ label: 'trait-val', value: 'trait-val' }],
            },
          ],
        },
      },
    },
  ];

  test.each(cases)('$name', ({ cfg }) => {
    const modified = hasModifiedFields(cfg, originalRule, false);
    expect(modified).toBe(true);
  });
});
