/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';

import { render, screen, fireEvent } from 'design/utils/testing';

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
  fireEvent.mouseDown(document);
  expect(screen.queryByText('hello world')).not.toBeInTheDocument();
});

const ExampleUseTestBox = () => {
  const [open, setOpen] = useState(false);
  const ref = useRefClickOutside<HTMLDivElement>({ open, setOpen });

  return (
    <>
      <div onClick={() => setOpen(true)}>click me</div>
      <div ref={ref}>{open && <div>hello world</div>}</div>
    </>
  );
};
