import { forwardRef, type RefAttributes } from 'react';

import { Image, type ImageProps } from 'design-new';
import AGPLLogoDark from 'design-new/assets/images/agpl-dark.svg';
import AGPLLogoLight from 'design-new/assets/images/agpl-light.svg';
import CommunityLogoDark from 'design-new/assets/images/community-dark.svg';
import CommunityLogoLight from 'design-new/assets/images/community-light.svg';
import EnterpriseLogoDark from 'design-new/assets/images/enterprise-dark.svg';
import EnterpriseLogoLight from 'design-new/assets/images/enterprise-light.svg';
import { useColorMode } from 'design-new/components/ui/color-mode';

import { cfg, type TeleportEdition } from '../../config';

interface LogoMap {
  light: string;
  dark: string;
}

export const logos: Record<TeleportEdition, LogoMap> = {
  oss: {
    light: AGPLLogoLight,
    dark: AGPLLogoDark,
  },
  community: {
    light: CommunityLogoLight,
    dark: CommunityLogoDark,
  },
  ent: {
    light: EnterpriseLogoLight,
    dark: EnterpriseLogoDark,
  },
};

function getLogoForEdition(
  edition: TeleportEdition,
  colorMode: 'light' | 'dark'
) {
  return logos[edition][colorMode];
}

export const LogoHero = forwardRef<
  HTMLImageElement,
  ImageProps & RefAttributes<HTMLImageElement>
>(function LogoHero(props, ref) {
  const { colorMode } = useColorMode();

  const logo = getLogoForEdition(cfg.edition, colorMode);

  return <Image src={logo} maxH="120px" maxW="200px" {...props} ref={ref} />;
});
