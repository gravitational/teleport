import React from 'react';
import { Route } from 'teleport/components/Router';
import Teleport, {
  renderPublicRoutes,
  renderPrivateRoutes,
  Props,
} from 'teleport/Teleport';

import ossConfig from 'teleport/config';

import cfg from 'e-teleport/config';
import WaitingRoom from 'e-teleport/WaitingRoom';

import { getEnterpriseFeatures } from 'e-teleport/features';

const TeleportE: React.FC<Props> = ({ history, ctx }) => {
  return (
    <Teleport
      history={history}
      features={getEnterpriseFeatures()}
      ctx={ctx}
      renderPublicRoutes={publicERoutes}
      renderPrivateRoutes={privateERoutes}
    />
  );
};

const Login = React.lazy(
  () => import(/* webpackChunkName: "e-welcome" */ './Login')
);

const Recovery = React.lazy(
  () => import(/* webpackChunkName: "e-recovery" */ './Recovery')
);

function publicERoutes() {
  return renderPublicRoutes(
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
    />
  );
}

const Main = React.lazy(
  () => import(/* webpackChunkName: "e-main" */ './Main')
);

const Discover = React.lazy(() => import('e-teleport/Discover'));

function privateERoutes() {
  return renderPrivateRoutes(
    <WaitingRoom key="waiting room">
      <Route path={ossConfig.routes.discover} component={Discover} />
      <Route path={ossConfig.routes.root} component={Main} />
    </WaitingRoom>
  );
}

export default TeleportE;
