import React from 'react';
import AccountPage from 'teleport/Account/AccountNew';

import Recovery from './Recovery/RecoveryNew';

export default function Account() {
  return <AccountPage enterpriseComponent={Recovery} />;
}
