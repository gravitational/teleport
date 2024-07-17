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

import { FC, useState } from 'react';

import { PromptMFARequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import {
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  Text,
  Image,
  Flex,
  Box,
  H2,
} from 'design';
import * as icons from 'design/Icon';
import Validation from 'shared/components/Validation';
import { requiredToken } from 'shared/components/Validation/rules';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';

import { Option } from 'shared/components/Select';

import { assertUnreachable } from 'shared/utils/assertUnreachable';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import LinearProgress from 'teleterm/ui/components/LinearProgress';
import svgHardwareKey from 'teleterm/ui/ClusterConnect/ClusterLogin/FormLogin/PromptWebauthn/hardware.svg';
import { useLogger } from 'teleterm/ui/hooks/useLogger';
import { routing } from 'teleterm/ui/uri';

type MfaType = 'webauthn' | 'totp';

export const ReAuthenticate: FC<{
  promptMfaRequest: PromptMFARequest;
  onCancel: () => void;
  onSuccess: (otp: string) => void;
}> = props => {
  const logger = useLogger('ReAuthenticate');
  const { promptMfaRequest: req } = props;

  // TODO(ravicious): At the moment it doesn't seem like it's possible for both Webauthn and TOTP to
  // be available at the same time (see lib/client/mfa.PromptConfig/GetRunOptions). Whenever both
  // Webauthn and TOTP are supported, Webauthn is preferred. Well, unless AllowStdinHijack is
  // specified, but lib/teleterm doesn't do this and AllowStdinHijack has a scary comment next to it
  // telling you not to use it.
  //
  // Alas, the data structure certainly allows for this so the modal was designed with supporting
  // such scenario in mind.
  const availableMfaTypes: MfaType[] = [];
  // Add Webauthn first to prioritize it if both Webauthn and TOTP are available.
  if (req.webauthn) {
    availableMfaTypes.push('webauthn');
  }
  if (req.totp) {
    availableMfaTypes.push('totp');
  }
  if (availableMfaTypes.length === 0) {
    // This shouldn't happen but is technically allowed by the req data structure.
    logger.warn('availableMfaTypes is empty, defaulting to webauthn and totp');
    availableMfaTypes.push('webauthn', 'totp');
  }

  const [selectedMfaType, setSelectedMfaType] = useState(availableMfaTypes[0]);
  const [otpToken, setOtpToken] = useState('');

  const { clusterUri } = req;
  const { clustersService } = useAppContext();
  // TODO(ravicious): Use a profile name here from the URI and remove the dependency on
  // clustersService. https://github.com/gravitational/teleport/issues/33733
  const rootClusterUri = routing.ensureRootClusterUri(clusterUri);
  const rootClusterName =
    clustersService.findRootClusterByResource(rootClusterUri)?.name ||
    routing.parseClusterName(rootClusterUri);
  const clusterName =
    clustersService.findCluster(clusterUri)?.name ||
    routing.parseClusterName(clusterUri);
  const isLeafCluster = routing.isLeafCluster(clusterUri);

  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <Validation>
        {({ validator }) => (
          <form
            onSubmit={e => {
              e.preventDefault();
              validator.validate() && props.onSuccess(otpToken);
            }}
          >
            <DialogHeader
              justifyContent="space-between"
              mb={0}
              alignItems="baseline"
            >
              <H2 mb={4}>
                Verify your identity on <strong>{rootClusterName}</strong>
              </H2>
              <ButtonIcon
                type="button"
                onClick={props.onCancel}
                color="text.slightlyMuted"
              >
                <icons.Cross size="medium" />
              </ButtonIcon>
            </DialogHeader>

            <DialogContent mb={4}>
              <Flex flexDirection="column" gap={4} alignItems="flex-start">
                <Text typography="body1" color="text.slightlyMuted">
                  {req.reason}
                  {isLeafCluster && ` from trusted cluster "${clusterName}"`}
                </Text>

                <Flex width="100%" gap={3} flex-wrap="no-wrap">
                  {availableMfaTypes.length > 1 && (
                    <FieldSelect
                      flex="1"
                      label="Two-factor Type"
                      value={mfaTypeToOption(selectedMfaType)}
                      options={availableMfaTypes.map(mfaTypeToOption)}
                      onChange={option =>
                        setSelectedMfaType(
                          (option as Option<string, string>).value as MfaType
                        )
                      }
                      menuIsOpen={true}
                    />
                  )}

                  {selectedMfaType === 'totp' ? (
                    <FieldInput
                      flex="1"
                      autoFocus
                      label="Authenticator Code"
                      rule={requiredToken}
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      value={otpToken}
                      onChange={e => setOtpToken(e.target.value)}
                      placeholder="123 456"
                      mb={0}
                    />
                  ) : (
                    // Empty box to occupy hald of flex width if TOTP input is not shown.
                    <Box flex="1" />
                  )}
                </Flex>

                {selectedMfaType === 'webauthn' && (
                  <>
                    <Image width="200px" src={svgHardwareKey} mx="auto" />
                    <Box
                      width="100%"
                      textAlign="center"
                      style={{ position: 'relative' }}
                    >
                      <Text bold>Insert your security key and tap it</Text>
                      <LinearProgress />
                    </Box>
                  </>
                )}
              </Flex>
            </DialogContent>

            <DialogFooter>
              <Flex gap={3}>
                {selectedMfaType === 'totp' && (
                  <ButtonPrimary type="submit">Continue</ButtonPrimary>
                )}
                <ButtonSecondary type="button" onClick={props.onCancel}>
                  Cancel
                </ButtonSecondary>
              </Flex>
            </DialogFooter>
          </form>
        )}
      </Validation>
    </DialogConfirmation>
  );
};

const mfaTypeToOption = (mfaType: MfaType): Option<string, string> => {
  let label: string;

  switch (mfaType) {
    case 'webauthn':
      label = 'Hardware Key';
      break;
    case 'totp':
      label = 'Authenticator App';
      break;
    default:
      assertUnreachable(mfaType);
  }

  return { value: mfaType, label };
};
