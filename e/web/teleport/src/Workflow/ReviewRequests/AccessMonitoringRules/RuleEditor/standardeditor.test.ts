import {
  AccessMonitoringRule,
  AccessMonitoringRuleSubject,
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
    metadata: {
      name: '',
    },
    spec: {
      subjects: [],
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
      field: accessRequestMatchConditionOptions.find(
        a => a.value === AccessRequestMatchCondition.AnyRoles
      ),
      values: [],
    },
    isDirty: false,
  });

  expect(got).toStrictEqual({
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
  rule.spec.condition = 'invalid';

  const cfg = getConfigurableFieldsForStandardEditor(rule, []);
  expect(cfg).toStrictEqual({
    ruleName: 'some-name',
    pluginOption: { value: 'slack', label: 'slack' },
    recipients: [{ value: 'llama', label: 'llama' }],
    ruleCondition: null,
  });

  const got = buildRuleFromStandardEditor({
    rule,
    ...cfg,
    pluginOption: { value: 'mattermost', label: 'mattermost' },
    ruleCondition: null,
    isDirty: false,
  });

  expect(got).toStrictEqual({
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

test('hasModifiedFields: no modified fields', () => {
  const originalRule: AccessMonitoringRule = {
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
      name: 'modify condition',
      cfg: {
        ...cfg,
        ruleCondition: {
          field: accessRequestMatchConditionOptions.find(
            a => a.value === AccessRequestMatchCondition.AnyRoles
          ),
          values: [],
        },
      },
    },
  ];

  cases.forEach(test => {
    it(`${test.name}`, () => {
      const modified = hasModifiedFields(test.cfg, originalRule, false);
      expect(modified).toBe(true);
    });
  });
});
