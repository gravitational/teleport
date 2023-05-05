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
import { debounce } from 'shared/utils/highbar';
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
    background: theme.colors.levels.surfaceSecondary,
    boxSizing: 'border-box',
    color: theme.colors.text.main,
    width: '100%',
    minHeight: '24px',
    minWidth: '300px',
    outline: 'none',
    borderRadius: '4px',
    border: '1px solid transparent',
    padding: '2px 12px',
    '&:hover, &:focus': {
      color: theme.colors.text.contrast,
      borderColor: theme.colors.levels.elevated,
      opacity: 1,
    },
    '::placeholder': {
      opacity: 1,
      color: theme.colors.text.slightlyMuted,
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
