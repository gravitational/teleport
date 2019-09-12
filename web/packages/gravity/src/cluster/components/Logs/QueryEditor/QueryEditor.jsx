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
import { endsWith } from 'lodash';
import Suggestions from './Suggestions';
import { KeyEnum } from './enums';
import styled from 'styled-components';
import MenuFilter from './MenuFilter';

export const LABEL_POD = 'pod';
export const LABEL_CONTAINER = 'container';
const LABELS = [LABEL_POD, LABEL_CONTAINER];
const SEPARATOR = ':';

// regular expressions
const VALUE_REGEX = '[\\d\\w.-]';
const LABEL_REGEX = `\\b(${LABELS.join('|')})`;
const TRIGGER_REGEX = new RegExp(`${LABEL_REGEX}${SEPARATOR}${VALUE_REGEX}*$`);

const withSeparator = text => `${text}${SEPARATOR}`;

export class QueryEditorInput extends React.Component {

  constructor(props){
    super(props)
  }

  setRef = e => {
    this.refInput = e;
  }

  onChange = e => {
    this._onChange({
      value: e.target.value,
      curPos: this.getCursor()
    });
  }

  onClick = () => {
    this._onChange({
      ...this.props.state,
      curPos: this.getCursor()
    });
  }

  onKeyDown = e => {
    var keyCode = e.which;
    switch (keyCode) {
      case KeyEnum.RETURN:
        this.props.onEnter && this.props.onEnter(e);
        return;
      case KeyEnum.ESC:
        this.props.onEscape && this.props.onEscape(e);
        return;
      case KeyEnum.TAB:
        this.props.onTab && this.props.onTab(e);
        return;
      case KeyEnum.UP:
        this.props.onUpArrow && this.props.onUpArrow(e);
        return;
      case KeyEnum.DOWN:
        this.props.onDownArrow && this.props.onDownArrow(e);
        return;
    }
  }

  onKeyUp = e => {
    var keyCode = e.which;
    switch (keyCode) {
      // ignore enter key
      case KeyEnum.RETURN:
        return;
    }

    this._onChange({
      ...this.props.state,
      curPos: this.getCursor()
    });
  }

  calcNewState() {
    return {
      curPos: this.getCursor(),
      value: this.getValue()
    };
  }

  getValue() {
    return this.refInput.value;
  }

  getCursor(){
    if ( this.refInput.selectionEnd === this.refInput.selectionStart){
      return this.refInput.selectionEnd;
    }

    return -1;
  }

  setFocus() {
    this.refInput.focus();
  }

  setCursor(pos) {
    this.refInput.selectionEnd = pos;
    this.refInput.selectionStart = pos;
  }

  _onChange(state) {
    const { curPos, value } = this.props.state;
    if(state.curPos === curPos && state.value === value){
      return;
    }

    this.props.onChange(state);
  }

  componentDidMount() {
    this.setCursor(this.props.state.curPos);
  }

  componentDidUpdate(){
    let curPos = this.getCursor();
    if (curPos !== this.props.state.curPos) {
      this.setCursor(this.props.state.curPos);
    }
  }

  render() {
    const { value } = this.props.state;
    return (
      <input ref={this.setRef} autoFocus
        placeholder="Search..."
        value={value}
        onClick={this.onClick}
        onBlur={this.onBlur}
        onKeyDown={this.onKeyDown}
        onKeyUp={this.onKeyUp}
        onChange={this.onChange}
      />
    )
  }
}

class QueryEditor extends React.Component {

  constructor(props) {
    super(props);
    const { query = '' } = this.props;
    this.state = {
      suggestions: null,
      typeahead: null,
      inputState: {
        value: query,
        curPos: query.length
      }
    };
  }

  onChange = newInputState => {
    this.setState(this.calcState(newInputState));
  }

  onUpArrow = e => {
    this.handleArrow(e, -1);
  }

  onDownArrow = e => {
    this.handleArrow(e, 1);
  }

  onEnter = e => {
    e.preventDefault();
    e.stopPropagation();
    if (this.state.typeahead) {
      this.replaceWithSuggestion(this.state.menu.curItem);
    }else{
      this.props.onChange && this.props.onChange(this.state.inputState.value);
    }
  }

  onEscape = () => {
    this.hideSuggestions();
  }

  onBlur = () => {
    this.hideSuggestions();
  }

  onSuggestionMouseClick = index => {
    this.replaceWithSuggestion(index);
  }

  onMenuOpen = () => {
    this.hideSuggestions()
  }

  calcState(newInputState) {
    const typeahead = this.calcTypeaheadState(newInputState);
    const menu = this.calcMenuState(typeahead);
    return {
      inputState: newInputState,
      typeahead,
      menu
    }
  }

  calcMenuState(typeahead) {
    if (!typeahead) {
      return null;
    }

    const { selectedIndex, text = '' } = typeahead;
    const filtereData = filterSuggestions(this.props.suggestions, text);
    const curItem = normalizeSelectedIndex(selectedIndex, filtereData.length);
    return {
      options: filtereData,
      curItem
    }
  }

  calcTypeaheadState(inputState){
    let text = inputState.value.slice(0, inputState.curPos);
    let matchArr = TRIGGER_REGEX.exec(text);
    let start, end;
    if (matchArr !== null) {
      start = matchArr.index;
      end = start + matchArr[0].length;
      text = matchArr[0];
    }else{
      return null;
    }

    return {
      text,
      start,
      end,
      selectedIndex: 0
    };
  }

  hideSuggestions(){
    this.setState({
      typeahead: null
    })
  }

  append(prefix) {
    let value = this.state.inputState.value;
    let extraSpace = ''
    if (value && !endsWith(value, ' ')){
      extraSpace = ' ';
    }

    value = `${value}${extraSpace}${prefix}`;
    this.setState(this.calcState({
      value: value,
      curPos: value.length
    }));

    this.refInput.setFocus();
  }

  handleArrow(e, nudgeAmount) {
    if (!this.state.typeahead) {
      return;
    }

    e.preventDefault();
    e.stopPropagation();
    this.state.typeahead.selectedIndex += nudgeAmount;
    this.state.menu = this.calcMenuState(this.state.typeahead)
    this.setState({});
  }

  replaceWithSuggestion(index) {
    const { options } = this.state.menu;
    const option = options[index];
    if (!option) {
      return;
    }

    let { start, end } = this.state.typeahead;
    let value = this.state.inputState.value;
    let matchArr = new RegExp(`^(\\w|-)+(\\s)?`).exec(value.slice(end));
    if (matchArr !== null) {
      end = end + matchArr[0].length;
    }

    const textToInsert = `${option.type}${SEPARATOR}${option.text} `;
    value = value.substr(0, start) + textToInsert + value.substr(end);

    const newinputState = {
      value: value,
      curPos: start + textToInsert.length
    }

    const newState = this.calcState(newinputState)
    this.setState(newState)
    this.refInput.setFocus();
    this.props.onChange && this.props.onChange(value);
  }

  renderSuggestions() {
    if (this.state.typeahead === null) {
      return null;
    }

    const { selectedIndex, text = '' } = this.state.typeahead;
    const filtereData = filterSuggestions(this.props.suggestions, text);
    const curItem = normalizeSelectedIndex(selectedIndex, filtereData.length);

    return (
      <Suggestions
        onMouseClick={this.onSuggestionMouseClick}
        curItem={curItem}
        data={filtereData}
      />
    )
  }

  render() {
    const { inputState } = this.state;
    return (
      <StyledQueryEditor>
        <MenuFilter
          onOpen={this.onMenuOpen}
          onPod={ () => this.append(withSeparator(LABEL_POD))  }
          onContainer={ () => this.append(withSeparator(LABEL_CONTAINER)) }
        >
        </MenuFilter>
        <SuggestionBox>
          <QueryEditorInput
            ref={e => this.refInput = e}
            state={inputState}
            onChange={this.onChange}
            onEscape={this.onEscape}
            onEnter={this.onEnter}
            onUpArrow={this.onUpArrow}
            onDownArrow={this.onDownArrow}
          />
          {this.renderSuggestions()}
        </SuggestionBox>
      </StyledQueryEditor>
    );
  }
}

function normalizeSelectedIndex(selectedIndex, max) {
  let index = selectedIndex % max;
  if (index < 0) {
    index += max;
  }
  return index;
}

function filterSuggestions(data=[], query) {
  query = query.trim();
  let [ queryType, queryText ] = query.split(':');
  return data.filter(({ type, text }) => {
    return type === queryType && text.toLowerCase().startsWith(queryText.toLowerCase());
  });
}

const SuggestionBox = styled.div`
  position: relative;
  width: 100%;
  input {
    width: 100%;
  }
`
const StyledQueryEditor = styled.div`
  font-size: 14px;
  margin: 0;
  display: flex;
  width: 100%;
`

export default QueryEditor;