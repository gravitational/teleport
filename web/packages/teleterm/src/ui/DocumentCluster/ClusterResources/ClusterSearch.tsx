/**
 * Copyright 2021 Gravitational, Inc.
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
import styled from 'styled-components';
import { debounce } from 'lodash';
import { space, width, color, height } from 'styled-system';

export default function ClusterSearch(props: Props) {
  const ref = React.useRef<HTMLInputElement>();

  const handleOnChange = React.useMemo(() => {
    return debounce(() => {
      props.onChange(ref.current.value);
    }, 100);
  }, []);

  React.useEffect(() => {
    return () => handleOnChange.cancel();
  }, []);

  return <Input ref={ref} placeholder="Search..." onChange={handleOnChange} />;
}

const Input = styled.input(props => {
  const { theme } = props;
  return {
    background: theme.colors.primary.light,
    boxSizing: 'border-box',
    color: theme.colors.text.primary,
    width: '100%',
    minHeight: '30px',
    minWidth: '300px',
    border: 'none',
    outline: 'none',
    borderRadius: '4px',
    padding: '2px 12px',
    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText,
      background: theme.colors.primary.lighter,
      opacity: 1,
    },

    ...space(props),
    ...width(props),
    ...height(props),
    ...color(props),
  };
});

type Props = {
  onChange(value: string): void;
};
