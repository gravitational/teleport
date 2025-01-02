import cfg from 'e-teleport/config';
import { AccountPage } from 'teleport/Account';

import Recovery from './Recovery';
import { UserTrustedDevices } from './UserTrustedDevices';

export function Account() {
  return (
    <AccountPage
      enterpriseComponent={cfg.oss.recoveryCodesEnabled ? Recovery : undefined}
      userTrustedDevicesComponent={UserTrustedDevices}
    />
  );
}
