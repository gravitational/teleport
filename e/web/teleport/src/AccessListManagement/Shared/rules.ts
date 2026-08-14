interface RuleResult {
  valid: boolean;
  message?: string;
}
export type RuleFunc<T> = (v: T) => () => RuleResult;

export const noopRule: RuleFunc<any> = () => () => ({ valid: true });
