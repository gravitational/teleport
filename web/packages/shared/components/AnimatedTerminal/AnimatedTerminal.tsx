/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useEffect, useMemo, useRef, useState } from 'react';

import {
  KeywordHighlight,
  SelectedLines,
  TerminalContent,
} from 'shared/components/AnimatedTerminal/TerminalContent';

import { Window } from 'shared/components/Window';

import { BufferEntry, createTerminalContent, TerminalLine } from './content';

interface AnimatedTerminalProps {
  lines: TerminalLine[];
  startDelay?: number;
  highlights?: KeywordHighlight[];
  selectedLines?: SelectedLines;
  stopped?: boolean;
  onCompleted?: () => void;
}

export function AnimatedTerminal(props: AnimatedTerminalProps) {
  const lastLineIndex = useRef(0);
  const content = useMemo(
    () => createTerminalContent(props.lines, lastLineIndex.current),
    [props.lines]
  );

  const [counter, setCounter] = useState(0);
  const [completed, setCompletedState] = useState(false);

  const lines = useRef<BufferEntry[]>([]);

  function setCompleted(completed: boolean) {
    setCompletedState(completed);
    if (completed) {
      props.onCompleted && props.onCompleted();
    }
  }

  useEffect(() => {
    let timeout: number;
    let request: number;

    async function animate() {
      const { value, done } = await content.next();

      if (value) {
        if (value.length) {
          const nextLineIndex = value[value.length - 1].id + 1;
          if (nextLineIndex > lastLineIndex.current) {
            lastLineIndex.current = nextLineIndex;
          }
        }

        lines.current = value;
        setCounter(count => count + 1);
      }

      if (done) {
        setCompleted(true);
        setCounter(count => count + 1);

        return;
      }

      request = requestAnimationFrame(animate);
    }

    function setup() {
      request = requestAnimationFrame(animate);
    }

    if (!props.startDelay) {
      setup();
    } else {
      timeout = window.setTimeout(setup, props.startDelay);
    }

    return () => {
      cancelAnimationFrame(request);
      clearTimeout(timeout);
    };
  }, [props.startDelay, props.lines, content]);

  let renderedLines = lines.current;
  if (props.stopped) {
    renderedLines = props.lines.map((line, index) => ({
      id: index,
      text: line.text,
      isCommand: line.isCommand,
      isCurrent: index === props.lines.length - 1,
    }));
  }

  return (
    <Window title="Terminal">
      <TerminalContent
        lines={renderedLines}
        completed={completed}
        counter={counter}
        highlights={props.highlights}
        selectedLines={props.selectedLines}
      />
    </Window>
  );
}
