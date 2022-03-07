import styled from 'styled-components';

export const ListItem = styled.li`
  white-space: nowrap;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  cursor: pointer;
  width: 100%;
  position: relative;
  font-size: 14px;
  padding: 0 16px;
  font-weight: ${props => props.theme.regular};
  font-family: ${props => props.theme.font};
  color: ${props => props.theme.colors.text.primary};
  height: 36px;
  background: inherit;
  border: none;
  border-radius: 4px;

  background: ${props =>
    props.isFocused ? props.theme.colors.primary.light : null};

  &:hover {
    background: ${props => props.theme.colors.primary.light};
  }

  &:focus,
  &:hover {
    color: ${props => props.theme.colors.primary.contrastText};
  }
`;
