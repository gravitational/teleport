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
import BpfViewer, { formatEvents } from 'shared/components/BpfViewer';
import useTtyBpfMapper from './useTtyBpfMapper';

export default function BpfLogs({ tty, events, split }) {
  const cursor = useTtyBpfMapper(tty, events);
  const ref = React.useRef();

  React.useEffect(() => {
    const { session, editor } = ref.current;
    const length = session.getLength();
    const doc = session.getDocument();

    // clear the content and insert the first line
    if (cursor === 0) {
      doc.removeFullLines(0, length);
      editor.insert(formatEvents([events[0]]).join('\n'));
    } else if (cursor > length) {
      const sliced = formatEvents(events.slice(length, cursor));
      const formatted = `\n${sliced.join('\n')}`;
      session.insert({ row: length, column: 0 }, formatted);
    } else if (cursor < length) {
      doc.removeFullLines(cursor, length);
    }

    editor.gotoLine(cursor);
  }, [cursor]);

  React.useEffect(() => {
    ref.current.editor.resize();
  }, [split]);

  return <BpfViewer showGutter={false} ref={ref} events={[]} />;
}
