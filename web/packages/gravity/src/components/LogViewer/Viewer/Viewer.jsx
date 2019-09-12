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
import ace from 'brace';
import 'brace/mode/text';
import 'brace/theme/ambiance';
import 'brace/ext/searchbox';

export default class Viewer extends React.Component {

  shouldComponentUpdate(){
    return false;
  }

  componentDidMount() {
    this.session = this._createSession();
    this.editor = ace.edit(this.aceViewerRef);
    this.editor.renderer.setShowGutter(false);
    this.editor.renderer.setShowPrintMargin(false);
    this.editor.setWrapBehavioursEnabled(false);
    this.editor.setHighlightActiveLine(false);
    this.editor.setTheme("ace/theme/ambiance");
    this.editor.setSession(this.session);
    this.editor.setShowInvisibles(false);
    this.editor.setReadOnly(true);

    this.editor.on('focus', () => {
      if(this.props.onFocus){
        this.props.onFocus();
      }
    })

    this.editor.renderer.once('afterRender', () => {
      this.scrollToLastRow();
    });

    /*
    * Automatically scrolling cursor into view after selection change this will be
    * disabled in the next version set editor.$blockScrolling = Infinity to disable this message
    */
    this.editor.$blockScrolling = Infinity;
  }

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.session = null;
  }

  clear(){
    this.session = this._createSession();
    this.editor.setSession(this.session);
  }

  insert(text){
    let session = this.editor.getSession();
    // check if user changed the scroll bar position
    let isLastRowVisible = Math.abs(session.getScreenLength() - this.editor.getLastVisibleRow()) <= 1;


    session.insert( {
      row: session.getLength(),
      column: 0 }, text);

    if (this.props.autoScroll && isLastRowVisible) {
      this.scrollToLastRow();
    }
  }

  scrollToLastRow() {
    this.editor.navigateFileEnd();
    this.editor.renderer.scrollCursorIntoView();
  }

  render(){
    return (
      <div ref={ e => this.aceViewerRef = e } style={editorStyle}/>
    )
  }

  _createSession() {
    let session = new ace.EditSession('');
    session.setUseWrapMode(false)
    return session;
  }
}

const editorStyle =  {
  position: 'absolute',
  top: '0px',
  right: '0px',
  bottom: '0px',
  left: '0px'
};