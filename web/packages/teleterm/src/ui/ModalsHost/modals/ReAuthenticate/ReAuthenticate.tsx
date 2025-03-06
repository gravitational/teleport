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

import { FC, useEffect, useState } from 'react';

import {
  Box,
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Image,
  Text,
} from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import * as icons from 'design/Icon';
import { PromptMFARequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { requiredToken } from 'shared/components/Validation/rules';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import svgHardwareKey from 'teleterm/ui/ClusterConnect/ClusterLogin/FormLogin/PromptPasswordless/hardware.svg';
import PromptSsoStatus from 'teleterm/ui/ClusterConnect/ClusterLogin/FormLogin/PromptSsoStatus';
import { LinearProgress } from 'teleterm/ui/components/LinearProgress';
import { routing } from 'teleterm/ui/uri';

export const ReAuthenticate: FC<{
  promptMfaRequest: PromptMFARequest;
  onCancel: () => void;
  onOtpSubmit: (otp: string) => void;
  onSsoContinue: (redirectUrl: string) => void;
  hidden?: boolean;
}> = props => {
  const { promptMfaRequest: req, onSsoContinue } = props;

  const availableMfaTypes = makeAvailableMfaTypes(req);

  const [selectedMfaType, setSelectedMfaType] = useState<AvailableMfaType>(
    availableMfaTypes[0]
  );

  useEffect(() => {
    // If SSO is the selected value, open the redirect window instead of waiting for the user to
    // select SSO. This handles both a situation where the user selects the SSO option and a
    // situation where SSO is already selected when the component renders.
    if (selectedMfaType.value === 'sso') {
      onSsoContinue(req.sso.redirectUrl);
    }
  }, [selectedMfaType.value, req.sso?.redirectUrl, onSsoContinue]);

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
      open={!props.hidden}
      keepInDOMAfterClose
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
              validator.validate() && props.onOtpSubmit(otpToken);
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
                <Text color="text.slightlyMuted">
                  {req.reason}
                  {isLeafCluster && ` from trusted cluster "${clusterName}"`}
                </Text>

                <Flex width="100%" gap={3} flex-wrap="no-wrap">
                  {availableMfaTypes.length > 1 && (
                    <FieldSelect
                      flex="1"
                      label="Two-factor Type"
                      value={selectedMfaType}
                      options={availableMfaTypes}
                      onChange={mfaType => {
                        setSelectedMfaType(mfaType);
                      }}
                    />
                  )}

                  {selectedMfaType.value === 'totp' ? (
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

                {selectedMfaType.value === 'webauthn' && (
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

                {selectedMfaType.value === 'sso' && <PromptSsoStatus />}
              </Flex>
            </DialogContent>

            <DialogFooter>
              <Flex gap={3}>
                {selectedMfaType.value === 'totp' && (
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

type MfaType = 'webauthn' | 'totp' | 'sso';
type AvailableMfaType = Option<MfaType, string>;

const totp = { value: 'totp' as MfaType, label: 'Authenticator App' };
const webauthn = { value: 'webauthn' as MfaType, label: 'Hardware Key' };

function makeAvailableMfaTypes(req: PromptMFARequest): AvailableMfaType[] {
  let availableMfaTypes: AvailableMfaType[] = [];

  if (req.webauthn) {
    availableMfaTypes.push(webauthn);
  }
  if (req.totp) {
    availableMfaTypes.push(totp);
  }
  // put sso last in the list so we don't automatically open the browser unless
  // sso is the only one in the list.
  if (req.sso) {
    availableMfaTypes.push({
      value: 'sso',
      label: req.sso.displayName || req.sso.connectorId,
    });
  }

  // This shouldn't happen but is technically allowed by the req data structure.
  if (availableMfaTypes.length === 0) {
    availableMfaTypes.push(webauthn);
  }
  return availableMfaTypes;
}
