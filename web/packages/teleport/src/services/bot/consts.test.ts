import { getBotType } from './consts';
import { BotUiFlow } from './types';

describe('getBotType', () => {
  test('no labels', () => {
    expect(getBotType(null)).toBeNull();
  });

  test('valid github-actions-ssh label', () => {
    const labels = new Map(
      Object.entries({ 'teleport.internal/ui-flow': 'github-actions-ssh' })
    );
    expect(getBotType(labels)).toEqual(BotUiFlow.GitHubActionsSsh);
  });

  test('unknown label value', () => {
    const labels = new Map(
      Object.entries({ 'teleport.internal/ui-flow': 'unknown' })
    );
    expect(getBotType(labels)).toBeNull();
  });

  test('unrelated label', () => {
    const labels = new Map(
      Object.entries({ 'unrelated-label': 'github-actions-ssh' })
    );
    expect(getBotType(labels)).toBeNull();
  });
});
