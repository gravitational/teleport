import { Option } from 'shared/components/Select';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleSubject,
  AccessMonitoringRuleVersion,
} from 'e-teleport/services/accessmonitoringrule/types';
import { Plugin } from 'teleport/services/integrations';
import {
  AccessMonitoringRuleState,
  AccessReviewDecision,
} from 'teleport/services/resources';

import {
  convertRuleConditionToPredicateExpression,
  getNotificationRuleCondition,
  getReviewRuleCondition,
  RuleCondition,
} from './rulecondition';

export type ReviewDecisionOption = {
  value: AccessReviewDecision;
  label: string;
};

export const reviewDecisionOptions: ReviewDecisionOption[] = [
  {
    value: 'APPROVED',
    label: 'Approved',
  },
  {
    value: 'DENIED',
    label: 'Denied',
  },
];

/**
 * defines select few fields from the AccessMonitoringRule resource
 * that can be editable by the user in the standard editor.
 */
export type ConfigurableFieldsForStandardEditor = {
  ruleName: string;
  pluginOption: Option;
  recipients: Option[];
  ruleCondition: RuleCondition;
  automaticReview: Option;
  reviewDecisionOption: ReviewDecisionOption;
  desiredState: AccessMonitoringRuleState;
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
    },
  };
}

// unsupportedFieldErrors returns an array of error messages for the unsupported
// fields.
function unsupportedFieldErrors(obj: Record<string, any>): string[] {
  return Object.keys(obj).map(key => `Unsupported field: ${key}`);
}

function ruleToReviewsEditor(
  rule: AccessMonitoringRule
): ConfigurableFieldsForStandardEditor {
  const getAutomaticReviewIntegration = (integration: string): Option => {
    switch (integration) {
      case 'builtin':
        return { label: integration, value: integration };
      default:
        return { label: '', value: '' };
    }
  };

  const getReviewDecision = (decision: string): ReviewDecisionOption => {
    const proposedState = decision?.toUpperCase();
    switch (proposedState) {
      case 'APPROVED':
        return { label: 'Approved', value: proposedState };
      case 'DENIED':
        return { label: 'Denied', value: proposedState };
      default:
        return { label: '', value: '' };
    }
  };

  const getDesiredState = (desiredState: string): AccessMonitoringRuleState => {
    switch (desiredState) {
      case 'reviewed':
        return desiredState;
      default:
        return '';
    }
  };

  const configurableFields = {
    ruleName: '',
    pluginOption: null,
    recipients: [],
    ruleCondition: {},
    automaticReview: null,
    reviewDecisionOption: null,
    desiredState: getDesiredState(''),
    errors: [],
  };

  if (!rule) {
    configurableFields.errors.push(`rule is required`);
    return configurableFields;
  }

  // We use destructuring to strip fields from objects and assert that nothing
  // has been left. Therefore, we don't want Lint to warn us that we didn't use
  // some of the fields.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars, unused-imports/no-unused-vars
  const { kind, version, metadata, spec, ...unsupported } = rule;
  if (unsupported) {
    configurableFields.errors.push(...unsupportedFieldErrors(unsupported));
  }
  configurableFields.ruleName = metadata?.name || '';

  if (!spec) {
    configurableFields.errors.push(`spec is required`);
    return configurableFields;
  }

  const {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, unused-imports/no-unused-vars
    subjects,
    condition,
    desired_state,
    notification,
    automatic_review,
    ...unsupportedSpec
  } = spec;
  if (unsupportedSpec) {
    configurableFields.errors.push(...unsupportedFieldErrors(unsupportedSpec));
  }

  if (notification) {
    const { name, recipients, ...unsupportedNotification } = notification;
    if (unsupportedNotification) {
      configurableFields.errors.push(
        ...unsupportedFieldErrors(unsupportedNotification)
      );
    }
    if (name) {
      configurableFields.pluginOption = { value: name, label: name };
    }
    if (recipients) {
      configurableFields.recipients = recipients.map(r => ({
        value: r,
        label: r,
      }));
    }
  }

  if (automatic_review) {
    const { integration, decision, ...unsupportedAutomaticReview } =
      automatic_review;
    if (unsupportedAutomaticReview) {
      configurableFields.errors.push(
        ...unsupportedFieldErrors(unsupportedAutomaticReview)
      );
    }
    configurableFields.automaticReview =
      getAutomaticReviewIntegration(integration);
    if (!configurableFields.automaticReview.value) {
      configurableFields.errors.push(
        `Unsupported automatic_review.integration: ${configurableFields.automaticReview}`
      );
    }
    configurableFields.reviewDecisionOption = getReviewDecision(decision);
    if (!configurableFields.reviewDecisionOption.value) {
      configurableFields.errors.push(
        `Unsupported automatic_review.decision: ${automatic_review?.decision}`
      );
    }
  }

  configurableFields.ruleCondition = getReviewRuleCondition(condition);
  if (!configurableFields.ruleCondition) {
    configurableFields.errors.push(`Unsupported condition: ${condition}`);
  }

  configurableFields.desiredState = getDesiredState(desired_state);
  if (!configurableFields.desiredState) {
    configurableFields.errors.push(
      `Unsupported desired_state: ${desired_state}`
    );
  }
  return configurableFields;
}

// ruleToNotificationsEditor parses the notifications AccessMonitoringRule
// object and extracts the fields that are editable in the standard editor.
// Unsupported fields are returned in the errors array.
function ruleToNotificationsEditor(
  rule: AccessMonitoringRule
): ConfigurableFieldsForStandardEditor {
  const configurableFields = {
    ruleName: '',
    pluginOption: null,
    recipients: [],
    ruleCondition: {},
    automaticReview: null,
    reviewDecisionOption: null,
    desiredState: null,
    errors: [],
  };

  if (!rule) {
    configurableFields.errors.push(`rule is required`);
    return configurableFields;
  }

  // We use destructuring to strip fields from objects and assert that nothing
  // has been left. Therefore, we don't want Lint to warn us that we didn't use
  // some of the fields.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars, unused-imports/no-unused-vars
  const { kind, version, metadata, spec, ...unsupported } = rule;
  if (unsupported) {
    configurableFields.errors.push(...unsupportedFieldErrors(unsupported));
  }
  configurableFields.ruleName = metadata?.name || '';

  if (!spec) {
    configurableFields.errors.push(`spec is required`);
    return configurableFields;
  }

  const {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, unused-imports/no-unused-vars
    subjects,
    condition,
    notification,
    ...unsupportedSpec
  } = spec;
  if (unsupportedSpec) {
    configurableFields.errors.push(...unsupportedFieldErrors(unsupportedSpec));
  }

  if (notification) {
    const { name, recipients, ...unsupportedNotification } = notification;
    if (unsupportedNotification) {
      configurableFields.errors.push(
        ...unsupportedFieldErrors(unsupportedNotification)
      );
    }
    if (name) {
      configurableFields.pluginOption = { value: name, label: name };
    }
    if (recipients) {
      configurableFields.recipients = recipients.map(r => ({
        value: r,
        label: r,
      }));
    }
  }

  configurableFields.ruleCondition = getNotificationRuleCondition(condition);
  if (!configurableFields.ruleCondition) {
    configurableFields.errors.push(`Unsupported condition: ${condition}`);
  }
  return configurableFields;
}

/**
 * Returns configurable fields for a review rule with default values
 * or extracts them from an existing rule.
 */
export function getConfigurableReviewFieldsForStandardEditor(
  rule: AccessMonitoringRule | null
): ConfigurableFieldsForStandardEditor {
  if (!rule) {
    // creating a new rule
    return {
      ruleName: '',
      recipients: [],
      pluginOption: null,
      ruleCondition: getReviewRuleCondition(''),
      automaticReview: { label: 'builtin', value: 'builtin' },
      reviewDecisionOption: reviewDecisionOptions.find(
        o => o.value === 'APPROVED'
      ),
      desiredState: 'reviewed',
    };
  }

  return ruleToReviewsEditor(rule);
}

/**
 * Returns configurable fields for a notifications rule with default values
 * or extracts them from an existing rule.
 */
export function getConfigurableNotificationFieldsForStandardEditor(
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
      automaticReview: null,
      reviewDecisionOption: null,
      desiredState: null,
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
  const {
    ruleName,
    recipients,
    ruleCondition,
    pluginOption,
    automaticReview,
    reviewDecisionOption,
    desiredState,
  } = configurableFields;

  const getSubjects = () => {
    const subjects = rule.spec.subjects;
    if (subjects?.length > 0) {
      return subjects;
    }
    // Default.
    return [AccessMonitoringRuleSubject.AccessRequest];
  };

  const notificationRoutingSpec = {
    ...rule.spec.notification,
    name: pluginOption?.value || '',
    recipients: recipients?.map(r => r.value),
  };

  const automaticReviewSpec = {
    ...rule.spec.automatic_review,
    integration: automaticReview?.value || '',
    decision: reviewDecisionOption?.value || '',
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
      desired_state: desiredState,
      notification:
        notificationRoutingSpec.name !== '' ? notificationRoutingSpec : null,
      automatic_review:
        automaticReviewSpec.integration !== '' ? automaticReviewSpec : null,
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
        ?.map(v => v.traitValues.join(''))
        .join('') !==
      updated.ruleCondition?.traitsCondition
        ?.map(v => v.traitValues.join(''))
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
    updated.pluginOption?.value !== originalRule?.spec.notification?.name;

  const modifiedRecipients = () => {
    const originalRecipients =
      originalRule?.spec.notification?.recipients || [];
    return (
      updated.recipients?.map(r => r.value).join('') !==
      originalRecipients.join('')
    );
  };

  const modifiedAutomaticReview = () =>
    updated.automaticReview?.value !==
    originalRule?.spec.automatic_review?.integration;

  const modifiedReviewDecision = () =>
    updated.reviewDecisionOption?.value !==
    originalRule?.spec.automatic_review?.decision;

  const modifiedDesiredState = () =>
    updated.desiredState !== originalRule?.spec.desired_state;

  return (
    modifiedRolesConditionField() ||
    modifiedRuleName() ||
    modifiedRolesConditionValues() ||
    modifiedPluginOption() ||
    modifiedRecipients() ||
    modifiedTraitsConditionField() ||
    modifiedAutomaticReview() ||
    modifiedReviewDecision() ||
    modifiedDesiredState()
  );
}
