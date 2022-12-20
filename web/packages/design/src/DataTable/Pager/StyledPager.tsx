import styled from 'styled-components';

import Icon from 'design/Icon';

export const StyledArrowBtn = styled.button`
  background: none;
  border: none;
  cursor: pointer;

  ${Icon} {
    font-size: 20px;
    transition: all 0.3s;
    opacity: 0.5;
  }

  &:hover,
  &:focus {
    ${Icon} {
      opacity: 1;
    }
  }

  &:disabled {
    cursor: default;
    ${Icon} {
      opacity: 0.1;
    }
  }
`;

export const StyledFetchMoreBtn = styled.button`
  color: ${props => props.theme.colors.link};
  background: none;
  text-decoration: underline;
  text-transform: none;
  outline: none;
  border: none;
  font-weight: bold;
  line-height: 0;
  font-size: 12px;

  &:hover,
  &:focus {
    cursor: pointer;
  }

  &:disabled {
    color: ${props => props.theme.colors.action.disabled};
    cursor: wait;
  }
`;
