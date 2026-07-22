/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import ace from 'ace-builds';
import { useEffect, useMemo, useState } from 'react';

import * as copyModule from 'design/utils/copyToClipboard';
import { render, screen, userEvent, waitFor } from 'design/utils/testing';
import * as downloadsModule from 'shared/utils/download';

import TextEditor from './TextEditor';

describe('TextEditor', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  test('copy content button', async () => {
    render(
      <TextEditor
        copyButton
        data={[
          {
            content: 'my-content',
            type: 'yaml',
          },
        ]}
      />
    );
    const mockedCopyToClipboard = jest.spyOn(copyModule, 'copyToClipboard');

    await userEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(mockedCopyToClipboard).toHaveBeenCalledWith('my-content');
  });

  test('download content button', async () => {
    render(
      <TextEditor
        downloadButton
        downloadFileName="test.yaml"
        data={[
          {
            content: 'my-content',
            type: 'yaml',
          },
        ]}
      />
    );
    const mockedDownload = jest.spyOn(downloadsModule, 'downloadObject');

    await userEvent.click(screen.getByTitle('Download'));
    expect(mockedDownload).toHaveBeenCalledWith('test.yaml', 'my-content');
  });

  describe('with a stateful wrapper', () => {
    test('editing', async () => {
      const onChange = jest.fn();

      render(
        <StatefulWrapper initialValue={EXAMPLE_DOC} onChange={onChange} />
      );

      const editor = getEditorRef();
      expect(editor.session.getMode()['$id']).toBe('ace/mode/yaml');

      // Check that the initial document is rendered
      await waitForMockToBeCalled(onChange, 1);
      expect(onChange).toHaveBeenLastCalledWith(EXAMPLE_DOC);
      expect(editor.getCursorPosition()).toStrictEqual({ row: 0, column: 0 });

      // Insert some text
      editor.moveCursorTo(5, 13); // After "cranberry", zero-indexed
      editor.insert('\n');
      editor.insert('- durian');
      await waitForMockToBeCalled(onChange, 2);
      expect(onChange).toHaveBeenLastCalledWith(
        EXAMPLE_DOC.replace('cranberry', 'cranberry\n  - durian')
      );
      expect(editor.getCursorPosition()).toStrictEqual({ row: 6, column: 10 });

      // Comment a line
      editor.selection.setRange({
        start: { row: 6, column: 0 },
        end: { row: 6, column: 10 },
      });
      editor.toggleCommentLines();
      await waitForMockToBeCalled(onChange, 3);
      expect(onChange).toHaveBeenLastCalledWith(
        EXAMPLE_DOC.replace('cranberry', 'cranberry\n  # - durian')
      );
      expect(editor.getCursorPosition()).toStrictEqual({ row: 6, column: 12 });

      // Undo the previous change
      editor.undo();
      await waitForMockToBeCalled(onChange, 4);
      expect(onChange).toHaveBeenLastCalledWith(
        EXAMPLE_DOC.replace('cranberry', 'cranberry\n  - durian')
      );
      expect(editor.getCursorPosition()).toStrictEqual({ row: 6, column: 2 });

      // Select the whole document and clear all text
      editor.selectAll();
      editor.remove();
      await waitForMockToBeCalled(onChange, 5);
      expect(onChange).toHaveBeenLastCalledWith('');
      expect(editor.getCursorPosition()).toStrictEqual({ row: 0, column: 0 });

      // Paste the original example doc
      editor.insert(EXAMPLE_DOC, true);
      await waitForMockToBeCalled(onChange, 6);
      expect(onChange).toHaveBeenLastCalledWith(EXAMPLE_DOC);
      expect(editor.getCursorPosition()).toStrictEqual({ row: 6, column: 0 });
    });
  });

  test('new content', async () => {
    const onChange = jest.fn();

    const { rerender } = render(
      <TextEditor
        data={[
          {
            content: '',
            type: 'yaml',
          },
        ]}
        onChange={onChange}
      />
    );

    const editor = getEditorRef();

    editor.insert('# hello');
    await waitForMockToBeCalled(onChange, 1);
    expect(onChange).toHaveBeenLastCalledWith('# hello');

    editor.undo();
    await waitForMockToBeCalled(onChange, 2);
    expect(onChange).toHaveBeenLastCalledWith('');

    rerender(
      <TextEditor
        data={[
          {
            content: '# world',
            type: 'yaml',
          },
        ]}
        onChange={onChange}
      />
    );
    await waitForMockToBeCalled(onChange, 3);
    expect(onChange).toHaveBeenLastCalledWith('# world');

    // Ensure an undo doesn't revert the new content change
    editor.undo();
    await waitForMockToBeCalled(onChange, 3);
    expect(onChange).toHaveBeenLastCalledWith('# world');
  });

  test('readonly', async () => {
    const onChange = jest.fn();

    const { rerender } = render(
      <TextEditor
        data={[
          {
            content: EXAMPLE_DOC,
            type: 'yaml',
          },
        ]}
        onChange={onChange}
        readOnly
      />
    );

    const editor = getEditorRef();
    expect(editor.getReadOnly()).toBeTruthy();
    expect(editor.getValue()).toBe(EXAMPLE_DOC);

    // Re-render the component with new content
    rerender(
      <TextEditor
        data={[
          {
            content: '# empty',
            type: 'yaml',
          },
        ]}
        onChange={onChange}
        readOnly
      />
    );
    await waitForMockToBeCalled(onChange, 1);
    expect(onChange).toHaveBeenLastCalledWith('# empty');
    expect(editor.getValue()).toBe('# empty');
  });

  test('sessions', async () => {
    const user = userEvent.setup();

    render(<SessionWrapper />);

    const editor = getEditorRef();
    expect(editor.getValue()).toBe('session-1');

    await user.click(screen.getByRole('button', { name: 'session-2' }));
    expect(editor.getValue()).toBe('session-2');

    editor.moveCursorTo(0, 999);
    editor.insert('-edited');
    expect(editor.getValue()).toBe('session-2-edited');

    await user.click(screen.getByRole('button', { name: 'session-3' }));
    expect(editor.getValue()).toBe('session-3');

    // Check the edit is retained
    await user.click(screen.getByRole('button', { name: 'session-2' }));
    expect(editor.getValue()).toBe('session-2-edited');
  });
});

function getEditorRef() {
  const element = screen.getByTestId('text-editor');
  return window['ace'].edit(element) as ace.Editor;
}

function waitForMockToBeCalled(fn: jest.Mock, times: number) {
  return waitFor(() => expect(fn).toHaveBeenCalledTimes(times));
}

function StatefulWrapper(props: {
  initialValue: string;
  onChange?: (content: string) => void;
}) {
  const [value, setValue] = useState(props.initialValue);

  // Calls the onChange callback when the state value changes
  useEffect(() => {
    props.onChange?.(value);
  }, [props, value]);

  return (
    <TextEditor
      data={[
        {
          content: value,
          type: 'yaml',
        },
      ]}
      onChange={(content: string) => setValue(content)}
      readOnly={false}
    />
  );
}

function SessionWrapper() {
  const [activeSession, setActiveSession] = useState(0);

  const goToSession = (index: number) => {
    setActiveSession(index);
  };

  // Keep this stable to avoid resetting the content of each session on re-render
  const data = useMemo(
    () => [
      {
        content: 'session-1',
        type: 'yaml',
      },
      {
        content: 'session-2',
        type: 'yaml',
      },
      {
        content: 'session-3',
        type: 'yaml',
      },
    ],
    []
  );

  return (
    <>
      <button onClick={() => goToSession(0)}>session-1</button>
      <button onClick={() => goToSession(1)}>session-2</button>
      <button onClick={() => goToSession(2)}>session-3</button>
      <TextEditor data={data} activeIndex={activeSession} />
    </>
  );
}

const EXAMPLE_DOC = `# this is a comment
title: "List of fruit"
items:
  - apple
  - banana
  - cranberry
`;
