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

import React, { useState } from 'react';

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
