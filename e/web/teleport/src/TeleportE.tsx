import React from 'react';
import { Route } from 'teleport/components/Router';
import Teleport, {
  renderPublicRoutes,
  renderPrivateRoutes,
  Props,
} from 'teleport/Teleport';
import cfg from 'e-teleport/config';
import WaitingRoom from 'e-teleport/WaitingRoom';
import { LicenseEnforcer } from './License';
import Login from './Login';
import Recovery from './Recovery';
import Invite, { ResetPassword } from './Invite';
import Main from './Main';

const TeleportE: React.FC<Props> = ({ history, ctx }) => {
  return (
    <Teleport
      history={history}
      ctx={ctx}
      renderPublicRoutes={publicRoutes}
      renderPrivateRoutes={privateRoutes}
    />
  );
};

function publicRoutes() {
  return [
    <Route
      key="ent-1"
      title="Login"
      path={cfg.oss.routes.login}
      component={Login}
    />,
    <Route
      key="ent-2"
      title="Invite"
      path={cfg.oss.routes.userInvite}
      component={Invite}
    />,
    <Route
      key="ent-3"
      title="Password Reset"
      path={cfg.oss.routes.userReset}
      component={ResetPassword}
    />,
    <Route
      key="ent-4"
      title="Recovery"
      path={cfg.routes.recovery}
      component={Recovery}
    />,
    ...renderPublicRoutes(),
  ];
}

function privateRoutes() {
  return (
    <WaitingRoom>
      <LicenseEnforcer />
      {renderPrivateRoutes(Main)}
    </WaitingRoom>
  );
}

export default TeleportE;
