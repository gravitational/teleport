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

import { fireEvent, render, screen } from 'design/utils/testing';

import { useRefClickOutside } from './useRefClickOutside';

test('useRefClickOutside', () => {
  render(<ExampleUseTestBox />);

  expect(screen.queryByText('hello world')).not.toBeInTheDocument();

  // Open.
  fireEvent.click(screen.getByText('click me'));
  expect(screen.getByText('hello world')).toBeInTheDocument();

  // Clicking inside of element, should not close it.
  fireEvent.mouseDown(screen.getByText('hello world'));
  expect(screen.getByText('hello world')).toBeInTheDocument();

  // Clicking outside of element, should close it.
  fireEvent.mouseDown(screen.getByText('outside'));
  expect(screen.queryByText('hello world')).not.toBeInTheDocument();
});

const ExampleUseTestBox = () => {
  const [open, setOpen] = useState(false);
  const ref = useRefClickOutside<HTMLDivElement>({ open, setOpen });

  return (
    <>
      <div onClick={() => setOpen(true)}>click me</div>
      <div ref={ref}>{open && <div>hello world</div>}</div>
      <div>outside</div>
    </>
  );
};
