import { MemoryRouter } from 'react-router-dom';

import cfg from 'e-teleport/config';

import { SSOConfirm } from './SSOConfirm';

export default {
  title: 'TeleportE/SSOConfirm',
};

export function Error() {
  return (
    <MemoryRouter>
      <SSOConfirm />
    </MemoryRouter>
  );
}

export function Success() {
  return (
    <MemoryRouter
      initialEntries={[
        {
          pathname: cfg.routes.ssoConfirm,
          search: '?channel_id=123&response={"mfa_token": "token"}',
        },
      ]}
    >
      <SSOConfirm />
    </MemoryRouter>
  );
}
