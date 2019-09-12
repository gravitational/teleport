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
import ProgressBar from './ProgressBar';
import Xterm from './Xterm';
import cfg from 'gravity/config';
import { Danger } from 'design/Alert';
import { TtyPlayer } from 'gravity/lib/term/ttyPlayer';
import { Indicator, Text, Box } from 'design';

export default class Player extends React.Component {

  constructor(props) {
    super(props);

    const { sid, siteId } = this.props.match.params;
    const url = cfg.getSiteSshSessionUrl({ siteId, sid });

    this.tty = new TtyPlayer({url});
    this.state = this.calculateState();
    document.title = `Player ${sid}`;
  }

  calculateState(){
    return {
      eventCount: this.tty.getEventCount(),
      duration: this.tty.duration,
      min: 1,
      time: this.tty.getCurrentTime(),
      isLoading: this.tty.isLoading,
      isPlaying: this.tty.isPlaying,
      isError: this.tty.isError,
      errText: this.tty.errText,
      current: this.tty.current,
      canPlay: this.tty.length > 1
    };
  }

  componentDidMount() {
    this.tty.on('change', this.updateState);
    this.tty.connect();
    this.tty.play();
  }

  componentWillUnmount() {
    this.tty.stop();
    this.tty.removeAllListeners();
  }

  updateState = () => {
    const newState = this.calculateState();
    this.setState(newState);
  }

  onTogglePlayStop = () => {
    if(this.state.isPlaying){
      this.tty.stop();
    }
    else{
      this.tty.play();
    }
  }

  onMove = value => {
    this.tty.move(value);
  }

  render() {
    return (
      <Container>
        {this.renderContent()}
      </Container>
    )
  }

  renderContent() {
    const { isError, errText, isLoading, eventCount } = this.state;

    if(isError) {
      return (
        <StatusBox>
          <Danger m={10}>
            {errText || 'Error'}
          </Danger>
        </StatusBox>
      );
    }

    if(isLoading) {
      return (
        <StatusBox>
          <Indicator />
        </StatusBox>
      )
    }

    if(!isLoading && eventCount === 0 ) {
      return (
        <StatusBox>
          <Text typography="h4">
            Recording for this session is not available.
          </Text>
        </StatusBox>
      )
    }

    return (
      <>
        <Xterm p="2" tty={this.tty} />
        {this.renderProgressBar()}
      </>
    );
  }

  renderProgressBar() {
    const {
      isPlaying,
      time,
      min,
      duration,
      current,
      eventCount
    } = this.state;

    if(eventCount <= 0) {
      return null;
    }

    return (
      <ProgressBar
        isPlaying={isPlaying}
        time={time}
        min={min}
        max={duration}
        value={current}
        onToggle={this.onTogglePlayStop}
        onChange={this.onMove}/>
    );
  }
}

const StatusBox = props => (
  <Box width="100%" textAlign="center" p={3} {...props}/>
)

const Container = styled.div`
  display: flex;
  height: 100%;
  width: 100%;
  overflow: auto;
  flex-direction: column;
  flex: 1;
  justify-content: space-between;
`