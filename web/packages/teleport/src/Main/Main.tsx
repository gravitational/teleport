/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import * as RouterDOM from 'react-router-dom';
import React, { Suspense } from 'react';
import styled from 'styled-components';
import { Indicator } from 'design';
import { Failed } from 'design/CardError';

import { Redirect, Switch, Route } from 'teleport/components/Router';
import { CatchError } from 'teleport/components/CatchError';
import cfg from 'teleport/config';
import SideNav from 'teleport/SideNav';
import TopBar from 'teleport/TopBar';
import { BannerList } from 'teleport/components/BannerList';
import localStorage from 'teleport/services/localStorage';
import history from 'teleport/services/history';

import { ClusterAlert, LINK_LABEL } from 'teleport/services/alerts/alerts';

import { MainContainer } from './MainContainer';
import { OnboardDiscover } from './OnboardDiscover';
import useMain from './useMain';

import type { BannerType } from 'teleport/components/BannerList/BannerList';

interface MainProps {
  initialAlerts?: ClusterAlert[];
  customBanners?: React.ReactNode[];
}

export function Main(props: MainProps) {
  const { alerts, ctx, customBanners, dismissAlert, status, statusText } =
    useMain({
      initialAlerts: props.initialAlerts,
      customBanners: props.customBanners,
    });

  const [showOnboardDiscover, setShowOnboardDiscover] = React.useState(true);

  if (status === 'failed') {
    return <Failed message={statusText} />;
  }

  if (status !== 'success') {
    return (
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
    );
  }

  function handleOnboard() {
    updateOnboardDiscover();
    history.push(cfg.routes.discover);
  }

  function handleOnClose() {
    updateOnboardDiscover();
    setShowOnboardDiscover(false);
  }

  function updateOnboardDiscover() {
    const discover = localStorage.getOnboardDiscover();
    localStorage.setOnboardDiscover({ ...discover, notified: true });
  }

  // render feature routes
  const $features = ctx.features.map((f, index) => {
    const { path, title, exact, component } = f.route;
    const Cmpt = component;
    return (
      <Route title={title} key={index} path={path} exact={exact}>
        <CatchError>
          <Suspense fallback={null}>
            <Cmpt />
          </Suspense>
        </CatchError>
      </Route>
    );
  });

  // default feature to show when hitting the index route
  const indexRoute =
    ctx.storeNav.getSideItems()[0]?.getLink(cfg.proxyCluster) ||
    cfg.routes.support;

  // The backend defines the severity as an integer value with the current
  // pre-defined values: LOW: 0; MEDIUM: 5; HIGH: 10
  const mapSeverity = (severity: number) => {
    if (severity < 5) {
      return 'info';
    }
    if (severity < 10) {
      return 'warning';
    }
    return 'danger';
  };

  const banners: BannerType[] = alerts.map(alert => ({
    message: alert.spec.message,
    severity: mapSeverity(alert.spec.severity),
    link: alert.metadata.labels[LINK_LABEL],
    id: alert.metadata.name,
  }));

  const onboard = localStorage.getOnboardDiscover();
  const requiresOnboarding =
    onboard && !onboard.hasResource && !onboard.notified;

  return (
    <>
      <RouterDOM.Switch>
        <Redirect exact={true} from={cfg.routes.root} to={indexRoute} />
      </RouterDOM.Switch>
      <BannerList
        banners={banners}
        customBanners={customBanners}
        onBannerDismiss={dismissAlert}
      >
        <MainContainer>
          <SideNav />
          <HorizontalSplit>
            <ContentMinWidth>
              <TopBar />
              <Switch>{$features}</Switch>
            </ContentMinWidth>
          </HorizontalSplit>
        </MainContainer>
      </BannerList>
      {requiresOnboarding && showOnboardDiscover && (
        <OnboardDiscover onClose={handleOnClose} onOnboard={handleOnboard} />
      )}
    </>
  );
}

export const ContentMinWidth = styled.div`
  min-width: calc(1250px - var(--sidebar-width));
`;

export const HorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow-x: auto;
`;

export const StyledIndicator = styled(HorizontalSplit)`
  align-items: center;
  justify-content: center;
`;
