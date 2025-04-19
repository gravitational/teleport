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

  // Prefix each line with indentation.
  const expression = convertRuleConditionToPredicateExpression(ruleCondition)
    .split('\n')
    .map(line => '    ' + line)
    .join('\n');

  const condition = expression !== '' ? `|-\n${expression}` : `''`;

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
