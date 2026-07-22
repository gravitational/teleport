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

import { useState } from 'react';

import { SelectCreatable as Component, Option } from './SelectCreatable';

export default {
  title: 'Teleport/Discover/Shared/SelectCreatable',
};

const data = ['apple', 'banana', 'carrot'];
const fixedData = ['pumpkin', 'watermelon'];

export const SelectCreatableWithoutFixed = () => {
  const [fruitInputValue, setFruitInputValue] = useState('');
  const [fruits, setFruits] = useState<Option[]>(() => {
    return data.map(l => ({
      value: l,
      label: l,
    }));
  });

  function handleUserKeyDown(event) {
    if (!fruitInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setFruits([
          ...fruits,
          { value: fruitInputValue, label: fruitInputValue },
        ]);
        setFruitInputValue('');
        event.preventDefault();
    }
  }

  return (
    <Component
      inputValue={fruitInputValue}
      onInputChange={input => setFruitInputValue(input)}
      onKeyDown={handleUserKeyDown}
      placeholder="Start typing users and press enter"
      value={fruits}
      onChange={value => setFruits(value || [])}
      options={data.map(l => ({
        value: l,
        label: l,
      }))}
    />
  );
};

export const SelectCreatableWithFixed = () => {
  const [fruitInputValue, setFruitInputValue] = useState('');
  const [fruits, setFruits] = useState<Option[]>(() => {
    const fixedFruits = fixedData.map(l => ({
      value: l,
      label: l,
      isFixed: true,
    }));
    const fruits = data.map(l => ({
      value: l,
      label: l,
    }));
    return [...fixedFruits, ...fruits];
  });

  function handleUserKeyDown(event) {
    if (!fruitInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setFruits([
          ...fruits,
          { value: fruitInputValue, label: fruitInputValue },
        ]);
        setFruitInputValue('');
        event.preventDefault();
    }
  }

  return (
    <Component
      inputValue={fruitInputValue}
      onInputChange={input => setFruitInputValue(input)}
      onKeyDown={handleUserKeyDown}
      placeholder="Start typing users and press enter"
      value={fruits}
      onChange={(value, action) => {
        if (action.action === 'clear') {
          setFruits(
            fixedData.map(l => ({
              label: l,
              value: l,
              isFixed: true,
            }))
          );
        } else {
          setFruits(value || []);
        }
      }}
      options={data.map(l => ({
        value: l,
        label: l,
      }))}
    />
  );
};
