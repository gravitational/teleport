import { inferencePolicySchema } from './policy';

test('validates with valid kinds and model', () => {
  const input = {
    providedByTeleportCloud: false,
    kinds: ['ssh', 'k8s', 'db'],
    model: 'gpt-4',
  };

  const result = inferencePolicySchema.safeParse(input);

  expect(result.success).toBe(true);
});

test('validates with single kind', () => {
  const input = {
    providedByTeleportCloud: false,
    kinds: ['ssh'],
    model: 'anthropic.claude-v2',
  };

  const result = inferencePolicySchema.safeParse(input);

  expect(result.success).toBe(true);
});

test('rejects empty kinds array', () => {
  const input = {
    providedByTeleportCloud: false,
    kinds: [],
    model: 'gpt-4',
  };

  const result = inferencePolicySchema.safeParse(input);

  expect(result.success).toBe(false);
  expect(result.error.issues[0].message).toBe('At least one type is required');
});

test('rejects empty model', () => {
  const input = {
    providedByTeleportCloud: false,
    kinds: ['ssh'],
    model: '',
  };

  const result = inferencePolicySchema.safeParse(input);

  expect(result.success).toBe(false);
  expect(result.error.issues[0].message).toBe('This is required');
});

test('rejects invalid resource kind', () => {
  const input = {
    providedByTeleportCloud: false,
    kinds: ['invalid_kind'],
    model: 'gpt-4',
  };

  const result = inferencePolicySchema.safeParse(input);

  expect(result.success).toBe(false);
});

test('validates with desktop kind', () => {
  const input = {
    providedByTeleportCloud: false,
    kinds: ['desktop'],
    model: 'gpt-4',
  };

  const result = inferencePolicySchema.safeParse(input);

  expect(result.success).toBe(true);
});
