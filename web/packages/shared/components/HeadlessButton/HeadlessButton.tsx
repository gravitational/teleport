/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import styled from 'styled-components';
import { forwardRef, RefObject , DetailedHTMLProps, ButtonHTMLAttributes } from "react";

export const HeadlessButton = forwardRef(
  (
    {
      className,
      ...props
    }: DetailedHTMLProps<
      ButtonHTMLAttributes<HTMLButtonElement>,
      HTMLButtonElement
    >,
    ref: RefObject<HTMLButtonElement>
  ) => {
    return (
      <Wrapper {...props} className={className} ref={ref} />
    );
  }
);

const Wrapper = styled.button`
  appearance: none;
  box-sizing: border-box;
  display: inline-block;
  min-width: 0;
  padding: 0;
  border: 0;
  font-size: inherit;
  line-height: inherit;
  background-color: transparent;
`;
