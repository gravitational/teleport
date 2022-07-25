import React from 'react';
import { render, fireEvent, waitFor } from 'design/utils/testing';

import { MenuLogin } from './MenuLogin';

test('does not accept an empty value when required is set to true', async () => {
  const onSelect = jest.fn();
  const { findByText, findByPlaceholderText } = render(
    <MenuLogin
      placeholder="MenuLogin input"
      required={true}
      getLoginItems={() => []}
      onSelect={() => onSelect()}
    />
  );

  fireEvent.click(await findByText('CONNECT'));
  await waitFor(async () =>
    fireEvent.keyPress(await findByPlaceholderText('MenuLogin input'), {
      key: 'Enter',
      keyCode: 13,
    })
  );

  expect(onSelect).toHaveBeenCalledTimes(0);
});

test('accepts an empty value when required is set to false', async () => {
  const onSelect = jest.fn();
  const { findByText, findByPlaceholderText } = render(
    <MenuLogin
      placeholder="MenuLogin input"
      required={false}
      getLoginItems={() => []}
      onSelect={() => onSelect()}
    />
  );

  fireEvent.click(await findByText('CONNECT'));
  await waitFor(async () =>
    fireEvent.keyPress(await findByPlaceholderText('MenuLogin input'), {
      key: 'Enter',
      keyCode: 13,
    })
  );

  expect(onSelect).toHaveBeenCalledTimes(1);
});
