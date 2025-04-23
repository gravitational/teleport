import { Image } from 'design';

import mcLogo from './mcLogo.svg';

export const McLogo = () => {
  return (
    <Image
      src={mcLogo}
      width="fit-content"
      alt="mc logo"
      css={`
        padding-left: ${props => props.theme.space[3]}px;
        padding-right: ${props => props.theme.space[3]}px;
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
