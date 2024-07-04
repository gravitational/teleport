/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import styled from 'styled-components';
import { Flex } from 'design';
import { ShieldCheck } from 'design/Icon';
import { HoverTooltip } from 'shared/components/ToolTip';

import session from 'teleport/services/websession';

export const DeviceTrustIcon = ({ iconSize = 24 }: { iconSize?: number }) => {
  const deviceTrusted = session.getIsDeviceTrusted();

  if (!deviceTrusted) {
    return null;
  }

  return (
    <Wrapper>
      <HoverTooltip
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
        transformOrigin={{ vertical: 'top', horizontal: 'center' }}
        tipContent={'This session is authorized with Device Trust'}
        css={`
          height: 100%;
        `}
      >
        <ShieldCheck color="success.main" size={iconSize} />
      </HoverTooltip>
    </Wrapper>
  );
};

const Wrapper = styled(Flex)`
  height: 100%;
  padding-left: 8px;
`;
