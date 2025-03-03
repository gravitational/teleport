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

import { memo, useId } from 'react';
import { components, OptionProps } from 'react-select';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import Input from 'design/Input';
import LabelInput from 'design/LabelInput';
import { RadioGroup } from 'design/RadioGroup';
import Text, { H4 } from 'design/Text';
import Select from 'shared/components/Select';

import { SectionProps } from './sections';
import {
  createDBUserModeOptions,
  createHostUserModeOptions,
  OptionsModel,
  requireMFATypeOptions,
  sessionRecordingModeOptions,
  SSHPortForwardingModeOption,
  sshPortForwardingModeOptions,
} from './standardmodel';

/**
 * Options tab. This component is memoized to optimize performance; make sure
 * that the properties don't change unless necessary.
 */
export const Options = memo(function Options({
  value,
  isProcessing,
  onChange,
}: SectionProps<OptionsModel, never>) {
  const theme = useTheme();
  const id = useId();
  const maxSessionTTLId = `${id}-max-session-ttl`;
  const clientIdleTimeoutId = `${id}-client-idle-timeout`;
  const requireMFATypeId = `${id}-require-mfa-type`;
  const createHostUserModeId = `${id}-create-host-user-mode`;
  const createDBUserModeId = `${id}-create-db-user-mode`;
  const defaultSessionRecordingModeId = `${id}-default-session-recording-mode`;
  const sshSessionRecordingModeId = `${id}-ssh-session-recording-mode`;
  const sshPortForwardingModeId = `${id}-ssh-port-forwarding-mode`;

  return (
    <OptionsGridContainer
      border={1}
      borderColor={theme.colors.interactive.tonal.neutral[0]}
      borderRadius={3}
      p={3}
    >
      <OptionsHeader>Global Settings</OptionsHeader>

      <OptionLabel htmlFor={maxSessionTTLId}>Max Session TTL</OptionLabel>
      <Input
        id={maxSessionTTLId}
        value={value.maxSessionTTL}
        disabled={isProcessing}
        onChange={e => onChange({ ...value, maxSessionTTL: e.target.value })}
      />

      <OptionLabel htmlFor={clientIdleTimeoutId}>
        Client Idle Timeout
      </OptionLabel>
      <Input
        id={clientIdleTimeoutId}
        value={value.clientIdleTimeout}
        disabled={isProcessing}
        onChange={e =>
          onChange({ ...value, clientIdleTimeout: e.target.value })
        }
      />

      <Box>Disconnect When Certificate Expires</Box>
      <BoolRadioGroup
        name="disconnect-expired-cert"
        value={value.disconnectExpiredCert}
        onChange={d => onChange({ ...value, disconnectExpiredCert: d })}
      />

      <OptionLabel htmlFor={requireMFATypeId}>Require Session MFA</OptionLabel>
      <Select
        inputId={requireMFATypeId}
        isDisabled={isProcessing}
        options={requireMFATypeOptions}
        value={value.requireMFAType}
        onChange={t => onChange?.({ ...value, requireMFAType: t })}
      />

      <OptionLabel htmlFor={defaultSessionRecordingModeId}>
        Default Session Recording Mode
      </OptionLabel>
      <Select
        inputId={defaultSessionRecordingModeId}
        isDisabled={isProcessing}
        options={sessionRecordingModeOptions}
        value={value.defaultSessionRecordingMode}
        onChange={m => onChange?.({ ...value, defaultSessionRecordingMode: m })}
      />

      <OptionsHeader separator>SSH</OptionsHeader>

      <OptionLabel htmlFor={createHostUserModeId}>
        Create Host User Mode
      </OptionLabel>
      <Select
        inputId={createHostUserModeId}
        isDisabled={isProcessing}
        options={createHostUserModeOptions}
        value={value.createHostUserMode}
        onChange={m => onChange?.({ ...value, createHostUserMode: m })}
      />

      <Box>Agent Forwarding</Box>
      <BoolRadioGroup
        name="forward-agent"
        value={value.forwardAgent}
        onChange={f => onChange({ ...value, forwardAgent: f })}
      />

      <OptionLabel htmlFor={sshSessionRecordingModeId}>
        Session Recording Mode
      </OptionLabel>
      <Select
        inputId={sshSessionRecordingModeId}
        isDisabled={isProcessing}
        options={sessionRecordingModeOptions}
        value={value.sshSessionRecordingMode}
        onChange={m => onChange?.({ ...value, sshSessionRecordingMode: m })}
      />

      <OptionLabel htmlFor={sshPortForwardingModeId}>
        Port Forwarding Mode
      </OptionLabel>
      <Select
        components={sshPortForwardingModeComponents}
        inputId={sshPortForwardingModeId}
        isDisabled={isProcessing}
        options={sshPortForwardingModeOptions}
        value={value.sshPortForwardingMode}
        onChange={m => onChange?.({ ...value, sshPortForwardingMode: m })}
      />

      <OptionsHeader separator>Database</OptionsHeader>

      <Box>Create Database User</Box>
      <BoolRadioGroup
        name="create-db-user"
        value={value.createDBUser}
        onChange={c => onChange({ ...value, createDBUser: c })}
      />

      {/* TODO(bl-nero): a bug in YAML unmarshalling backend breaks the
          createDBUserMode field. Fix it and add the field here. */}
      <OptionLabel htmlFor={createDBUserModeId}>
        Create Database User Mode
      </OptionLabel>
      <Select
        inputId={createDBUserModeId}
        isDisabled={isProcessing}
        options={createDBUserModeOptions}
        value={value.createDBUserMode}
        onChange={m => onChange?.({ ...value, createDBUserMode: m })}
      />

      <OptionsHeader separator>Desktop</OptionsHeader>

      <Box>Create Desktop User</Box>
      <BoolRadioGroup
        name="create-desktop-user"
        value={value.createDesktopUser}
        onChange={c => onChange({ ...value, createDesktopUser: c })}
      />

      <Box>Allow Clipboard Sharing</Box>
      <BoolRadioGroup
        name="desktop-clipboard"
        value={value.desktopClipboard}
        onChange={c => onChange({ ...value, desktopClipboard: c })}
      />

      <Box>Allow Directory Sharing</Box>
      <BoolRadioGroup
        name="desktop-directory-sharing"
        value={value.desktopDirectorySharing}
        onChange={s => onChange({ ...value, desktopDirectorySharing: s })}
      />

      <Box>Record Desktop Sessions</Box>
      <BoolRadioGroup
        name="record-desktop-sessions"
        value={value.recordDesktopSessions}
        onChange={r => onChange({ ...value, recordDesktopSessions: r })}
      />
    </OptionsGridContainer>
  );
});

const SSHPortForwardingModeOptionComponent = (
  props: OptionProps<SSHPortForwardingModeOption, false>
) => {
  return (
    <components.Option {...props}>
      {props.label} <Text typography="body3">{props.data.description}</Text>
    </components.Option>
  );
};

const sshPortForwardingModeComponents = {
  Option: SSHPortForwardingModeOptionComponent,
};

const OptionsGridContainer = styled(Box)`
  display: grid;
  grid-template-columns: 1fr 1fr;
  align-items: baseline;
  row-gap: ${props => props.theme.space[3]}px;
`;

const OptionsHeader = styled(H4)<{ separator?: boolean }>`
  grid-column: 1/3;
  border-top: ${props =>
    props.separator
      ? `${props.theme.borders[1]} ${props.theme.colors.interactive.tonal.neutral[0]}`
      : 'none'};
  padding-top: ${props =>
    props.separator ? `${props.theme.space[3]}px` : '0'};
`;

function BoolRadioGroup({
  name,
  value,
  onChange,
}: {
  name: string;
  value: boolean;
  onChange(b: boolean): void;
}) {
  return (
    <RadioGroup
      name={name}
      flexDirection="row"
      options={[
        { label: 'True', value: 'true' },
        { label: 'False', value: 'false' },
      ]}
      value={String(value)}
      onChange={d => onChange(d === 'true')}
    />
  );
}

const OptionLabel = styled(LabelInput)`
  ${props => props.theme.typography.body2}
`;
