/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { render, screen, fireEvent } from 'design/utils/testing';

import { LabelSelector } from './LabelSelector';

describe('teleport/LabelSelector', () => {
  it('clicking the Pill area opens add input', () => {
    render(<LabelSelector onChange={() => {}} />);
    expect(screen.queryByTestId('add-label-container')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('label-container'));
    expect(screen.queryByTestId('add-label-container')).toBeInTheDocument();
  });

  it('shows a message when a label is valid', () => {
    const { container } = render(<LabelSelector onChange={() => {}} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = container.querySelector(
      'input[name=addLabel]'
    );
    fireEvent.change(labelInput, { target: { value: 'foo: bar' } });
    expect(screen.queryByTestId('create-label-msg')).toBeInTheDocument();
    expect(screen.queryByTestId('create-label-error')).not.toBeInTheDocument();
  });

  it('allows new labels to be added and sends them to the onchange handler', () => {
    const onChange = jest.fn();
    const { container } = render(<LabelSelector onChange={onChange} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = container.querySelector(
      'input[name=addLabel]'
    );
    fireEvent.change(labelInput, { target: { value: 'foo: bar' } });
    fireEvent.keyPress(labelInput, { key: 'Enter', charCode: 13 });
    expect(onChange.mock.calls).toEqual([[[]], [['foo: bar']]]);
  });

  it('prevents invalid labels to be submitted', () => {
    const onChange = jest.fn();
    const { container } = render(<LabelSelector onChange={onChange} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = container.querySelector(
      'input[name=addLabel]'
    );
    fireEvent.change(labelInput, { target: { value: 'foo bar' } });
    fireEvent.keyPress(labelInput, { key: 'Enter', charCode: 13 });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it('shows a message when a label is invalid', () => {
    const { container } = render(<LabelSelector onChange={() => {}} />);
    fireEvent.click(screen.getByTestId('label-container'));
    const labelInput: HTMLInputElement = container.querySelector(
      'input[name=addLabel]'
    );
    fireEvent.change(labelInput, { target: { value: 'foo bar' } });
    expect(screen.queryByTestId('create-label-msg')).not.toBeInTheDocument();
    expect(screen.queryByTestId('create-label-error')).toBeInTheDocument();
  });
});
