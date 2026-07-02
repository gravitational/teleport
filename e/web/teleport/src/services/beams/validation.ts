import { z } from 'zod';

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
});

const listBeamsResponseSchema = z.object({
  items: z.array(beamSchema),
  next_page_token: z.string(),
});

export type Beam = z.output<typeof beamSchema>;
export type BeamsListResponse = z.output<typeof listBeamsResponseSchema>;

export function parseListBeamsResponse(
  data: unknown
): data is BeamsListResponse {
  return listBeamsResponseSchema.safeParse(data).success;
}

export function parseBeamResponse(data: unknown): data is Beam {
  return beamSchema.safeParse(data).success;
}
