import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import { ApiError } from 'teleport/services/api/parseError';

import { pluginsService } from './plugins';

beforeEach(() => {
  jest.clearAllMocks();
});

test('create static auth plugins', async () => {
  jest.spyOn(api, 'postFormData').mockResolvedValue({});

  await pluginsService.createStaticAuthPlugin({} as any);

  expect(api.postFormData).toHaveBeenCalledTimes(1);
  expect(api.postFormData).toHaveBeenCalledWith(
    cfg.api.plugin.createStaticAuth,
    {},
    undefined
  );
});

test('create static auth plugins with fallback', async () => {
  const pathNotFoundErr = new ApiError({
    message: '',
    response: { status: 404 } as Response,
    proxyVersion: {
      major: 0,
      minor: 0,
      patch: 0,
      string: '',
      preRelease: '',
    },
  });
  jest
    .spyOn(api, 'postFormData')
    .mockRejectedValueOnce(pathNotFoundErr)
    .mockResolvedValue({});

  await pluginsService.createStaticAuthPlugin({} as any);

  expect(api.postFormData).toHaveBeenCalledTimes(2);
  expect(api.postFormData).toHaveBeenNthCalledWith(
    1,
    cfg.api.plugin.createStaticAuth,
    {},
    undefined
  );
  expect(api.postFormData).toHaveBeenNthCalledWith(
    2,
    cfg.api.plugin.createDeprecated,
    {},
    undefined
  );
});
