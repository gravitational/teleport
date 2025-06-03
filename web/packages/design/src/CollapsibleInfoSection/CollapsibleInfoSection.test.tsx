/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { fireEvent, render, screen, userEvent } from 'design/utils/testing';

import { CollapsibleInfoSection } from './CollapsibleInfoSection';

describe('CollapsibleInfoSection', () => {
  test('renders with default props, closed by default', () => {
    render(<CollapsibleInfoSection>Some content</CollapsibleInfoSection>);

    expect(screen.getByText('More info')).toBeInTheDocument();
    expect(screen.queryByText('Some content')).not.toBeInTheDocument();
  });

  test('opens when the toggle is clicked, and closes again when clicked again', async () => {
    render(<CollapsibleInfoSection>Some content</CollapsibleInfoSection>);

    await userEvent.click(screen.getByRole('button', { name: /More info/i }));
    expect(screen.getByText('Less info')).toBeInTheDocument();
    expect(screen.getByText('Some content')).toBeInTheDocument();

    const button = screen.getByRole('button', { name: /Less info/i });
    await userEvent.click(button);

    // In a real browser, `onTransitionEnd` would eventually be fired, hiding the content.
    // Since we're testing w/ JSDOM, let's simulate:
    fireEvent.transitionEnd(button.nextElementSibling!);

    expect(screen.getByText('More info')).toBeInTheDocument();
    expect(screen.queryByText('Some content')).not.toBeInTheDocument();
  });

  test('can be open by default using defaultOpen prop', () => {
    render(
      <CollapsibleInfoSection defaultOpen>
        This starts out visible
      </CollapsibleInfoSection>
    );

    expect(screen.getByText('Less info')).toBeInTheDocument();
    expect(screen.getByText('This starts out visible')).toBeInTheDocument();
  });

  test('allows custom open and close labels', async () => {
    render(
      <CollapsibleInfoSection
        openLabel="Show details"
        closeLabel="Hide details"
      >
        Custom labels
      </CollapsibleInfoSection>
    );

    expect(screen.getByText('Show details')).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole('button', { name: /Show details/i })
    );
    expect(screen.getByText('Hide details')).toBeInTheDocument();
    expect(screen.getByText('Custom labels')).toBeInTheDocument();
  });

  test('calls onClick callback with isOpen toggle state', async () => {
    const handleClick = jest.fn();
    render(
      <CollapsibleInfoSection onClick={handleClick}>
        Content
      </CollapsibleInfoSection>
    );

    await userEvent.click(screen.getByRole('button', { name: /More info/i }));
    expect(handleClick).toHaveBeenCalledWith(true);

    await userEvent.click(screen.getByRole('button', { name: /Less info/i }));
    expect(handleClick).toHaveBeenCalledWith(false);
  });

  test('does not toggle when disabled', async () => {
    render(
      <CollapsibleInfoSection disabled>Disabled content</CollapsibleInfoSection>
    );

    await userEvent.click(screen.getByRole('button', { name: /More info/i }));

    expect(screen.getByText('More info')).toBeInTheDocument();
    expect(screen.queryByText('Disabled content')).not.toBeInTheDocument();
  });
});
