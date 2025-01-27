/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import ace from 'ace-builds/src-min-noconflict/ace';
import { Component } from 'react';
import styled from 'styled-components';

import 'ace-builds/src-noconflict/mode-json';
import 'ace-builds/src-noconflict/mode-yaml';
import 'ace-builds/src-noconflict/ext-searchbox';

import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { Copy, Download } from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import { downloadObject } from 'shared/utils/download';

import StyledTextEditor from './StyledTextEditor';

const { UndoManager } = ace.require('ace/undomanager');

class TextEditor extends Component {
  onChange = () => {
    const isClean = this.editor.session.getUndoManager().isClean();
    if (this.props.onDirty) {
      this.props.onDirty(!isClean);
    }

    const content = this.editor.session.getValue();
    if (this.props.onChange) {
      this.props.onChange(content);
    }
  };

  getData() {
    return this.sessions.map(s => s.getValue());
  }

  componentDidUpdate(prevProps) {
    if (prevProps.activeIndex !== this.props.activeIndex) {
      this.setActiveSession(this.props.activeIndex);
    }
    if (prevProps.readOnly !== this.props.readOnly) {
      this.editor.setReadOnly(this.props.readOnly);
    }

    this.editor.resize();
  }

  createSession({ content, type, tabSize = 2 }) {
    const mode = getMode(type);
    let session = new ace.EditSession(content);
    let undoManager = new UndoManager();
    undoManager.markClean();
    session.setUndoManager(undoManager);
    session.setUseWrapMode(false);
    session.setOptions({ tabSize, useSoftTabs: true, useWorker: false });
    session.setMode(mode);
    return session;
  }

  setActiveSession(index) {
    let activeSession = this.sessions[index];
    if (!activeSession) {
      activeSession = this.createSession({ content: '' });
    }

    this.editor.setSession(activeSession);
    if (!this.props.readOnly) {
      this.editor.focus();
    }
  }

  initSessions(data = []) {
    this.isDirty = false;
    this.sessions = data.map(item => this.createSession(item));
    this.setActiveSession(0);
  }

  componentDidMount() {
    const { data, readOnly } = this.props;
    this.editor = ace.edit(this.ace_viewer);
    this.editor.setFadeFoldWidgets(true);
    this.editor.setWrapBehavioursEnabled(true);
    this.editor.setHighlightActiveLine(false);
    this.editor.setShowInvisibles(false);
    this.editor.renderer.setShowGutter(false);
    this.editor.renderer.setShowPrintMargin(false);
    this.editor.renderer.setShowGutter(true);
    this.editor.on('input', this.onChange);
    this.editor.setReadOnly(readOnly);
    this.editor.setTheme({ cssClass: 'ace-teleport' });
    this.initSessions(data);

    if (!readOnly) {
      this.editor.focus();
    }
  }

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.session = null;
  }

  render() {
    const { bg = 'levels.sunken' } = this.props;
    const hasButton = this.props.copyButton || this.props.downloadButton;

    return (
      <StyledTextEditor bg={bg}>
        <div ref={e => (this.ace_viewer = e)} />
        {hasButton && (
          <ButtonSection>
            {this.props.copyButton && (
              <EditorButton
                title="Copy to clipboard"
                onClick={() => copyToClipboard(this.editor.session.getValue())}
              >
                <Copy size="medium" />
              </EditorButton>
            )}
            {this.props.downloadButton && (
              <EditorButton
                title="Download"
                onClick={() =>
                  downloadObject(
                    this.props.downloadFileName,
                    this.editor.session.getValue()
                  )
                }
              >
                <Download size="medium" />
              </EditorButton>
            )}
          </ButtonSection>
        )}
      </StyledTextEditor>
    );
  }
}

function getMode(docType) {
  if (docType === 'json') {
    return 'ace/mode/json';
  }

  return 'ace/mode/yaml';
}

const EditorButton = styled(ButtonSecondary)`
  padding: ${({ theme }) => theme.space[2]}px;
  background-color: transparent;
`;

const ButtonSection = styled(Flex)`
  position: absolute;
  right: 0;
  top: 0;
  z-index: 10;
`;

export default TextEditor;
