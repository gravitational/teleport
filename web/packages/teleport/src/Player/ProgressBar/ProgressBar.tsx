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
import * as Icons from 'design/Icon';

import Slider from './Slider';

export default function ProgressBar(props: ProgressBarProps) {
  const Icon = props.isPlaying ? Icons.CirclePause : Icons.CirclePlay;
  return (
    <StyledProgessBar style={props.style} id={props.id}>
      <ActionButton onClick={props.toggle}>
        <Icon />
      </ActionButton>
      <PlaySpeedSelector onChange={props.onPlaySpeedChange} />
      <TimeText>{props.time}</TimeText>
      <SliderContainer>
        <Slider
          min={props.min}
          max={props.max}
          value={props.current}
          onChange={props.move}
          defaultValue={1}
          withBars
          className="grv-slider"
        />
      </SliderContainer>
      <Restart onRestart={props.onRestart} />
    </StyledProgessBar>
  );
}

function Restart(props: { onRestart?: () => void }) {
  if (!props.onRestart) {
    return null;
  }

  return (
    <ActionButton
      style={{
        marginLeft: '16px',
      }}
      onClick={props.onRestart}
    >
      <Icons.Refresh />
    </ActionButton>
  );
}

export type ProgressBarProps = {
  max: number;
  min: number;
  time: any;
  isPlaying: boolean;
  current: number;
  move: (value: any) => void;
  toggle: () => void;
  style?: React.CSSProperties;
  id?: string;
  onPlaySpeedChange?: (newSpeed: number) => void;
  onRestart?: () => void;
};

function PlaySpeedSelector(props: { onChange?: (speed: number) => void }) {
  if (!props.onChange) {
    return null;
  }

  const handleChange = (event: React.ChangeEvent<HTMLSelectElement>) => {
    props.onChange(parseFloat(event.target.value));
  };

  return (
    <PlaySpeedSelectorItem onChange={handleChange} defaultValue={'1.0'}>
      <option value="0.25">0.25x</option>
      <option value="0.5">0.5x</option>
      <option value="1.0">1.0x</option>
      <option value="2.0">2.0x</option>
      <option value="4.0">4.0x</option>
      <option value="8.0">8.0x</option>
      <option value="16.0">16.0x</option>
    </PlaySpeedSelectorItem>
  );
}

const PlaySpeedSelectorItem = styled.select`
  margin-left: 8px;
  border: none;
  background: inherit;
`;

const SliderContainer = styled.div`
  display: flex;
  flex: 1;
  flex-direction: column;
`;

const TimeText = styled.div(
  props => `
  text-align: center;
  font-family: ${props.theme.fonts.mono};
  font-size: ${props.theme.fontSizes[1]}px;
  line-height: 24px;
  width: 80px;
  opacity: 0.56;
`
);

const ActionButton = styled.button`
  background: inherit;
  border: none;
  cursor: pointer;
  font-size: 24px;
  height: 24px;
  outline: none;
  opacity: 0.87;
  padding: 0;
  text-align: center;
  transition: all 0.3s;
  width: 24px;

  &:hover {
    opacity: 1;

    .icon {
      color: ${props => props.theme.colors.success};
    }
  }

  .icon {
    height: 24px;
    width: 24px;
  }
`;

const StyledProgessBar = styled.div`
  background: ${props => props.theme.colors.levels.surface};
  display: flex;
  color: ${props => props.theme.colors.text.main};
  padding: 16px;

  .grv-slider {
    display: block;
    padding: 0;
    height: 24px;
  }

  .grv-slider .bar {
    border-radius: 200px;
    height: 8px;
    margin: 8px 0;
  }

  .grv-slider .handle {
    background-color: ${props => props.theme.colors.text.main};
    border-radius: 200px;
    box-shadow:
      0 0 4px rgba(0, 0, 0, 0.12),
      0 4px 4px rgba(0, 0, 0, 0.24);
    width: 16px;
    height: 16px;
    left: -8px;
    top: 4px;
  }

  .grv-slider .bar-0 {
    background-color: ${props => props.theme.colors.success};
    box-shadow: none;
  }

  .grv-slider .bar-1 {
    background-color: ${props => props.theme.colors.spotBackground[2]};
  }
`;
