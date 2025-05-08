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
