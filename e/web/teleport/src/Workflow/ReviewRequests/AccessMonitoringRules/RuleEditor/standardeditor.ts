import { Option } from 'shared/components/Select';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleSubject,
} from 'e-teleport/services/accessmonitoringrule/types';
import { Plugin } from 'teleport/services/integrations';

import {
  convertRuleConditionToPredicateExpression,
  getRuleCondition,
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
 * Returns the rule object with required fields defined with empty values.
 */
export function newAccessMonitoringRule(): AccessMonitoringRule {
  return {
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
      ruleCondition: getRuleCondition(''),
    };
  }

  let recipients: Option[] = [];
  const definedRecipients = rule.spec.notification?.recipients;
  if (definedRecipients) {
    recipients = definedRecipients.map(r => ({ value: r, label: r }));
  }

  let pluginOption: Option = getFirstPluginOption();
  const definedIntegrationName = rule.spec.notification?.name;
  if (definedIntegrationName) {
    pluginOption = {
      value: definedIntegrationName,
      label: definedIntegrationName,
    };
  }

  return {
    ruleName: rule.metadata.name,
    pluginOption,
    recipients,
    ruleCondition: getRuleCondition(rule.spec.condition),
  };
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

  const originalRuleCondition = getRuleCondition(originalRule?.spec.condition);

  const modifiedRuleConditionField = () =>
    originalRuleCondition?.field.value !== updated.ruleCondition?.field.value;

  const modifiedRuleName = () =>
    updated.ruleName !== originalRule?.metadata.name;

  const modifiedRuleConditionValues = () =>
    originalRuleCondition?.values?.map(v => v.value).join('') !==
    updated.ruleCondition?.values?.map(v => v.value).join('');

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
    modifiedRuleConditionField() ||
    modifiedRuleName() ||
    modifiedRuleConditionValues() ||
    modifiedPluginOption() ||
    modifiedRecipients()
  );
}
