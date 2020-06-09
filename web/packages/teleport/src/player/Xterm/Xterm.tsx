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
import React, { useEffect, useRef } from 'react';
import Terminal from 'teleport/lib/term/terminal';
import Tty from 'teleport/lib/term/tty';
import StyledXterm from 'teleport/console/components/StyledXterm';

export default function Xterm({ tty }: { tty: Tty }) {
  const refContainer = useRef<HTMLElement>();

  useEffect(() => {
    const term = new TerminalPlayer(tty, {
      el: refContainer.current,
      scrollBack: 1000,
    });

    window['mama'] = term;

    term.open();
    term.term.focus();

    function cleanup() {
      term.destroy();
    }

    return cleanup;
  }, [tty]);

  return <StyledXterm p="2" ref={refContainer} />;
}

class TerminalPlayer extends Terminal {
  // do not attempt to connect
  connect() {}

  resize(cols, rows) {
    // ensure that cursor is visible as xterm hides it on blur event
    this.term.cursorState = 1;
    super.resize(cols, rows);
  }

  // prevent user resize requests
  _requestResize() {}
}
