import notificationTemplate from './notification_rule.yaml?raw';
import reviewTemplate from './review_rule.yaml?raw';
import { convertRuleConditionToPredicateExpression } from './rulecondition';
import { ConfigurableFieldsForStandardEditor } from './standardeditor';

export type YamlEditor = {
  content: string;
  /**
   * will be true if initial yaml
   * content were modified
   */
  isDirty: boolean;
  /**
   * requiresReset just means the current "content"
   * cannot be parsed by the standard editor.
   *
   * eg: the resource field `condition` contains
   * a more complex predicate expression than the
   * standard editor can parse.
   */
  requiresReset?: boolean;
};

/**
 * returns yaml string for notification rules. The placeholders
 * are replaced with configurable fields.
 * Used only with new rules as the template adds
 * helpful comments to the user.
 */
export function newNotificationRuleYaml(
  configurableFields: ConfigurableFieldsForStandardEditor
) {
  let template: string = notificationTemplate;
  const { ruleName, ruleCondition, pluginOption, recipients } =
    configurableFields;

  let condition = '';
  const expression = convertRuleConditionToPredicateExpression(ruleCondition);
  const tokens = expression.split('\n');
  if (tokens.length === 1) {
    // Return single line expressions as is.
    condition = expression;
  } else if (tokens.length > 0) {
    // Return multiline expressions prefixed with '|-' and indented.
    const indented = tokens.map(line => '    ' + line).join('\n');
    condition = `|-\n${indented}`;
  }

  template = template.replace(
    '{{rule.metadata.name}}',
    ruleName || 'new_rule_name'
  );
  template = template.replace('{{rule.condition}}', condition);
  template = template.replace(
    '{{rule.notification.name}}',
    pluginOption?.value || ''
  );
  template = template.replace(
    '{{rule.notification.recipients}}',
    `[${recipients.map(r => r.value)}]`
  );

  return template;
}

/**
 * returns yaml string for review rules. The placeholders
 * are replaced with configurable fields.
 * Used only with new rules as the template adds
 * helpful comments to the user.
 */
export function newReviewRuleYaml(
  configurableFields: ConfigurableFieldsForStandardEditor
) {
  let template: string = reviewTemplate;
  const {
    ruleName,
    ruleCondition,
    pluginOption,
    recipients,
    automaticReview,
    reviewDecisionOption,
    desiredState,
  } = configurableFields;

  let condition = '';
  const expression = convertRuleConditionToPredicateExpression(ruleCondition);
  const tokens = expression.split('\n');
  if (tokens.length === 1) {
    // Return single line expressions as is.
    condition = expression;
  } else if (tokens.length > 0) {
    // Return multiline expressions prefixed with '|-' and indented.
    const indented = tokens.map(line => '    ' + line).join('\n');
    condition = `|-\n${indented}`;
  }

  template = template.replace(
    '{{rule.metadata.name}}',
    ruleName || 'new_rule_name'
  );
  template = template.replace('{{rule.condition}}', condition);
  template = template.replace('{{rule.desired_state}}', desiredState || '');
  template = template.replace(
    '{{rule.notification.name}}',
    pluginOption?.value || ''
  );
  template = template.replace(
    '{{rule.notification.recipients}}',
    `[${recipients.map(r => r.value)}]`
  );
  template = template.replace(
    '{{rule.automatic_review.integration}}',
    automaticReview?.value || ''
  );
  template = template.replace(
    '{{rule.automatic_review.decision}}',
    reviewDecisionOption?.value || ''
  );

  return template;
}
