import { Image } from 'design';

import bblpLogo from './bblpLogo.svg';

export const BblpLogo = () => {
  return (
    <Image
      src={bblpLogo}
      width="fit-content"
      alt="bblp logo"
      css={`
        padding-left: ${props => props.theme.space[4]}px;
        height: 26px;
        @media screen and (min-width: ${p => p.theme.breakpoints.small}) {
          height: 28px;
        }
        @media screen and (min-width: ${p => p.theme.breakpoints.large}) {
          height: 30px;
        }
      `}
    />
  );
};
