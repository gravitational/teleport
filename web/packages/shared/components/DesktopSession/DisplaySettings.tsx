/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Flex, H2, Text, Toggle } from 'design';
import { Monitor } from 'design/Icon';
import { MenuIcon } from 'shared/components/MenuAction';

interface DisplaySettingsProps {
  hiDpiEnabled: boolean;
  onToggleHiDpi: () => void;
  screenIsHiDpi: boolean;
  hiDpiSupported: boolean;
}

export function DisplaySettings({
  hiDpiEnabled,
  onToggleHiDpi,
  screenIsHiDpi,
  hiDpiSupported,
}: DisplaySettingsProps) {
  return (
    <MenuIcon Icon={Monitor} tooltip="Display Settings">
      <Container>
        <Flex
          gap={2}
          flexDirection="column"
          onClick={e => {
            // Stop the menu from closing when clicking inside the settings container.
            e.stopPropagation();
          }}
        >
          <H2 mb={2}>Display Settings</H2>

          <Toggle
            isToggled={hiDpiEnabled}
            onToggle={onToggleHiDpi}
            disabled={!hiDpiSupported}
          >
            <Text ml={2}>Optimize for Retina displays</Text>
          </Toggle>

          <Text>
            Enabling this option will make the session look sharper on
            high-resolution displays, but it may increase CPU usage and reduce
            performance on some systems.
          </Text>

          {!hiDpiSupported ? (
            <Text color="text.muted" fontSize="small">
              HiDPI is not supported for this session. The version of Teleport
              running on the server may be too old.
            </Text>
          ) : (
            !screenIsHiDpi && (
              <Text color="text.muted" fontSize="small">
                Your screen does not support HiDPI. This option is only
                effective on HiDPI screens.
              </Text>
            )
          )}

          <Text color="text.muted" fontSize="small">
            Only recommended for connections to Windows 10, Windows Server 2016,
            and later. This setting will be persisted for future sessions to the
            same host.
          </Text>
        </Flex>
      </Container>
    </MenuIcon>
  );
}

const Container = styled.div`
  background: ${props => props.theme.colors.levels.elevated};
  padding: ${props => props.theme.space[4]}px;
  width: 370px;
`;
