import { newSchedule } from 'shared/components/ScheduleEditor';

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
  getConfigurableNotificationFieldsForStandardEditor,
  getConfigurableReviewFieldsForStandardEditor,
  hasModifiedFields,
  newAccessMonitoringRule,
} from './standardeditor';

test('buildRuleFromStandardEditor: empty notification fields', () => {
  const emptyRule = newAccessMonitoringRule();
  expect(emptyRule).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: 'new_rule_name',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '',
    },
  });

  const got = buildRuleFromStandardEditor({
    rule: emptyRule,
    ...getConfigurableNotificationFieldsForStandardEditor(null, []),
    isDirty: false,
  });

  expect(got).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: 'new_rule_name',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '',
      desired_state: null,
      notification: null,
      automatic_review: null,
      schedules: {},
    },
  });
});

test('buildRuleFromStandardEditor: empty review fields', () => {
  const emptyRule = newAccessMonitoringRule();
  expect(emptyRule).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: 'new_rule_name',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '',
    },
  });

  const got = buildRuleFromStandardEditor({
    rule: emptyRule,
    ...getConfigurableReviewFieldsForStandardEditor(null),
    isDirty: false,
  });

  expect(got).toStrictEqual({
    kind: 'access_monitoring_rule',
    version: 'v1',
    metadata: {
      name: 'new_rule_name',
    },
    spec: {
      subjects: [AccessMonitoringRuleSubject.AccessRequest],
      condition: '',
      desired_state: 'reviewed',
      notification: null,
      automatic_review: {
        decision: 'APPROVED',
        integration: 'builtin',
      },
      schedules: {},
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
    automaticReview: { value: 'pagerduty', label: 'pagerduty' },
    reviewDecisionOption: { label: 'Approved', value: 'APPROVED' },
    desiredState: 'reviewed',
    schedule: null,
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
      desired_state: 'reviewed',
      notification: {
        name: 'slack',
        recipients: ['llama'],
      },
      automatic_review: {
        integration: 'pagerduty',
        decision: 'APPROVED',
      },
      schedules: {},
    },
  });
});

test('buildRuleFromStandardEditor: partial configurable notification fields defined', () => {
  const rule = newAccessMonitoringRule();
  rule.metadata.name = 'some-name';
  rule.spec.notification = {
    name: 'slack',
    recipients: ['llama'],
  };

  const cfg = getConfigurableNotificationFieldsForStandardEditor(rule, []);
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
    automaticReview: null,
    reviewDecisionOption: null,
    desiredState: null,
    schedule: null,
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
      automatic_review: null,
      desired_state: null,
      schedules: {},
    },
  });
});

test('buildRuleFromStandardEditor: partial configurable review fields defined', () => {
  const rule = newAccessMonitoringRule();
  rule.metadata.name = 'some-name';
  rule.spec.automatic_review = {
    integration: 'builtin',
    decision: 'APPROVED',
  };
  rule.spec.desired_state = 'reviewed';

  const cfg = getConfigurableReviewFieldsForStandardEditor(rule);
  expect(cfg).toStrictEqual({
    ruleName: 'some-name',
    pluginOption: null,
    recipients: [],
    ruleCondition: {
      rolesCondition: {
        field: accessRequestMatchConditionOptions.find(
          a => a.value === AccessRequestMatchCondition.MatchAllRoles
        ),
        values: [],
      },
      traitsCondition: null,
      resourcesCondition: null,
    },
    errors: [],
    automaticReview: { value: 'builtin', label: 'builtin' },
    reviewDecisionOption: { label: 'Approved', value: 'APPROVED' },
    desiredState: 'reviewed',
    schedule: null,
  });

  const schedule = newSchedule();
  schedule.shifts.Monday = {
    startTime: { value: '00:00', label: '00:00' },
    endTime: { value: '23:59', label: '23:59' },
  };

  const got = buildRuleFromStandardEditor({
    rule,
    ...cfg,
    automaticReview: { value: 'pagerduty', label: 'pagerduty' },
    ruleCondition: null,
    isDirty: false,
    schedule: schedule,
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
      notification: null,
      automatic_review: {
        integration: 'pagerduty',
        decision: 'APPROVED',
      },
      desired_state: 'reviewed',
      schedules: {
        default: {
          time: {
            timezone: 'UTC',
            shifts: [
              {
                start: '00:00',
                end: '23:59',
                weekday: 'Monday',
              },
            ],
          },
        },
      },
    },
  });
});

describe('getConfigurableNotificationFieldsForStandardEditor unsupported fields', () => {
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
    const cfg = getConfigurableNotificationFieldsForStandardEditor(rule, []);
    expect(cfg.errors).toStrictEqual(errors);
  });
});

describe('getConfigurableReviewFieldsForStandardEditor unsupported fields', () => {
  const rule = newAccessMonitoringRule();
  const cases: {
    name: string;
    rule: AccessMonitoringRule;
    errors: string[];
  }[] = [
    {
      name: 'unsupported desired_state',
      rule: {
        ...rule,
        spec: {
          ...rule.spec,
          desired_state: 'unsupported_state',
          automatic_review: {
            integration: 'builtin',
            decision: 'APPROVED',
          },
        },
      },
      errors: ['Unsupported desired_state: unsupported_state'],
    },
    {
      name: 'unsupported decision',
      rule: {
        ...rule,
        spec: {
          ...rule.spec,
          desired_state: 'reviewed',
          automatic_review: {
            integration: 'builtin',
            decision: 'unsupported_decision',
          },
        },
      },
      errors: ['Unsupported automatic_review.decision: unsupported_decision'],
    },
    {
      name: 'invalid condition',
      rule: {
        ...rule,
        spec: {
          ...rule.spec,
          condition: 'invalid',
          desired_state: 'reviewed',
          automatic_review: {
            integration: 'builtin',
            decision: 'DENIED',
          },
        },
      },
      errors: ['Unsupported condition: invalid'],
    },
    {
      name: 'multiple schedules unsupported',
      rule: {
        ...rule,
        spec: {
          ...rule.spec,
          desired_state: 'reviewed',
          schedules: {
            schedule1: {
              time: {
                timezone: 'UTC',
                shifts: [],
              },
            },
            schedule2: {
              time: {
                timezone: 'UTC',
                shifts: [],
              },
            },
          },
        },
      },
      errors: ['The standard editor only supports 1 schedule'],
    },
  ];

  test.each(cases)('$name', ({ rule, errors }) => {
    const cfg = getConfigurableReviewFieldsForStandardEditor(rule);
    expect(cfg.errors).toStrictEqual(errors);
  });
});

test('hasModifiedFields: no modified fields', () => {
  const originalRule: AccessMonitoringRule = {
    kind: 'access_monitoring_rule',
    version: AccessMonitoringRuleVersion.V1,
    metadata: {
      name: 'rule-name_no_modified',
    },
    spec: {
      subjects: [],
      condition: 'contains_any(access_request.spec.roles, set("test"))',
      notification: {
        name: 'slack',
        recipients: ['apple'],
      },
      desired_state: null,
    },
  };

  const modified = hasModifiedFields(
    getConfigurableNotificationFieldsForStandardEditor(originalRule, []),
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
    getConfigurableNotificationFieldsForStandardEditor(originalRule, []),
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

  const cfg = getConfigurableNotificationFieldsForStandardEditor(
    originalRule,
    []
  );

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
              traitKey: { label: 'trait-key', value: 'trait-key' },
              traitValues: [{ label: 'trait-val', value: 'trait-val' }],
            },
          ],
        },
      },
    },
    {
      name: 'modify resources condition',
      cfg: {
        ...cfg,
        ruleCondition: {
          resourcesCondition: [{ name: 'env', value: 'dev' }],
        },
      },
    },
    {
      name: 'modify schedule',
      cfg: {
        ...cfg,
        schedule: {
          name: 'default',
          timezone: { value: 'UTC', label: 'UTC' },
          shifts: {
            ...newSchedule().shifts,
            Monday: {
              startTime: { value: '00:00', label: '00:00' },
              endTime: { value: '23:59', label: '23:59' },
            },
          },
        },
      },
    },
  ];

  test.each(cases)('$name', ({ cfg }) => {
    const modified = hasModifiedFields(cfg, originalRule, false);
    expect(modified).toBe(true);
  });
});
