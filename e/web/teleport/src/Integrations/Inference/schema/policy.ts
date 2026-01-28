import { z } from 'zod';

import { ResourceKind } from './types';

/**
 * This defines the schema for the policy part of an inference integration.
 */

export const inferencePolicySchema = z.object({
  kinds: z.array(ResourceKind).min(1, 'At least one type is required'),
  model: z.string().min(1, 'This is required'),
});
