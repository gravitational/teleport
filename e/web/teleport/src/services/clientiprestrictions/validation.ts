import { z } from 'zod';

/** An empty value is treated as "enforced" for backward compatibility. */
const modes = ['draft', 'enforced', ''] as const;

const states = [
  'draft',
  'pending',
  'active',
  'expired',
  'unknown',
  '',
] as const;

/**
 * Validates the endpoint's response and fills in what it leaves out: every field
 * but `cidrs` is `omitempty` on the Go side, and `cidrs` itself arrives as null
 * when there are none. The defaults here are what let the rest of the panel read
 * these fields without guarding each one.
 */
const clientIpRestrictionSchema = z.object({
  cidrs: z
    .array(z.string())
    .nullish()
    .transform(cidrs => cidrs ?? []),
  mode: z.enum(modes).default(''),
  expires: z.string().optional(),
  status: z.enum(states).default(''),
  revision: z.string().default(''),
});

export type ClientIpRestrictionMode = (typeof modes)[number];
export type ClientIpRestrictionState = (typeof states)[number];
export type ClientIpRestriction = z.output<typeof clientIpRestrictionSchema>;

export function parseClientIpRestrictionResponse(
  data: unknown
): ClientIpRestriction {
  const result = clientIpRestrictionSchema.safeParse(data);
  if (!result.success) {
    throw new Error('failed to parse client IP restriction response');
  }
  return result.data;
}
