/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Component, createRef } from 'react';

import TextEditor from 'shared/components/TextEditor';

type LiveTextEditorProps = Omit<
  React.ComponentProps<typeof TextEditor>,
  'readOnly'
> & {
  data: [{ content: string; type?: 'terraform' | 'json' | 'yaml' }];
};

export default class LiveTextEditor extends Component<LiveTextEditorProps> {
  private editorRef = createRef<TextEditor>();

  componentDidUpdate(prevProps: LiveTextEditorProps) {
    const prevContent = prevProps.data?.[0]?.content;
    const currentContent = this.props.data?.[0]?.content;
    const contentChanged = prevContent !== currentContent;

    if (contentChanged && currentContent && this.editorRef.current?.editor) {
      this.editorRef.current.setActiveSession(0);
      this.editorRef.current.editor.session.setValue(currentContent);
      this.editorRef.current.editor.session.getUndoManager().markClean();
    }
  }

  render() {
    return <TextEditor ref={this.editorRef} readOnly={true} {...this.props} />;
  }
}
