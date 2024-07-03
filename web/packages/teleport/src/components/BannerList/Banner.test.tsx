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
    expect(screen.getByRole('icon')).toHaveClass('icon-info');
    expect(container.firstChild).toHaveStyle({ 'background-color': '#039be5' });
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
    expect(screen.getByRole('icon')).toHaveClass('icon-info');
    expect(container.firstChild).toHaveStyle({ 'background-color': '#FFAB00' });
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
    expect(container.firstChild).toHaveStyle({ 'background-color': '#FF6257' });
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
