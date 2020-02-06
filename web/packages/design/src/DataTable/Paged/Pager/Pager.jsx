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
import Icon, {
  CircleArrowLeft,
  CircleArrowRight,
} from 'design/Icon';
import { Text } from 'design';
import PropTypes from 'prop-types';

export default function Pager(props) {
  const { startFrom = 0, endAt = 0, totalRows = 0, onPrev, onNext } = props;
  const isPrevDisabled = totalRows === 0 || startFrom === 0;
  const isNextDisabled = totalRows === 0 || endAt === totalRows;

  return (
    <>
      <Text typography="body2" color="primary.contrastText">
        SHOWING <strong>{startFrom + 1}</strong> to <strong>{endAt}</strong> of{' '}
        <strong>{totalRows}</strong>
      </Text>
      <StyledButtons>
        <button
          onClick={onPrev}
          title="Previous Page"
          disabled={isPrevDisabled}
        >
          <CircleArrowLeft fontSize="3" />
        </button>
        <button onClick={onNext} title="Next Page" disabled={isNextDisabled}>
          <CircleArrowRight fontSize="3" />
        </button>
      </StyledButtons>
    </>
  );
}

Pager.propTypes = {
  startFrom: PropTypes.number.isRequired,
  endAt: PropTypes.number.isRequired,
  totalRows: PropTypes.number.isRequired,
  onPrev: PropTypes.func.isRequired,
  onNext: PropTypes.func.isRequired
}

export const StyledButtons = styled.div`
  button {
    background: none;
    border: none;
    border-radius: 200px;
    cursor: pointer;
    height: 24px;
    padding: 0;
    margin: 0 2px;
    min-width: 24px;
    outline: none;
    transition: all 0.3s;

    &:hover {
      background: ${props => props.theme.colors.primary.main};
      ${Icon} {
        opacity: 1;
      }
    }

    ${Icon} {
      opacity: 0.56;
      transition: all 0.3s;
    }
  }
`;
