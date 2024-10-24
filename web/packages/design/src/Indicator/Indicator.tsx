/*
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

import { useEffect, useState } from 'react';
import styled from 'styled-components';

import { IconProps } from 'design/Icon/Icon';

import { Spinner as SpinnerIcon } from '../Icon';
import { rotate360 } from '../keyframes';

const delayToMs: Record<Delay, number> = {
  none: 0,
  short: 400, // 0.4s;
  long: 600, // 0.6s;
};

type Delay = 'none' | 'short' | 'long';

interface IndicatorProps extends IconProps {
  delay?: Delay;
}

export function Indicator({ delay = 'short', ...iconProps }: IndicatorProps) {
  const [canDisplay, setCanDisplay] = useState(false);

  useEffect(() => {
    const timeout = delayToMs[delay];

    const timer = setTimeout(() => {
      setCanDisplay(true);
    }, timeout);

    return () => clearTimeout(timer);
  }, [delay]);

  if (!canDisplay) {
    return null;
  }

  return <StyledSpinner {...iconProps} data-testid="indicator" />;
}

const StyledSpinner = styled(SpinnerIcon)`
  color: ${props => props.color || props.theme.colors.spotBackground[2]};
  display: inline-block;

  svg {
    animation: ${rotate360} 1.5s infinite linear;
    ${({ size = '48px' }) => `
    height: ${size};
    width: ${size};
  `}
  }
`;
