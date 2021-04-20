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
import CatchError from 'teleport/components/CatchError';
import cfg from 'teleport/config';
import SideNav from 'teleport/SideNav';
import TopBar from 'teleport/TopBar';
import getFeatures from 'teleport/features';
import useMain, { State } from './useMain';

export default function Container() {
  const [features] = React.useState(() => getFeatures());
  const state = useMain(features);
  return <Main {...state} />;
}

export function Main(props: State) {
  const { status, statusText, ctx } = props;

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

  return (
    <>
      <RouterDOM.Switch>
        <Redirect exact={true} from={cfg.routes.root} to={indexRoute} />
      </RouterDOM.Switch>
      <StyledMain>
        <SideNav />
        <HorizontalSplit>
          <TopBar />
          <Switch>{$features}</Switch>
        </HorizontalSplit>
      </StyledMain>
    </>
  );
}

export const StyledMain = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  flex: 1;
  position: absolute;
  min-width: 1000px;
`;

const HorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;

  // Allows shrinking beyond content size on flexed childrens.
  min-width: 0;
`;

const StyledIndicator = styled(HorizontalSplit)`
  align-items: center;
  justify-content: center;
`;
