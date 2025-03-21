import newruletemplate from './accessmonitoringrule.yaml?raw';
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
 * returns yaml string with placeholders replaced
 * with configurable fields.
 * Used only with new rules as the template adds
 * helpful comments to the user.
 */
export function newYamlRuleFromTemplate(
  configurableFields: ConfigurableFieldsForStandardEditor
) {
  let template: string = newruletemplate;
  const { ruleName, ruleCondition, pluginOption, recipients } =
    configurableFields;

  template = template.replace(
    '{{rule.metadata.name}}',
    ruleName || 'new_rule_name'
  );
  template = template.replace(
    '{{rule.condition}}',
    // The single quote is required to represent condition
    // as a string. Normally the yaml library we use will insert
    // single quotes if required, but for new yamls we don't require
    // the yaml lib since we use a pre-defined template.
    `'${convertRuleConditionToPredicateExpression(ruleCondition)}'`
  );
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
