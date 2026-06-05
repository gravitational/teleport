import { http, HttpResponse } from 'msw';

import cfg from 'e-teleport/config';
import { ListBeamsResponse } from 'e-teleport/services/beams/api';
import { JsonObject } from 'teleport/types';

export const listBeamsSuccess = (
  mock: ListBeamsResponse = {
    items: [
      {
        name: 'a0f98569-2559-42e0-8ac3-2bc6bf5db6c9',
        alias: 'cosmic-author',
        expires: '2026-04-25T16:30:00Z',
        user: 'user@example.com',
        delegation_session_id: '6f6caa47-adbb-49fb-ae01-d0c8508b6ff3',
        node_id: '346613bb-78b8-4aeb-bbb0-937538ce8595',
        app_name: 'a7df5e45-99bb-4021-ac9a-9441d04c2f21',
      },
      {
        name: 'c960afa9-0307-4b34-9feb-8b6a2cd30925',
        alias: 'silent-light',
        expires: '2026-05-30T00:00:00Z',
        user: 'user@example.com',
        delegation_session_id: 'faa57ab2-60f7-4ecc-b9bc-1c061015818b',
        node_id: '98968eaa-fdd2-486d-b2a5-905cdd1e6352',
        app_name: '',
      },
      {
        name: 'c960afa9-0307-4b34-9feb-8b6a2cd30925',
        alias: 'solid-flux',
        expires: '2027-04-24T00:00:00Z',
        user: 'user@example.com',
        delegation_session_id: '',
        node_id: '',
        app_name: '',
      },
    ],
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
