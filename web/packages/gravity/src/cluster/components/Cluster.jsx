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

import React from 'react';
import styled from 'styled-components';
import { Switch, Route } from 'gravity/components/Router';
import { Indicator  } from 'design';
import SideNav from './SideNav';
import { Failed } from 'design/CardError';
import TopBar from './TopBar';
import Offline from './Offline';
import * as Layout from './Layout';
import cfg from 'gravity/config';
import { useAttempt } from 'shared/hooks';

// Cluster is main cluster component
export default function Cluster({features, onInit}){
  return (
    <Switch>
      <Route path={cfg.routes.siteOffline} component={Offline} />
      <ClusterContent features={features} onInit={onInit}/>
    </Switch>
  );
}

function ClusterContent({ features, onInit}){
  const [ attempt, attemptActions ] = useAttempt();
  const { isFailed, isSuccess, message } = attempt;

  React.useEffect(() => {
    attemptActions.do(() => {
      return onInit();
    });
  }, [])

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

  // render allowed features
  const allowedFeatures = features.filter(f => !f.isDisabled());
  const $features = allowedFeatures.map((item, index) => {
    const { path, title, exact, component } = item.getRoute();
    return (
      <Route
        title={title}
        key={index}
        path={path}
        exact={exact}
        component={component}/>
    )})

  return (
    <Layout.AppVerticalSplit>
      <SideNav />
      <Layout.AppHorizontalSplit>
        <TopBar pl="6" />
        <Switch>
          {$features}
        </Switch>
      </Layout.AppHorizontalSplit>
    </Layout.AppVerticalSplit>
  )
}

const StyledIndicator = styled(Layout.AppVerticalSplit)`
  align-items: center;
  justify-content: center;
`