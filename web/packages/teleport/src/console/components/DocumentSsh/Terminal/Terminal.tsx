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
import { Flex } from 'design';
import Tty from 'teleport/lib/term/tty';
import XTermCtrl from 'teleport/lib/term/terminal';
import StyledXterm from '../../StyledXterm';
import { getMappedAction } from 'teleport/console/useKeyboardNav';

export default class Terminal extends React.Component<{ tty: Tty }> {
  terminal: XTermCtrl;

  refTermContainer = React.createRef();

  componentDidMount() {
    this.terminal = new XTermCtrl(this.props.tty, {
      el: this.refTermContainer.current,
    });

    this.terminal.open();

    // TODO deprecated, use attachCustomKeyEventHandler when we upgrade xterm
    this.terminal.term.attachCustomKeydownHandler(event => {
      const { tabSwitch } = getMappedAction(event);
      if (tabSwitch) {
        return false;
      }
    });
  }

  componentWillUnmount() {
    this.terminal.destroy();
  }

  shouldComponentUpdate() {
    return false;
  }

  focus() {
    this.terminal.term.focus();
  }

  render() {
    return (
      <Flex
        flexDirection="column"
        height="100%"
        width="100%"
        px="2"
        style={{ overflow: 'auto' }}
      >
        <StyledXterm ref={this.refTermContainer} />
      </Flex>
    );
  }
}
