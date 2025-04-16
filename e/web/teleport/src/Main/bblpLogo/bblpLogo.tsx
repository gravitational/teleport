import { Image } from 'design';

import bblpLogo from './bblpLogo.svg';

export const BblpLogo = () => {
  return (
    <Image
      src={bblpLogo}
      width="fit-content"
      alt="bblp logo"
      css={`
        padding-left: ${props => props.theme.space[3]}px;
        padding-right: ${props => props.theme.space[3]}px;
        padding-top: ${props => props.theme.space[1]}px;
        height: 18px;
        @media screen and (min-width: ${p => p.theme.breakpoints.small}) {
          height: 28px;
          padding-left: ${props => props.theme.space[4]}px;
          padding-right: ${props => props.theme.space[4]}px;
        }
        @media screen and (min-width: ${p => p.theme.breakpoints.large}) {
          height: 30px;
        }
      `}
    />
  );
};
