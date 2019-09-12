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
import styled from 'styled-components';
import PropTypes from 'prop-types';
import Indicator from 'design/Indicator';
import { Danger } from 'design/Alert';
import { Flex } from 'design';
import Viewer from './Viewer';

const defaultState = {
  isLoading: false,
  isError: false
}
export default class LogViewer extends React.Component {

  static propTypes = {
    onFocus: PropTypes.func,
    autoScroll: PropTypes.bool,
    provider: PropTypes.object.isRequired
  }

  constructor(props) {
    super(props)
    this.editor = null;
    this.state = {
      ...defaultState
    }
  }

  onData = data => {
    this.viewerRef.insert(data.trim() + '\n');
  }

  onLoading = isLoading => {
    this.viewerRef.clear();
    this.setState({
      ...defaultState,
      isLoading});
  }

  onError = err => {
    this.setState(
      {
        ...defaultState,
        isError: true,
        errorText: err.message
      });
  }

  render() {
    const { onFocus, autoScroll, wrap, ...styles } = this.props;

    const providerProps = {
      onLoading: this.onLoading,
      onData: this.onData,
      onError: this.onError
    }

    const viewerProps = {
      onFocus,
      autoScroll,
      wrap,
    }

    // pass props with assigned callbacks
    const $provider = React.cloneElement(this.props.provider, providerProps);

    return (
      <Container bg="bgTerminal" color="#FFF" flex="1" {...styles}>
        <Viewer ref={ e => this.viewerRef = e } {...viewerProps} />
        {this.renderStatus()}
        {$provider}
      </Container>
    );
  }

  renderStatus() {
    const { isLoading, isError, errorText } = this.state;
    if (isError){
      return (
        <StyledStatusBox>
          <Danger width="100%" mx="2">
            {errorText}
          </Danger>
        </StyledStatusBox>
      )
    }

    if (isLoading){
      return (
        <StyledStatusBox>
          <Indicator delay="none"/>
        </StyledStatusBox>
      )
    }

    return null
  }
}

const Container = styled(Flex)`
  overflow: auto;

  // ace requires its parent to have relative position
  position: relative;

  .ace-ambiance {
    background-color: ${ props => props.theme.colors.bgTerminal};
  }

  .ace_scrollbar::-webkit-scrollbar-track {
    background: none !important;
  }

  .ace_editor {
    font-size: 12px;
    font-family: ${ props => props.theme.fonts.mono};
  }
`

const StyledStatusBox = styled.div`
  position: absolute;
  align-items: center;
  display: flex;
  height: 100px;
  width: 100%;
  justify-content: center;
`