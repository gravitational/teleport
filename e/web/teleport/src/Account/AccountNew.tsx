import React from 'react';
import AccountPage from 'teleport/Account/AccountNew';

import cfg from 'e-teleport/config';

import Recovery from './Recovery/RecoveryNew';

export default function Account() {
  return (
    <AccountPage
      enterpriseComponent={cfg.oss.recoveryCodesEnabled ? Recovery : undefined}
    />
  );
}
