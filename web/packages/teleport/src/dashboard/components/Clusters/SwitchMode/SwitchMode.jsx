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
import * as Icons from 'design/Icon';
import { Flex, ButtonSecondary } from 'design';

export const ModeEnum = {
  GRID: 'grid',
  TABLE: 'table',
};

export default function Switch({ mode = ModeEnum.GRID, onChange, ...styles }) {
  const gridColor = mode === ModeEnum.GRID ? '' : 'primary.dark';
  const tableColor = mode === ModeEnum.TABLE ? '' : 'primary.dark';

  function onBtnClick() {
    onChange(mode === ModeEnum.GRID ? ModeEnum.TABLE : ModeEnum.GRID);
  }

  return (
    <Flex {...styles}>
      <ButtonSecondary
        onClick={onBtnClick}
        as={StyledButton}
        disabled={mode === ModeEnum.GRID}
        title="grid view"
      >
        <Icons.CardViewSmall color={gridColor} fontSize="20px" />
      </ButtonSecondary>
      <ButtonSecondary
        onClick={onBtnClick}
        disabled={mode === ModeEnum.TABLE}
        as={StyledButton}
        title="table view"
      >
        <Icons.ListView color={tableColor} fontSize="20px" />
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
  width: 48px;

  :nth-child(1) {
    border-bottom-right-radius: 0px;
    border-top-right-radius: 0px;
  }

  :nth-child(2) {
    border-bottom-left-radius: 0px;
    border-top-left-radius: 0px;
  }

  :disabled {
    color: inherit;
  }
`;
