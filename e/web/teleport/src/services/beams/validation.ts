import { z } from 'zod';

import { Beam, BeamsListResponse } from './types';

export function parseListBeamsResponse(
  data: unknown
): data is BeamsListResponse {
  const result = listBeamsResponseSchema.safeParse(data);
  return result.success;
}

export function parseBeamResponse(data: unknown): data is Beam {
  const result = beamSchema.safeParse(data);
  return result.success;
}

const beamSchema = z.object({
  name: z.string(),
  alias: z.string(),
  user: z.string(),
  expires: z.string(),
  node_id: z.string(),
  app_name: z.string(),
  egress_mode: z.enum(['', 'unrestricted', 'restricted']),
  allowed_domains: z.array(z.string()),
  publish: z
    .object({
      port: z.number(),
      protocol: z.enum(['http', 'tcp']),
    })
    .optional(),
  compute_status: z.enum(['', 'provision_pending', 'provision_complete']),
}) satisfies z.ZodType<Beam>;

const listBeamsResponseSchema = z.object({
  items: z.array(beamSchema),
  next_page_token: z.string(),
}) satisfies z.ZodType<BeamsListResponse>;
