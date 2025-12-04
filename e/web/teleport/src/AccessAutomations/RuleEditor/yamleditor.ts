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
