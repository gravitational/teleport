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
import PropTypes from 'prop-types';
import styled from 'styled-components';
import Icon, { ListView, AddUsers, CardViewSmall } from 'design/Icon';
import { Flex, ButtonSecondary } from 'design';

export const ModeEnum = {
  VERTICAL: 'vertical',
  HORIZONTAL: 'horizontal',
  FULLSCREEN: 'fullscreen',
};

export default function Switch({
  mode = ModeEnum.VERTICAL,
  onChange,
  ...styles
}) {
  function onSetHorizontal() {
    onChange(ModeEnum.HORIZONTAL);
  }

  function onSetVertical() {
    onChange(ModeEnum.VERTICAL);
  }

  function onSetFullScreen() {
    onChange(ModeEnum.FULLSCREEN);
  }

  return (
    <Flex {...styles}>
      <ButtonSecondary
        active={mode !== ModeEnum.FULLSCREEN}
        size="small"
        onClick={onSetFullScreen}
        as={StyledButton}
        disabled={mode === ModeEnum.FULLSCREEN}
        title="TTY only"
      >
        <CardViewSmall />
      </ButtonSecondary>
      <ButtonSecondary
        active={mode !== ModeEnum.HORIZONTAL}
        size="small"
        as={StyledButton}
        disabled={mode === ModeEnum.HORIZONTAL}
        onClick={onSetHorizontal}
        title="Horizontal"
      >
        <AddUsers />
      </ButtonSecondary>
      <ButtonSecondary
        active={mode !== ModeEnum.VERTICAL}
        size="small"
        onClick={onSetVertical}
        disabled={mode === ModeEnum.VERTICAL}
        as={StyledButton}
        title="Vertical"
      >
        <ListView />
      </ButtonSecondary>
    </Flex>
  );
}

Switch.propTypes = {
  onChange: PropTypes.func.isRequired,
  mode: PropTypes.string.isRequired,
};

const StyledButton = styled.button`
  transition: none;
  padding: 0;
  width: 38px;

  ${Icon} {
    font-size: 14px;
    color: ${({ theme, active }) => {
      return active ? theme.colors.levels.sunkenSecondary : '';
    }};
  }

  :nth-child(1) {
    border-bottom-right-radius: 0px;
    border-top-right-radius: 0px;
  }

  :nth-child(2) {
    border-bottom-left-radius: 0px;
    border-bottom-right-radius: 0px;
    border-top-left-radius: 0px;
    border-top-right-radius: 0px;
    border-right: 1px solid;
    border-left: 1px solid;
    border-color: ${({ theme }) => theme.colors.levels.surfaceSecondary};
  }

  :nth-child(3) {
    border-bottom-left-radius: 0px;
    border-top-left-radius: 0px;
  }

  :disabled {
    color: inherit;
  }
`;
