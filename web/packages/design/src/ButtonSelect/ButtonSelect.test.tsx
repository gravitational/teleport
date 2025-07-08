/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { render, screen } from 'design/utils/testing';

import { ButtonSelect } from './ButtonSelect';

describe('ButtonSelect', () => {
  const options = [
    { value: '1', label: 'Option 1' },
    { value: '2', label: 'Option 2' },
    { value: '3', label: 'Option 3' },
  ];
  const activeValue = '1';

  function renderButtonSelect() {
    const onChange = jest.fn();
    render(
      <ButtonSelect
        options={options}
        activeValue={activeValue}
        onChange={onChange}
      />
    );
    return { buttons: screen.getAllByRole('button'), onChange };
  }

  it('renders buttons with correct labels', () => {
    const { buttons } = renderButtonSelect();
    expect(buttons).toHaveLength(options.length);
    expect(buttons[0]).toHaveTextContent('Option 1');
    expect(buttons[1]).toHaveTextContent('Option 2');
    expect(buttons[2]).toHaveTextContent('Option 3');
    expect(buttons[0]).toHaveAttribute('aria-label', 'Option 1');
    expect(buttons[1]).toHaveAttribute('aria-label', 'Option 2');
    expect(buttons[2]).toHaveAttribute('aria-label', 'Option 3');
  });

  it('applies data-active attribute correctly', () => {
    const { buttons } = renderButtonSelect();
    expect(buttons).toHaveLength(options.length);
    expect(buttons[0]).toHaveAttribute('aria-checked', 'true');
    expect(buttons[1]).toHaveAttribute('aria-checked', 'false');
    expect(buttons[2]).toHaveAttribute('aria-checked', 'false');
  });

  it('calls onChange with the correct value when a button is clicked', () => {
    const { buttons, onChange } = renderButtonSelect();
    buttons[1].click();
    expect(onChange).toHaveBeenCalledWith('2');
  });

  it('does not call onChange if the clicked button is already active', () => {
    const { buttons, onChange } = renderButtonSelect();
    buttons[0].click();
    expect(onChange).not.toHaveBeenCalled();
  });
});
