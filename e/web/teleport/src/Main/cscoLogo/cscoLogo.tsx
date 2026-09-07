import { css } from 'styled-components';
import styled from 'styled-components';

import { breakpointsPx, Image } from 'design';

import cscoLogo from './cscoLogo.svg';
import cscoLogoMobile from './cscoLogoMobile.svg';

const sharedLogoStyles = css`
  width: fit-content;
  padding-left: ${props => props.theme.space[4]}px;
  padding-right: ${props => props.theme.space[4]}px;
  padding-top: 0px;
  height: 18px;
`;

const FullLogo = styled(Image)`
  ${sharedLogoStyles}
  display: none;
  @media screen and (min-width: ${breakpointsPx.small}px) {
    display: block;
  }
`;

const MobileLogo = styled(Image)`
  ${sharedLogoStyles}
  display: block;
  @media screen and (min-width: ${breakpointsPx.small}px) {
    display: none;
  }
`;

export const CscoLogo = () => (
  <>
    <MobileLogo src={cscoLogoMobile} alt="Cisco logo" />
    <FullLogo src={cscoLogo} alt="Cisco logo" />
  </>
);
