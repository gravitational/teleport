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

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Box } from 'design';

import { MainContainer } from 'teleport/Main/MainContainer';

import { useLayout } from 'teleport/Main/LayoutContext';

import { Banner } from './Banner';

import type { Severity } from './Banner';
import type { ReactNode } from 'react';

export const BannerList = ({
  banners = [],
  children,
  customBanners = [],
  billingBanners = [],
  onBannerDismiss = () => {},
}: Props) => {
  const { hasDockedElement } = useLayout();

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
    <Wrapper
      hasDockedElement={hasDockedElement}
      bannerCount={
        shownBanners.length + customBanners.length + billingBanners.length
      }
    >
      {shownBanners.map(banner => (
        <Banner
          message={banner.message}
          severity={banner.severity}
          id={banner.id}
          link={banner.link}
          onClose={removeBanner}
          key={banner.id}
        />
      ))}
      {customBanners}
      {billingBanners}
      {children}
    </Wrapper>
  );
};

const Wrapper = styled(Box)<{ bannerCount: number; hasDockedElement: boolean }>`
  display: flex;
  height: 100vh;
  flex-direction: column;
  width: ${p => (p.hasDockedElement ? 'calc(100vw - 520px)' : '100vw')};

  ${MainContainer} {
    flex: 1;
    height: calc(100% - ${props => props.bannerCount * 38}px);
  }
`;

type Props = {
  banners: BannerType[];
  children?: ReactNode;
  customBanners?: ReactNode[];
  onBannerDismiss?: (string) => void;
  billingBanners?: ReactNode[];
};

export type BannerType = {
  message: string;
  severity: Severity;
  id: string;
  link?: string;
  hidden?: boolean;
};
