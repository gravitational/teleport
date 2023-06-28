/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { fireEvent, screen, render as testingRender } from 'design/utils/testing';

import { LayoutContextProvider } from 'teleport/Main/LayoutContext';

import { BannerList } from './BannerList';

import type { BannerType } from './BannerList';

function render(banner: React.ReactNode) {
  return testingRender(
    <LayoutContextProvider>
      {banner}
    </LayoutContextProvider>
  )
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

  it('supports rendering children', () => {
    const message = 'That was easy';
    render(
      <BannerList banners={banners}>
        <button>{message}</button>
      </BannerList>
    );
    expect(screen.getByText(message)).toBeInTheDocument();
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
