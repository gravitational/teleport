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

import { Box, Flex, Image, P2 } from 'design';
import DialogConfirmation, { DialogContent } from 'design/DialogConfirmation';
import { PromptHardwareKeyTouchRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

import svgHardwareKey from 'teleterm/ui/ClusterConnect/ClusterLogin/FormLogin/PromptPasswordless/hardware.svg';
import { CommandContainer } from 'teleterm/ui/components/CliCommand';
import { LinearProgress } from 'teleterm/ui/components/LinearProgress';

import { CommonHeader } from './CommonHeader';

export function Touch(props: {
  req: PromptHardwareKeyTouchRequest;
  onCancel(): void;
  hidden?: boolean;
}) {
  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '450px',
        width: '100%',
      })}
    >
      <CommonHeader
        onCancel={props.onCancel}
        proxyHostname={props.req.proxyHostname}
      />

      <DialogContent mb={4}>
        <P2>
          Touch your YubiKey to continue
          {props.req.command && <>{' with command:'}</>}
        </P2>
        <CommandContainer mt={2} mr={2} width="100%" bg="bgTerminal">
          <Box ml={2} mr={2}>{`$`}</Box>
          <span>{props.req.command}</span>
        </CommandContainer>
        <Flex
          flexDirection="column"
          gap={4}
          alignItems="center"
          css={`
            position: relative;
          `}
        >
          <Image mt={4} mb={4} width="200px" src={svgHardwareKey} />
          <LinearProgress />
        </Flex>
      </DialogContent>
    </DialogConfirmation>
  );
}
