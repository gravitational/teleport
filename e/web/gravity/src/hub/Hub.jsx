import React from 'react';
import styled from 'styled-components';
import { Redirect, Switch, Route } from 'gravity/components/Router';
import session from 'gravity/services/session';
import userGetters from 'gravity/flux/user/getters';
import { useFluxStore } from 'gravity/components/nuclear';
import { getters as navGetters } from 'e-gravity/hub/flux/nav';
import { Indicator  } from 'design';
import { withState, useAttempt  } from 'shared/hooks';
import { Failed } from 'design/CardError';
import { initHub } from './flux/actions';
import FeatureHubLicenses from './features/featureHubLicenses';
import FeatureHubClusters from './features/featureHubClusters';
import FeatureHubCatalog from './features/featureHubCatalog';
import FeatureHubSettings from './features/featureHubSettings';
import FeatureHubAccess from './features/featureHubAccess';
import HubTopNav from './components/HubTopNav';
import * as Layout from './components/components/Layout';
import 'gravity/flux';
import './flux';
import cfg from 'e-gravity/config';

export function Hub({ features, attempt, userName, onLogout, navItems }) {
  const { isFailed, isSuccess, message } = attempt;

  if(isFailed){
    return <Failed message={message} />;
  }

  if(!isSuccess){
    return (
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
    )
  }

  const allowedFeatures = features.filter( f => !f.isDisabled() );
  const $features = allowedFeatures.map((item, index) => {
    const { path, title, exact, component } = item.getRoute();
    return (
      <Route
        title={title}
        key={index}
        path={path}
        exact={exact}
        component={component}
      />
    )
  })

  // handle index route
  const indexTab = navItems.length > 0 ? navItems[0].to : null;

  return (
    <StyledLayout>
      <HubTopNav
        userName={userName}
        onLogout={onLogout}
        items={navItems}
      />
      <Switch>
        { indexTab && <Redirect exact from={cfg.routes.defaultEntry} to={indexTab}/> }
        {$features}
      </Switch>
    </StyledLayout>
  );
}

const StyledLayout = styled.div`
  flex-direction: column;
  position: absolute;
  width: 100%;
  height: 100%;
  display: flex;
  overflow: hidden;
`

const StyledIndicator = styled(Layout.AppVerticalSplit)`
  align-items: center;
  justify-content: center;
`

export default withState(() => {
  // global stores
  const userStore = useFluxStore(userGetters.user);
  const navStore = useFluxStore(navGetters.navStore);

  // local state
  const [ attempt, attemptActions ] = useAttempt();
  const [ features ] = React.useState(() => {
    return [
      new FeatureHubClusters(),
      new FeatureHubCatalog(),
      new FeatureHubLicenses(),
      new FeatureHubAccess(),
      new FeatureHubSettings(),
    ]
  });

  React.useEffect(() => {
    // initialize hub stores
    attemptActions.do(() => initHub(features));
  }, []);

  const userName = userStore ? userStore.userId : '';

  return {
    features,
    attempt,
    userName,
    navItems: navStore.topNav,
    onLogout: () => session.logout(),
  }
})(Hub);