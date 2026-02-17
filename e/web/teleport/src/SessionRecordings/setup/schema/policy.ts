import { z } from 'zod';

import { TELEPORT_CLOUD_MODEL } from 'e-teleport/SessionRecordings/setup/schema/accessMethods';

import { ResourceKind } from './types';

/**
 * This defines the schema for the policy part of an inference integration.
 */

const customPolicySchema = z.object({
  providedByTeleportCloud: z.literal(false),
  kinds: z.array(ResourceKind).min(1, 'At least one type is required'),
  model: z.string().min(1, 'This is required'),
});

const cloudPolicySchema = z.object({
  providedByTeleportCloud: z.literal(true),
  kinds: z.array(ResourceKind).min(1, 'At least one type is required'),
  model: z.literal(TELEPORT_CLOUD_MODEL),
  acceptedTerms: z.boolean().refine(v => v === true, {
    message: 'You must accept the AI Features Terms',
  }),
});

export const inferencePolicySchema = z.discriminatedUnion(
  'providedByTeleportCloud',
  [cloudPolicySchema, customPolicySchema]
);

export type InferencePolicySchema = z.infer<typeof inferencePolicySchema>;
