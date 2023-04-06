import styled from 'styled-components';

export const PickerContainer = styled.div`
  display: flex;
  flex-direction: column;
  position: absolute;
  box-sizing: border-box;
  z-index: 1000;
  font-size: 12px;
  color: ${props => props.theme.colors.text.contrast};
  background: ${props => props.theme.colors.levels.surface};
  box-shadow: 8px 8px 18px rgb(0, 0, 0, 0.56);
  border-radius: ${props => props.theme.radii[2]}px;
  border: 1px solid ${props => props.theme.colors.action.hover};
  text-shadow: none;

  // Account for border.
  width: calc(100% + 2px);
  margin-top: -1px;
`;
