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
import ace from 'ace-builds/src-min-noconflict/ace';

import 'ace-builds/src-noconflict/mode-json';
import 'ace-builds/src-noconflict/mode-yaml';
import 'ace-builds/src-noconflict/ext-searchbox';
import StyledTextEditor from './StyledTextEditor';

const { UndoManager } = ace.require('ace/undomanager');

class TextEditor extends React.Component {
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
    this.editor.focus();
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
    this.editor.focus();
  }

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.session = null;
  }

  render() {
    return (
      <StyledTextEditor>
        <div ref={e => (this.ace_viewer = e)} />
      </StyledTextEditor>
    );
  }
}

export default TextEditor;

function getMode(docType) {
  if (docType === 'json') {
    return 'ace/mode/json';
  }

  return 'ace/mode/yaml';
}
