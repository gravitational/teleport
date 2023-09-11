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
import { fireEvent, render, screen } from 'design/utils/testing';

import { Banner } from './Banner';

describe('components/BannerList/Banner', () => {
  it('displays the supplied message', () => {
    const msg = 'I am a banner';
    render(
      <Banner
        message={msg}
        severity="info"
        id="test-banner"
        onClose={() => {}}
      />
    );
    expect(screen.getByText(msg)).toBeInTheDocument();
  });

  it('renders an info banner', () => {
    const { container } = render(
      <Banner
        message="I am steve banner"
        severity="info"
        id="test-banner"
        onClose={() => {}}
      />
    );
    expect(screen.getByRole('icon')).toHaveClass('icon-info_outline');
    expect(container.firstChild).toHaveStyleRule('background-color', '#039be5');
  });

  it('renders a warning banner', () => {
    const { container } = render(
      <Banner
        message="I am steve banner"
        severity="warning"
        id="test-banner"
        onClose={() => {}}
      />
    );
    expect(screen.getByRole('icon')).toHaveClass('icon-info_outline');
    expect(container.firstChild).toHaveStyleRule('background-color', '#FFAB00');
  });

  it('renders a danger banner', () => {
    const { container } = render(
      <Banner
        message="I am steve banner"
        severity="danger"
        id="test-banner"
        onClose={() => {}}
      />
    );
    expect(screen.getByRole('icon')).toHaveClass('icon-warning');
    expect(container.firstChild).toHaveStyleRule('background-color', '#FF6257');
  });

  it('calls onClose when the X is clicked', () => {
    const id = 'test-banner';
    const onClose = jest.fn();
    render(
      <Banner
        message="I am steve banner"
        severity="info"
        id={id}
        onClose={onClose}
      />
    );

    fireEvent.click(screen.getByRole('button'));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledWith(id);
  });

  describe('with link', () => {
    it('renders valid URLs as links', () => {
      const message = 'some-message-with-valid-URL';
      render(
        <Banner
          message={message}
          severity="info"
          id="some-id"
          link="https://goteleport.com/docs"
          onClose={() => {}}
        />
      );
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(screen.getByRole('link', { name: message })).toHaveAttribute(
        'href',
        'https://goteleport.com/docs'
      );
    });

    it('renders invalid URLs as text', () => {
      const message = 'some-message';
      render(
        <Banner
          message={message}
          severity="info"
          id="some-id"
          link="{https://goteleport.com/docs"
          onClose={() => {}}
        />
      );
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(
        screen.queryByRole('link', { name: message })
      ).not.toBeInTheDocument();
    });

    it('renders non-teleport URL as text', () => {
      const message = 'message';
      render(
        <Banner
          message={message}
          severity="info"
          id="some-id"
          link="https://www.google.com/"
          onClose={() => {}}
        />
      );
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(
        screen.queryByRole('link', { name: message })
      ).not.toBeInTheDocument();
    });
  });
});
