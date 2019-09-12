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
import AppList from './AppList';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';
import LatestEventList from './LatestEventList';
import OperationList from './OperationList';
import DebugInfoButton from './DebugInfoButton';
import MetricsCharts from './MetricsCharts';
import OperationBanner from './OperationBanner';

export default function Dashboard(){
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          Dashboard
        </FeatureHeaderTitle>
        <DebugInfoButton ml="auto"/>
      </FeatureHeader>
      <OperationBanner mb="4"/>
      <MetricsCharts/>
      <LatestEventList mb="4"/>
      <OperationList mb="4" />
      <AppList/>
    </FeatureBox>
  );
}