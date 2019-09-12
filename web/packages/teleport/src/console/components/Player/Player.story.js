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
import $ from 'jQuery';
import styled from 'styled-components';
import { storiesOf } from '@storybook/react';
import Player from './Player';
import { TtyPlayer } from 'gravity/lib/term/ttyPlayer';
import sample from 'gravity/lib/term/fixtures/streamData';

storiesOf('TeleportConsole/Player', module)
  .add('loading', () => {
    const tty = new TtyPlayer('url');
    tty.connect = () => null;
    return <MockedPlayer tty={tty} />;
  })
  .add('error', () => {
    const tty = new TtyPlayer('url');
    tty.connect = () => null;
    tty.handleError('Unable to find');
    return <MockedPlayer tty={tty} />;
  })
  .add('not available (proxy enabled)', () => {
    const tty = new TtyPlayer('url');
    tty.connect = () => null;
    tty._setStatusFlag({ isReady: true });
    return <MockedPlayer tty={tty} />;
  })
  .add('with content', () => {
    const tty = new TtyPlayer('url');
    const events = tty._eventProvider._createEvents(sample.events);
    tty._eventProvider._fetchEvents = () => $.Deferred().resolve(events);
    tty._eventProvider._fetchContent = () => $.Deferred().resolve(sample.data);
    return (
      <Box>
        <MockedPlayer tty={tty} />
      </Box>
    );
  });

class MockedPlayer extends Player {
  constructor(props) {
    super({
      ...props,
      match: {
        params: {
          siteId: 'xxx',
          sid: 'bbb',
        },
      },
    });
    this.tty = props.tty;
  }
}

const Box = styled.div`
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  position: absolute;
`;
