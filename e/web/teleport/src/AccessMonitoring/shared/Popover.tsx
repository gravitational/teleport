import styled from 'styled-components';

export const Popover = styled.div`
  position: absolute;
  z-index: 2;
  background: ${p => p.theme.colors.levels.popout};
  border-radius: ${p => p.theme.radii[3]}px;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);

  &:before {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border-style: solid;
  }

  &:after {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border-style: solid;
  }
`;

export const PopoverHeader = styled.div`
  font-family: ${p => p.theme.fonts.mono};
  text-transform: uppercase;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: ${p => p.theme.fontSizes[1]}px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;
