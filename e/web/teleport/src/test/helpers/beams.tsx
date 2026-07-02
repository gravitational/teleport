import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { ReactNode } from 'react';

import cfg from 'e-teleport/config';
import { Beam, BeamsListResponse } from 'e-teleport/services/beams/types';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { makeAcl } from 'teleport/services/user/makeAcl';
import { JsonObject } from 'teleport/types';

export const defaultBeams: Beam[] = [
  {
    name: 'a0f98569-2559-42e0-8ac3-2bc6bf5db6c9',
    alias: 'cosmic-author',
    expires: '2026-04-25T16:30:00Z',
    user: 'user@example.com',
    node_id: '346613bb-78b8-4aeb-bbb0-937538ce8595',
    app_name: 'a7df5e45-99bb-4021-ac9a-9441d04c2f21',
    egress_mode: 'unrestricted',
    allowed_domains: [],
    publish: { port: 8080, protocol: 'http' },
    compute_status: 'provision_complete',
  },
  {
    name: 'c960afa9-0307-4b34-9feb-8b6a2cd30925',
    alias: 'silent-light',
    expires: '2026-05-30T00:00:00Z',
    user: 'user@example.com',
    node_id: '98968eaa-fdd2-486d-b2a5-905cdd1e6352',
    app_name: '',
    egress_mode: 'unrestricted',
    allowed_domains: [],
    compute_status: 'provision_complete',
  },
  {
    name: 'd5a82b41-7f3c-4d18-a25b-2c1e6e1d0b4a',
    alias: 'solid-flux',
    expires: '2027-04-24T00:00:00Z',
    user: 'user@example.com',
    node_id: '',
    app_name: '',
    egress_mode: 'unrestricted',
    allowed_domains: [],
    compute_status: 'provision_pending',
  },
];

export const listBeamsSuccess = (
  mock: BeamsListResponse = {
    items: defaultBeams,
    next_page_token: 'page-token-1',
  }
) =>
  http.get(cfg.api.beams.list, () => {
    return HttpResponse.json(mock);
  });

export const listBeamsForever = () =>
  http.get(
    cfg.api.beams.list,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );

export const listBeamsError = (
  status: number,
  error: string | null = null,
  fields: JsonObject = {}
) =>
  http.get(cfg.api.beams.list, () => {
    return HttpResponse.json({ error: { message: error }, fields }, { status });
  });

export const createBeamSuccess = (beam: Beam) =>
  http.post(cfg.api.beams.list, () => HttpResponse.json(beam));

export const getBeamSuccess = (beam: Beam) =>
  http.get(cfg.api.beams.get, () => HttpResponse.json(beam));

export const getBeamNotFound = () =>
  http.get(cfg.api.beams.get, () =>
    HttpResponse.json({ error: { message: 'not found' } }, { status: 404 })
  );

export const updateBeamSuccess = (beam: Beam) =>
  http.put(cfg.api.beams.get, () => HttpResponse.json(beam));

export const deleteBeamSuccess = () =>
  http.delete(cfg.api.beams.get, () => HttpResponse.json({}));

export function BeamsProviders({
  children,
  acl,
  queryClient,
  initialEntries = [cfg.routes.beamsList],
}: {
  children: ReactNode;
  acl: ReturnType<typeof makeAcl>;
  queryClient: QueryClient;
  initialEntries?: string[];
}) {
  const ctx = createTeleportContext({ customAcl: acl });
  return (
    <QueryClientProvider client={queryClient}>
      <TeleportProviderBasic teleportCtx={ctx} initialEntries={initialEntries}>
        {children}
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}
