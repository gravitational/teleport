import React from 'react';
import styled from 'styled-components';
import * as Icon from 'design/Icon';

export function PasskeyIcons() {
  return (
    <>
      <OverlappingChip>
        <Icon.FingerprintSimple p={2} />
      </OverlappingChip>
      <OverlappingChip>
        <Icon.UsbDrive p={2} />
      </OverlappingChip>
      <OverlappingChip>
        <Icon.UserFocus p={2} />
      </OverlappingChip>
      <OverlappingChip>
        <Icon.DeviceMobileCamera p={2} />
      </OverlappingChip>
    </>
  );
}

const OverlappingChip = styled.span`
  display: inline-block;
  background: ${props => props.theme.colors.levels.surface};
  border: ${props => props.theme.borders[1]};
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 50%;
  margin-right: -6px;
`;
