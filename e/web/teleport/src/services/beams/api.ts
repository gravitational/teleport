import { z } from 'zod';

export function parseListBeamsResponse(
  data: unknown
): data is ListBeamsResponse {
  const result = listBeamsResponseSchema.safeParse(data);
  return result.success;
}

const listBeamsResponseSchema = z.object({
  items: z
    .array(
      z.object({
        name: z.string(),
        alias: z.string().nullish(),
        user: z.string(),
        expires: z.string(),
        delegation_session_id: z.string().nullish(),
        node_id: z.string().nullish(),
        app_name: z.string().nullish(),
      })
    )
    .nullish(),
  next_page_token: z.string().nullish(),
});

export type ListBeamsResponse = z.output<typeof listBeamsResponseSchema>;
