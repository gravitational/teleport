import { useMemo } from 'react';

import { Box, Indicator } from 'design';

import cfg from 'e-teleport/config';
import RecoveryService from 'e-teleport/services/recovery/recovery';
import { Route, Switch } from 'teleport/components/Router';

import Devices from './Devices';
import Invalid from './InvalidLink';
import NewMfaDevice from './NewMfaDevice';
import NewPassword from './NewPassword';
import NewRecoveryCodes from './NewRecoveryCodes';
import useRecoveryFlow, { State } from './useRecoveryFlow';
import VerifyUser from './VerifyUser';

export default function Container() {
  const recoveryService = useMemo(() => new RecoveryService(), []);
  const state = useRecoveryFlow(recoveryService);
  return <RecoveryFlow {...state} />;
}

export function RecoveryFlow({
  attempt,
  recoveryService,
  token,
  goToNewCredential,
  goToDevices,
  goToCodes,
}: State) {
  if (attempt.status === 'processing') {
    return (
      <Box mx="auto" textAlign="center">
        <Indicator />
      </Box>
    );
  }

  if (attempt.status === 'failed') {
    return <Invalid />;
  }

  return (
    <Switch>
      <Route exact path={cfg.routes.recoveryStepVerify}>
        <VerifyUser
          token={token}
          recoveryService={recoveryService}
          done={goToNewCredential}
        />
      </Route>
      <Route exact path={cfg.routes.recoveryStepNewPassword}>
        <NewPassword
          tokenId={token.id}
          recoveryService={recoveryService}
          onNext={goToCodes}
        />
      </Route>
      <Route exact path={cfg.routes.recoveryStepNewDevice}>
        <NewMfaDevice
          tokenId={token.id}
          recoveryService={recoveryService}
          qrCode={token.qrCode}
          onNext={goToDevices}
        />
      </Route>
      <Route exact path={cfg.routes.recoveryStepDevices}>
        <Devices tokenId={token.id} onNext={goToCodes} />
      </Route>
      <Route exact path={cfg.routes.recoveryStepCodes}>
        <NewRecoveryCodes
          tokenId={token.id}
          recoveryService={recoveryService}
        />
      </Route>
    </Switch>
  );
}
