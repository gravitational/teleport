import React from 'react';
import { Route } from 'teleport/components/Router';
import Teleport, {
  renderPublicRoutes,
  renderPrivateRoutes,
  Props,
} from 'teleport/Teleport';

import cfg from 'e-teleport/config';
import WaitingRoom from 'e-teleport/WaitingRoom';
import { Discover } from 'e-teleport/Discover';

import { getEnterpriseFeatures } from 'e-teleport/features';

import Login from './Login';
import Recovery from './Recovery';
import Main from './Main';

const TeleportE: React.FC<Props> = ({ history, ctx }) => {
  return (
    <Teleport
      history={history}
      features={getEnterpriseFeatures()}
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
      key="ent-4"
      title="Recovery"
      path={cfg.routes.recovery}
      component={Recovery}
    />,
    ...renderPublicRoutes(),
  ];
}

function privateRoutes() {
  return <WaitingRoom>{renderPrivateRoutes(Main, Discover)}</WaitingRoom>;
}

export default TeleportE;
