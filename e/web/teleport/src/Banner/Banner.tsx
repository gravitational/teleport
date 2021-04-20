import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import { StyledMain } from 'teleport/Main';
import useTeleport from 'e-teleport/useTeleportE';
import SwitchBack from './Switchback';

export const Banner: React.FC = ({ children }) => {
  const ctx = useTeleport();

  // Don't display banner.
  if (!ctx.storeAccessRequests.getSessionExpiry()) {
    return <>{children}</>;
  }

  return (
    <BannerWrapper>
      <SwitchBack data-testid="banner" />
      {children}
    </BannerWrapper>
  );
};
export default Banner;

const BannerWrapper = styled(Box)`
  ${StyledMain} {
    height: calc(100% - 48px);
  }
  min-width: 1000px;
`;
