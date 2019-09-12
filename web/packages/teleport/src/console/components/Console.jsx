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
import cfg from 'teleport/config';
import { Route, Switch } from 'teleport/components/Router';
import Terminal from './Terminal';
import Player from './Player';
import { colors } from './colors';
import SessionCreator from './SessionCreator';

export default function Console(props) {
  const [isReady, setReady] = React.useState(false);
  React.useEffect(() => {
    props.onInit().then(() => setReady(true));
  }, []);

  if (!isReady) {
    return null;
  }
  return (
    <StyledConsole>
      <Switch>
        <Route path={cfg.routes.consoleSession} component={Terminal} />
        <Route path={cfg.routes.consoleConnect} component={SessionCreator} />
        <Route path={cfg.routes.consolePlayer} component={Player} />
      </Switch>
    </StyledConsole>
  );
}

const StyledConsole = styled.div`
  background-color: ${colors.bgTerminal};
  bottom: 0;
  left: 0;
  position: absolute;
  right: 0;
  top: 0;
`;
