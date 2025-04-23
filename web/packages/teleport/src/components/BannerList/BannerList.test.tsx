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

import React from 'react';

import {
  fireEvent,
  screen,
  render as testingRender,
} from 'design/utils/testing';

import { LayoutContextProvider } from 'teleport/Main/LayoutContext';

import { BannerList, type BannerType } from './BannerList';

function render(banner: React.ReactNode) {
  return testingRender(<LayoutContextProvider>{banner}</LayoutContextProvider>);
}

describe('components/BannerList/Banner', () => {
  let banners: BannerType[] = null;
  beforeEach(() => {
    banners = [
      {
        message: 'I am steve banner',
        severity: 'info',
        id: 'test-banner1',
      },
      {
        message: 'I am steve banter',
        severity: 'warning',
        id: 'test-banner2',
      },
    ];
  });

  it('renders all supplied banners', () => {
    render(<BannerList banners={banners} />);
    expect(screen.getByText(banners[0].message)).toBeInTheDocument();
    expect(screen.getByText(banners[1].message)).toBeInTheDocument();
  });

  it('hides banner when the banner close is clicked', () => {
    const dismiss = jest.fn();
    banners.pop();
    render(<BannerList banners={banners} onBannerDismiss={dismiss} />);
    expect(screen.getByText(banners[0].message)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button'));
    expect(screen.queryByText(banners[0].message)).not.toBeInTheDocument();
    expect(dismiss).toHaveBeenCalledTimes(1);
    expect(dismiss).toHaveBeenCalledWith('test-banner1');
  });

  it('does not modify the provided banner list on hide', () => {
    banners.pop();
    render(<BannerList banners={banners} />);
    expect(screen.getByText(banners[0].message)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button'));
    expect(screen.queryByText(banners[0].message)).not.toBeInTheDocument();
    expect(banners[0].hidden).toBeUndefined();
  });

  it('supports custom banners', () => {
    const customBannerMessage = 'Customized Steve Banner';
    const customBanner = [<div key="foo">{customBannerMessage}</div>];
    render(<BannerList banners={banners} customBanners={customBanner} />);
    expect(screen.getByText(banners[0].message)).toBeInTheDocument();
    expect(screen.getByText(banners[1].message)).toBeInTheDocument();
    expect(screen.getByText(customBannerMessage)).toBeInTheDocument();
  });
});
