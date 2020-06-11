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
import { TermEventEnum } from 'teleport/lib/term/enums';
import StyledXterm from 'teleport/console/components/StyledXterm';

export default function Xterm({ tty }: { tty: Tty }) {
  const refContainer = useRef<HTMLElement>();

  useEffect(() => {
    const term = new TerminalPlayer(tty, {
      el: refContainer.current,
      scrollBack: 1000,
    });

    term.open();
    term.term.focus();
    term.extendGetBufferCoords(refContainer);

    term.tty.on(TermEventEnum.DATA, () => {
      const el = term.term.element.querySelector('.terminal-cursor');
      el && el.scrollIntoView(false);
    });

    function cleanup() {
      term.destroy();
    }

    return cleanup;
  }, [tty]);

  return <StyledXterm p="2" ref={refContainer} />;
}

class TerminalPlayer extends Terminal {
  // do not attempt to connect
  connect() {
    // Disables terminal viewport scrolling so users can rely on players controls.
    this.term.viewport.onWheel = function() {};
    this.term.viewport.touchmove = function() {};
    this.term.viewport.touchstart = function() {};
  }

  /**
   * extendGetBufferCoords extends the original xterm's SelectionManager.prototype._getMouseBufferCoords
   * so that we can calculates the offset (differences of rows/cols) that results from the difference in
   * viewport scrolled position between child and parent viewports.
   *
   * @param ref The parent viewport that wraps around the actual terminal viewport (child).
   */
  extendGetBufferCoords(ref) {
    const parentEl = ref.current;
    const term = this.term;

    // Save reference to original code to get the coords from child viewport.
    const originalGetMouseBufferCoords = term.selectionManager._getMouseBufferCoords.bind(
      term.selectionManager
    );

    // Extends the original code to calculate the offset with the parent viewport.
    term.selectionManager._getMouseBufferCoords = function(e) {
      const coords = originalGetMouseBufferCoords(e);
      const lineHeight = term.charMeasure.height;
      const lineWidth = term.charMeasure.width;
      const viewportHeight = lineHeight * term.rows;
      const viewportWidth = lineWidth * term.cols;

      // Calculates differences of rows.
      let offsetY = 0;
      if (parentEl.scrollTop !== 0) {
        const scrollDiff = parentEl.scrollHeight - parentEl.scrollTop;
        if (scrollDiff > 0) {
          offsetY = Math.round((viewportHeight - scrollDiff) / lineHeight) + 1;
        }
      }

      // Calculates differences of columns.
      let offsetX = 0;
      if (parentEl.scrollLeft !== 0) {
        const scrollDiff = parentEl.scrollWidth - parentEl.scrollLeft;
        if (scrollDiff > 0) {
          offsetX = Math.round((viewportWidth - scrollDiff) / lineWidth) + 1;
        }
      }

      coords[0] += offsetX;
      coords[1] += offsetY;

      return coords;
    };
  }

  resize(cols, rows) {
    // ensure that cursor is visible as xterm hides it on blur event
    this.term.cursorState = 1;
    super.resize(cols, rows);
  }

  // prevent user resize requests
  _requestResize() {}
}
