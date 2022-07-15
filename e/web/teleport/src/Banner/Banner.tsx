import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import { StyledMain } from 'teleport/Main';
import useTeleport from 'e-teleport/useTeleportE';
import SwitchBack from './Switchback';
import { useBanner } from './useBanner';
import { LicenseWarning } from './LicenseWarning/LicenseWarning';

export const Banner: React.FC = ({ children }) => {
  const ctx = useTeleport();
  const { license } = useBanner();

  const banners: JSX.Element[] = [];

  if (license) {
    banners.push(
      <LicenseWarning
        key="license-warning-banner"
        severity={license.severity}
        text={license.text}
      />
    );
  }

  if (ctx.storeAccessRequests.getSessionExpiry()) {
    banners.push(
      <SwitchBack key="access-request-banner" data-testid="banner" />
    );
  }

  return (
    <BannersWrapper banners={banners.length}>
      {banners}
      {children}
    </BannersWrapper>
  );
};
export default Banner;

const BannersWrapper = styled(Box)`
  ${StyledMain} {
    height: calc(100% - ${props => props.banners * 48}px);
  }
  min-width: 1000px;
`;
