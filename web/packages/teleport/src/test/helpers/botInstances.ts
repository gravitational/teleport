import { http, HttpResponse } from 'msw';

import { ListBotInstancesResponse } from 'teleport/services/bot/types';

const listBotInstancesPath =
  '/v1/webapi/sites/:cluster_id/machine-id/bot-instance';

export const listBotInstancesSuccess = (mock: ListBotInstancesResponse) =>
  http.get(listBotInstancesPath, () => {
    return HttpResponse.json(mock);
  });

export const listBotInstancesError = (status: number) =>
  http.get(listBotInstancesPath, () => {
    return new HttpResponse(null, { status });
  });
