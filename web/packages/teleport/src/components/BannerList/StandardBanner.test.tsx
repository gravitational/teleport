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

import { fireEvent, render, screen, theme } from 'design/utils/testing';

import { userEventService } from 'teleport/services/userEvent';

import { StandardBanner } from './StandardBanner';

describe('StandardBanner', () => {
  it('displays the supplied message', () => {
    const msg = 'I am a banner';
    render(
      <StandardBanner
        message={msg}
        severity="info"
        id="test-banner"
        onDismiss={() => {}}
      />
    );
    expect(screen.getByText(msg)).toBeInTheDocument();
  });

  it('renders an info banner', () => {
    const { container } = render(
      <StandardBanner
        message="I am steve banner"
        severity="info"
        id="test-banner"
        onDismiss={() => {}}
      />
    );
    expect(screen.getByRole('graphics-symbol')).toHaveClass('icon-info');
    expect(container.firstChild).toHaveStyle({
      backgroundColor: theme.colors.interactive.tonal.informational[2],
    });
  });

  it('renders a warning banner', () => {
    const { container } = render(
      <StandardBanner
        message="I am steve banner"
        severity="warning"
        id="test-banner"
        onDismiss={() => {}}
      />
    );
    expect(screen.getByRole('graphics-symbol')).toHaveClass('icon-warning');
    expect(container.firstChild).toHaveStyle({
      backgroundColor: theme.colors.interactive.tonal.alert[2],
    });
  });

  it('renders a danger banner', () => {
    const { container } = render(
      <StandardBanner
        message="I am steve banner"
        severity="danger"
        id="test-banner"
        onDismiss={() => {}}
      />
    );
    expect(screen.getByRole('graphics-symbol')).toHaveClass(
      'icon-warningcircle'
    );
    expect(container.firstChild).toHaveStyle({
      backgroundColor: theme.colors.interactive.tonal.danger[2],
    });
  });

  it('calls onDismiss when the X is clicked', () => {
    const id = 'test-banner';
    const onDismiss = jest.fn();
    render(
      <StandardBanner
        message="I am steve banner"
        severity="info"
        id={id}
        onDismiss={onDismiss}
      />
    );

    fireEvent.click(screen.getByRole('button'));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  describe('with link', () => {
    it('renders valid URLs as link buttons', () => {
      const message = 'some-message-with-valid-URL';
      render(
        <StandardBanner
          message={message}
          severity="info"
          id="some-id"
          link="https://goteleport.com/docs"
          linkText="Open Docs"
          onDismiss={() => {}}
        />
      );
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Open Docs' })).toHaveAttribute(
        'href',
        'https://goteleport.com/docs'
      );
    });

    it('renders valid URLs with default link text', () => {
      const message = 'message-with-default-text';
      render(
        <StandardBanner
          message={message}
          severity="info"
          id="some-id"
          link="https://goteleport.com/docs"
          onDismiss={() => {}}
        />
      );
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Learn More' })).toHaveAttribute(
        'href',
        'https://goteleport.com/docs'
      );
    });

    it('captures click events', () => {
      jest.spyOn(userEventService, 'captureUserEvent');
      render(
        <StandardBanner
          message="some message"
          severity="info"
          id="some-id"
          link="https://goteleport.com/docs"
          onDismiss={() => {}}
        />
      );
      fireEvent.click(screen.getByRole('link', { name: 'Learn More' }));
      expect(userEventService.captureUserEvent).toHaveBeenCalledWith({
        alert: 'some-id',
        event: 'tp.ui.banner.click',
      });
    });

    it.each`
      case                     | url                               | linkText      | expected
      ${'invalid'}             | ${'{https://goteleport.com/docs'} | ${undefined}  | ${'{https://goteleport.com/docs'}
      ${'external'}            | ${'https://www.google.com'}       | ${undefined}  | ${'https://www.google.com'}
      ${'external, link text'} | ${'https://example.com'}          | ${'Find Out'} | ${'Find Out: https://example.com'}
    `(
      'renders invalid and external URLs as text: $case',
      ({ url, linkText, expected }) => {
        const message = 'some-message';
        render(
          <StandardBanner
            message={message}
            severity="info"
            id="some-id"
            link={url}
            linkText={linkText}
            onDismiss={() => {}}
          />
        );
        expect(screen.getByText(message)).toBeInTheDocument();
        expect(screen.getByText(expected)).toBeInTheDocument();
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
      }
    );
  });
});
