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

describe('ButtonToggle', () => {
  function getButtons() {
    return screen.getAllByRole('button');
  }

  it('renders buttons with correct labels', () => {
    render(
      <ButtonSelect
        options={[
          { key: '1', label: 'Option 1' },
          { key: '2', label: 'Option 2' },
          { key: '3', label: 'Option 3' },
        ]}
        activeIndex={0}
        onChange={() => {}}
      />
    );

    const buttons = getButtons();
    expect(buttons).toHaveLength(3);
    expect(buttons[0]).toHaveTextContent('Option 1');
    expect(buttons[1]).toHaveTextContent('Option 2');
    expect(buttons[2]).toHaveTextContent('Option 3');
  });

  it('selects an inactive button when it is clicked', () => {
    const onChangeMock = jest.fn();
    const options = [
      { key: '1', label: 'Option 1' },
      { key: '2', label: 'Option 2' },
    ];
    let activeIndex = 0;

    const { rerender } = render(
      <ButtonSelect
        options={options}
        activeIndex={activeIndex}
        onChange={index => {
          onChangeMock(index);
          activeIndex = index;
          rerender(
            <ButtonSelect
              options={options}
              activeIndex={activeIndex}
              onChange={onChangeMock}
            />
          );
        }}
      />
    );

    const buttons = getButtons();
    buttons[1].click();
    expect(onChangeMock).toHaveBeenCalledWith(1);
    expect(buttons[0]).toHaveAttribute('data-active', 'false');
    expect(buttons[1]).toHaveAttribute('data-active', 'true');
  });

  it('does not change active button when clicking the active button', () => {
    const onChangeMock = jest.fn();
    const options = [
      { key: '1', label: 'Option 1' },
      { key: '2', label: 'Option 2' },
    ];
    let activeIndex = 0;

    const { rerender } = render(
      <ButtonSelect
        options={options}
        activeIndex={activeIndex}
        onChange={index => {
          onChangeMock(index);
          activeIndex = index;
          rerender(
            <ButtonSelect
              options={options}
              activeIndex={activeIndex}
              onChange={onChangeMock}
            />
          );
        }}
      />
    );

    const buttons = getButtons();
    buttons[0].click();
    expect(onChangeMock).not.toHaveBeenCalled();
    expect(buttons[0]).toHaveAttribute('data-active', 'true');
  });
});
