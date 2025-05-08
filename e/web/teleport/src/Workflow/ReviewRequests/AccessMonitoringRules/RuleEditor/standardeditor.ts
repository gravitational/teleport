import { Option } from 'shared/components/Select';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleSubject,
  AccessMonitoringRuleVersion,
} from 'e-teleport/services/accessmonitoringrule/types';
import { Plugin } from 'teleport/services/integrations';

import {
  convertRuleConditionToPredicateExpression,
  getNotificationRuleCondition,
  RuleCondition,
} from './rulecondition';

/**
 * defines select few fields from the AccessMonitoringRule resource
 * that can be editable by the user in the standard editor.
 */
export type ConfigurableFieldsForStandardEditor = {
  ruleName: string;
  pluginOption: Option;
  recipients: Option[];
  ruleCondition: RuleCondition;
  /**
   * errors will be populated with any unsupported fields
   * that are not editable within the standard editor.
   */
  errors?: string[];
};

export type StandardEditor = ConfigurableFieldsForStandardEditor & {
  /**
   * refers to the last parsed from yaml (when user toggles
   * between from yaml back to standard editor).
   */
  rule: AccessMonitoringRule;
  /**
   * will be true if fields have been modified from the original.
   */
  isDirty: boolean;
};

/**
 * Returns the rule object with required fields defined with default values.
 */
export function newAccessMonitoringRule(): AccessMonitoringRule {
  return {
    kind: 'access_monitoring_rule',
    version: AccessMonitoringRuleVersion.V1,
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
  };
}

// unsupportedFieldErrors returns an array of error messages for the unsupported
// fields.
function unsupportedFieldErrors(obj: Record<string, any>): string[] {
  return Object.keys(obj).map(key => `Unsupported field: ${key}`);
}

// ruleToNotificationsEditor parses the notifications AccessMonitoringRule
// object and extracts the fields that are editable in the standard editor.
// Unsupported fields are returned in the errors array.
function ruleToNotificationsEditor(
  rule: AccessMonitoringRule
): ConfigurableFieldsForStandardEditor {
  const errors: string[] = [];

  // We use destructuring to strip fields from objects and assert that nothing
  // has been left. Therefore, we don't want Lint to warn us that we didn't use
  // some of the fields.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { kind, version, metadata, spec, ...unsupported } = rule;
  if (unsupported) {
    errors.push(...unsupportedFieldErrors(unsupported));
  }

  const {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    subjects,
    condition,
    notification,
    ...unsupportedSpec
  } = spec;
  if (unsupportedSpec) {
    errors.push(...unsupportedFieldErrors(unsupportedSpec));
  }

  let recipients: Option[] = [];
  if (notification?.recipients) {
    recipients = notification.recipients.map(r => ({ value: r, label: r }));
  }

  let pluginOption: Option = null;
  if (notification?.name) {
    pluginOption = { value: notification.name, label: notification.name };
  }

  const ruleCondition = getNotificationRuleCondition(condition);
  if (!ruleCondition) {
    errors.push(`Unsupported condition: ${condition}`);
  }

  return {
    ruleName: metadata?.name || '',
    pluginOption,
    recipients,
    ruleCondition,
    errors,
  };
}

/**
 * Returns configurable fields with default values
 * or extracts them from an existing rule.
 */
export function getConfigurableFieldsForStandardEditor(
  rule: AccessMonitoringRule | null,
  plugins: Plugin[]
): ConfigurableFieldsForStandardEditor {
  const getFirstPluginOption = () => {
    if (plugins.length === 1) {
      return {
        value: plugins[0].name,
        label: plugins[0].name,
      };
    }
  };

  if (!rule) {
    // creating a new rule
    return {
      ruleName: '',
      recipients: [],
      pluginOption: getFirstPluginOption(),
      ruleCondition: getNotificationRuleCondition(''),
    };
  }

  const fields = ruleToNotificationsEditor(rule);
  fields.pluginOption = fields.pluginOption || getFirstPluginOption();

  return fields;
}

/**
 * Returns the rule object held by editor with its
 * fields updated with the configurable fields that
 * may have been changed by the user.
 */
export function buildRuleFromStandardEditor(
  standardEditor: StandardEditor
): AccessMonitoringRule {
  const { rule, ...configurableFields } = standardEditor;
  const { ruleName, recipients, ruleCondition, pluginOption } =
    configurableFields;

  const getSubjects = () => {
    const subjects = rule.spec.subjects;
    if (subjects?.length > 0) {
      return subjects;
    }
    // Default.
    return [AccessMonitoringRuleSubject.AccessRequest];
  };

  return {
    ...rule,
    metadata: {
      ...rule.metadata,
      name: ruleName,
    },
    spec: {
      ...rule.spec,
      condition: convertRuleConditionToPredicateExpression(ruleCondition),
      subjects: getSubjects(),
      notification: {
        ...rule.spec.notification,
        recipients: recipients?.map(r => r.value),
        name: pluginOption?.value || '',
      },
    },
  };
}

/**
 * Detects if fields were modified by comparing against the original rule.
 *
 * If "yamlModified", it will always return true to handle an edge case
 * where the user can add more fields from the yaml editor that the
 * standard editor does not read.
 */
export function hasModifiedFields(
  updated: ConfigurableFieldsForStandardEditor,
  originalRule: AccessMonitoringRule,
  yamlModified: boolean
) {
  if (yamlModified) {
    return true;
  }

  const originalRuleCondition = getNotificationRuleCondition(
    originalRule?.spec.condition
  );

  const modifiedRolesConditionField = () =>
    originalRuleCondition?.rolesCondition?.field.value !==
    updated.ruleCondition?.rolesCondition?.field.value;

  const modifiedTraitsConditionField = () => {
    if (
      originalRuleCondition?.traitsCondition?.length !==
      updated.ruleCondition?.traitsCondition?.length
    ) {
      return true;
    }

    return (
      originalRuleCondition?.traitsCondition
        ?.map(v => v.values.join(''))
        .join('') !==
      updated.ruleCondition?.traitsCondition
        ?.map(v => v.values.join(''))
        .join('')
    );
  };

  const modifiedRuleName = () =>
    updated.ruleName !== originalRule?.metadata.name;

  const modifiedRolesConditionValues = () =>
    originalRuleCondition?.rolesCondition?.values
      ?.map(v => v.value)
      .join('') !==
    updated.ruleCondition?.rolesCondition?.values?.map(v => v.value).join('');

  const modifiedPluginOption = () =>
    updated.pluginOption?.value !== originalRule?.spec.notification.name;

  const modifiedRecipients = () => {
    const originalRecipients = originalRule?.spec.notification.recipients || [];
    return (
      updated.recipients?.map(r => r.value).join('') !==
      originalRecipients.join('')
    );
  };

  return (
    modifiedRolesConditionField() ||
    modifiedRuleName() ||
    modifiedRolesConditionValues() ||
    modifiedPluginOption() ||
    modifiedRecipients() ||
    modifiedTraitsConditionField()
  );
}
