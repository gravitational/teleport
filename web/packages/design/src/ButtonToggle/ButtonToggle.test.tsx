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

import { render, screen, userEvent } from 'design/utils/testing';

import { ButtonToggle } from './ButtonToggle';

// needed to check the intent attribute on the button
jest.mock('../Button/Button', () => ({
  ButtonPrimary: ({
    intent,
    children,
    ...props
  }: {
    intent?: string;
    children?: React.ReactNode;
    [key: string]: any;
  }) => (
    <button data-intent={intent} {...props}>
      {children}
    </button>
  ),
}));

describe('ButtonToggle', () => {
  function getButtons() {
    return screen.getAllByRole('button');
  }

  it('renders with left button active', async () => {
    const onChange = jest.fn();
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={true}
        onChange={onChange}
      />
    );

    const [leftBtn, rightBtn] = getButtons();
    expect(leftBtn).toHaveAttribute('data-intent', 'primary');
    expect(rightBtn).toHaveAttribute('data-intent', 'neutral');
  });

  it('renders with right button active when rightIsTrue', async () => {
    const onChange = jest.fn();
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={true}
        rightIsTrue
        onChange={onChange}
      />
    );

    const [leftBtn, rightBtn] = getButtons();
    expect(leftBtn).toHaveAttribute('data-intent', 'neutral');
    expect(rightBtn).toHaveAttribute('data-intent', 'primary');
  });

  it('toggles correctly', async () => {
    const onChange = jest.fn();
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={false}
        onChange={onChange}
      />
    );
    const [leftBtn, rightBtn] = getButtons();
    expect(leftBtn).toHaveAttribute('data-intent', 'neutral');
    expect(rightBtn).toHaveAttribute('data-intent', 'primary');

    await userEvent.click(leftBtn);
    expect(onChange).toHaveBeenCalledWith(true);
    expect(leftBtn).toHaveAttribute('data-intent', 'primary');
    expect(rightBtn).toHaveAttribute('data-intent', 'neutral');

    await userEvent.click(rightBtn);
    expect(onChange).toHaveBeenCalledWith(false);
    expect(leftBtn).toHaveAttribute('data-intent', 'neutral');
    expect(rightBtn).toHaveAttribute('data-intent', 'primary');
  });

  it('toggles correctly when rightIsTrue', async () => {
    const onChange = jest.fn();
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={false}
        rightIsTrue
        onChange={onChange}
      />
    );
    const [leftBtn, rightBtn] = getButtons();
    expect(leftBtn).toHaveAttribute('data-intent', 'primary');
    expect(rightBtn).toHaveAttribute('data-intent', 'neutral');

    await userEvent.click(rightBtn);
    expect(onChange).toHaveBeenCalledWith(true);
    expect(leftBtn).toHaveAttribute('data-intent', 'neutral');
    expect(rightBtn).toHaveAttribute('data-intent', 'primary');

    await userEvent.click(leftBtn);
    expect(onChange).toHaveBeenCalledWith(false);
    expect(leftBtn).toHaveAttribute('data-intent', 'primary');
    expect(rightBtn).toHaveAttribute('data-intent', 'neutral');
  });

  it('renders labels correctly', () => {
    render(
      <ButtonToggle
        leftLabel="OFF"
        rightLabel="ON"
        initialValue={false}
        onChange={() => {}}
      />
    );

    const [leftBtn, rightBtn] = getButtons();
    expect(leftBtn).toHaveTextContent('OFF');
    expect(rightBtn).toHaveTextContent('ON');
  });

  it('does not call onChange if clicking already active button', async () => {
    const onChange = jest.fn();
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={true}
        onChange={onChange}
      />
    );
    const [leftBtn] = getButtons();
    await userEvent.click(leftBtn);
    expect(onChange).not.toHaveBeenCalled();
  });

  it('is accessible: can be focused and toggled with Space and Enter', async () => {
    const onChange = jest.fn();
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={false}
        onChange={onChange}
      />
    );
    const entireButtonToggle = screen.getByTestId('button-toggle');
    entireButtonToggle.focus();
    expect(entireButtonToggle).toHaveFocus();

    await userEvent.keyboard(' ');
    expect(onChange).toHaveBeenCalledWith(true);

    await userEvent.keyboard('{Enter}');
    expect(onChange).toHaveBeenCalledWith(false);
  });

  it('has correct aria attributes', () => {
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={true}
        onChange={() => {}}
      />
    );
    const entireButtonToggle = screen.getByTestId('button-toggle');
    expect(entireButtonToggle).toHaveAttribute('role', 'switch');
    expect(entireButtonToggle).toBeChecked();
    expect(entireButtonToggle).toHaveAttribute('tabindex', '0');
  });

  it('updates aria-checked when toggled', async () => {
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={false}
        onChange={() => {}}
      />
    );
    const entireButtonToggle = screen.getByTestId('button-toggle');
    expect(entireButtonToggle).not.toBeChecked();
    entireButtonToggle.focus();
    await userEvent.keyboard('{Enter}');
    expect(entireButtonToggle).toBeChecked();
  });

  it('updates aria-checked when toggled (with rightIsTrue)', async () => {
    render(
      <ButtonToggle
        leftLabel="Left"
        rightLabel="Right"
        initialValue={false}
        rightIsTrue
        onChange={() => {}}
      />
    );
    const entireButtonToggle = screen.getByTestId('button-toggle');
    // with rightIsTrue, initialValue=false means checked
    expect(entireButtonToggle).toBeChecked();
    entireButtonToggle.focus();
    await userEvent.keyboard('{Enter}');
    expect(entireButtonToggle).not.toBeChecked();
  });
});
