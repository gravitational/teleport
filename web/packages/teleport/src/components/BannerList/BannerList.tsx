/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect, useState, type ReactNode } from 'react';

import { Box } from 'design';

import { StandardBanner, type Severity } from './StandardBanner';

export const BannerList = ({
  banners = [],
  customBanners = [],
  billingBanners = [],
  onBannerDismiss = () => {},
}: Props) => {
  const [bannerData, setBannerData] = useState<{ [id: string]: BannerType }>(
    {}
  );

  useEffect(() => {
    const newList = {};
    banners.forEach(banner => (newList[banner.id] = { ...banner }));
    setBannerData(newList);
  }, [banners]);

  const removeBanner = id => {
    const newList = {
      ...bannerData,
      [id]: { ...bannerData[id], hidden: true },
    };
    onBannerDismiss(id);
    setBannerData(newList);
  };

  const shownBanners = Object.values(bannerData).filter(
    banner => !banner.hidden
  );

  return (
    <Box>
      {shownBanners.map(banner => (
        <StandardBanner
          message={banner.message}
          severity={banner.severity}
          id={banner.id}
          link={banner.linkDestination}
          linkText={banner.linkText}
          onDismiss={() => removeBanner(banner.id)}
          key={banner.id}
        />
      ))}
      {customBanners}
      {billingBanners}
    </Box>
  );
};

type Props = {
  banners: BannerType[];
  customBanners?: ReactNode[];
  onBannerDismiss?: (string) => void;
  billingBanners?: ReactNode[];
};

export type BannerType = {
  message: string;
  severity: Severity;
  id: string;
  linkDestination?: string;
  linkText?: string;
  hidden?: boolean;
};
