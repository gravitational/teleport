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

import { fireEvent, render, screen } from 'design/utils/testing';

import { LabelSelector } from './LabelSelector';

describe('teleport/LabelSelector', () => {
  it('clicking the Pill area opens add input', () => {
    render(<LabelSelector onChange={() => {}} />);
    expect(screen.queryByTestId('add-label-container')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('label-container'));
    expect(screen.getByTestId('add-label-container')).toBeInTheDocument();
  });

  it('shows a message when a label is valid', () => {
    render(<LabelSelector onChange={() => {}} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = screen.getByRole('textbox');
    fireEvent.change(labelInput, { target: { value: 'foo: bar' } });
    expect(screen.getByTestId('create-label-msg')).toBeInTheDocument();
    expect(screen.queryByTestId('create-label-error')).not.toBeInTheDocument();
  });

  it('allows new labels to be added and sends them to the onchange handler', () => {
    const onChange = jest.fn();
    render(<LabelSelector onChange={onChange} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = screen.getByRole('textbox');
    fireEvent.change(labelInput, { target: { value: 'foo: bar' } });
    fireEvent.keyPress(labelInput, { key: 'Enter', charCode: 13 });
    expect(onChange.mock.calls).toEqual([[[]], [['foo: bar']]]);
  });

  it('prevents invalid labels to be submitted', () => {
    const onChange = jest.fn();
    render(<LabelSelector onChange={onChange} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = screen.getByRole('textbox');
    fireEvent.change(labelInput, { target: { value: 'foo bar' } });
    fireEvent.keyPress(labelInput, { key: 'Enter', charCode: 13 });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it('shows a message when a label is invalid', () => {
    render(<LabelSelector onChange={() => {}} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = screen.getByRole('textbox');
    fireEvent.change(labelInput, { target: { value: 'foo bar' } });
    expect(screen.queryByTestId('create-label-msg')).not.toBeInTheDocument();
    expect(screen.getByTestId('create-label-error')).toBeInTheDocument();
  });
});
