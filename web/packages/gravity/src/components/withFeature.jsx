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
import * as Cards from 'design/CardError';
import Indicator from 'design/Indicator';
import { useStore } from 'gravity/lib/stores';
import FeatureBase, { Activator } from 'gravity/lib/featureBase';
import CatchError from 'gravity/components/CatchError';

const withFeature = feature => component => {
  function FeatureWrapper(props) {
    // subscribe to feature store changes
    useStore(feature);

    if (feature.isProcessing()) {
      return (
        <StyledIndicator>
          <Indicator delay="long" />
        </StyledIndicator>
      );
    }

    if (feature.isFailed()) {
      const errorText = feature.state.statusText;
      return <Cards.Failed message={errorText} />;
    }

    return React.createElement(component, {
      ...props,
      feature,
    });
  }

  return function WithFeatureWrapper(props) {
    return (
      <CatchError>
        <FeatureWrapper {...props} />
      </CatchError>
    );
  };
};

const StyledIndicator = styled.div`
  align-items: center;
  display: flex;
  height: 200px;
  width: 100%;
  justify-content: center;
`;

export default withFeature;

export { FeatureBase, Activator };
