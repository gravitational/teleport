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

import { KeyboardEvent, useEffect, useState } from 'react';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Link,
  Text,
} from 'design';
import { DialogContent, DialogFooter } from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAttemptNext';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import { LabelsCreater } from 'teleport/Discover/Shared';
import { ResourceLabelTooltip } from 'teleport/Discover/Shared/ResourceLabelTooltip';
import { ResourceLabel } from 'teleport/services/agents';

import { State } from './useAddApp';

export function Automatically(props: Props) {
  const { onClose, attempt, token, labels, setLabels } = props;

  const [name, setName] = useState('');
  const [uri, setUri] = useState('');
  const [cmd, setCmd] = useState('');

  useEffect(() => {
    if (name && uri && token) {
      const cmd = createAppBashCommand(token.id, name, uri);
      setCmd(cmd);
    }
  }, [token]);

  function onGenerateScript(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    props.onCreate(name, uri);
  }

  function handleEnterPress(
    e: KeyboardEvent<HTMLInputElement>,
    validator: Validator
  ) {
    if (e.key === 'Enter') {
      onGenerateScript(validator);
    }
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          <DialogContent minHeight="254px" flex="0 0 auto">
            <Flex alignItems="top" flexDirection="row">
              <FieldInput
                rule={requiredAppName}
                label="App Name"
                autoFocus
                value={name}
                placeholder="jenkins"
                width="320px"
                mr="3"
                onKeyPress={e => handleEnterPress(e, validator)}
                onChange={e => setName(e.target.value.toLowerCase())}
                disabled={attempt.status === 'processing'}
              />
              <FieldInput
                rule={requiredAppUri}
                label="Internal Application URL"
                width="100%"
                value={uri}
                placeholder="https://localhost:4000"
                onKeyPress={e => handleEnterPress(e, validator)}
                onChange={e => setUri(e.target.value)}
                disabled={attempt.status === 'processing'}
              />
            </Flex>
            <Box mt={-3} mb={3}>
              <Flex alignItems="center" gap={1} mb={2} mt={4}>
                <Text bold>Add Labels (Optional)</Text>
                <ResourceLabelTooltip
                  toolTipPosition="top"
                  resourceKind="app"
                />
              </Flex>
              <LabelsCreater
                labels={labels}
                setLabels={setLabels}
                isLabelOptional={true}
                disableBtns={attempt.status === 'processing'}
                noDuplicateKey={true}
              />
            </Box>
            {!cmd && (
              <Text mb="3">
                Teleport can automatically set up application access. Provide
                the name and URL of your application to generate our
                auto-installer script.
                <Text mt="2">
                  The script will install the Teleport agent to provide secure
                  access to your application.
                </Text>
              </Text>
            )}
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            {cmd && (
              <>
                <Text mb="3">
                  Use the script below to add an application to your cluster.{' '}
                  The script will be valid for
                  <Text bold as="span">
                    {` ${token.expiryText}`}.
                  </Text>
                  {renderUrl(name)}
                </Text>
                <TextSelectCopy text={cmd} mb={2} />
              </>
            )}
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => onGenerateScript(validator)}
            >
              {cmd ? 'Regenerate Script' : 'Generate Script'}
            </ButtonPrimary>
            <ButtonSecondary
              disabled={attempt.status === 'processing'}
              onClick={onClose}
            >
              Close
            </ButtonSecondary>
          </DialogFooter>
        </>
      )}
    </Validation>
  );
}

function renderUrl(name = '') {
  const url = `https://${name}.${window.location.host}`;
  return (
    <Text>
      This app will be available on {`  `}
      <Link target="_blank" href={url}>
        {`${url}`}
      </Link>
    </Text>
  );
}

// Validation logic matches backend checks for app URI
const ALLOWED_APPURI_REGEXP = /^[-\w/:. ]+$/;
const requiredAppUri = value => () => {
  if (!value) {
    return {
      valid: false,
      message: 'Required',
    };
  }

  try {
    new URL(value);
  } catch {
    return {
      valid: false,
      message: 'URL is invalid',
    };
  }

  const appUriMatch = value.match(ALLOWED_APPURI_REGEXP);
  if (!appUriMatch) {
    return {
      valid: false,
      message: 'Invalid app URI',
    };
  }

  return {
    valid: true,
  };
};

/**
 * Conforms to rfc 1035 name syntax where:
 * - name should start with alphabets and end with alphanumerics
 * - interior characters are only alphanumerics and hyphens
 * - string must be 63 chars or less
 */
const REGEX_DNS1035LABEL = /^[a-z]([-a-z0-9]*[a-z0-9])?$/;
const DNS1035LABEL_MAXLENGTH = 63;
const requiredAppName = value => () => {
  if (!value || value.length === 0) {
    return {
      valid: false,
      message: 'Required',
    };
  }

  if (value.length > DNS1035LABEL_MAXLENGTH) {
    return {
      valid: false,
      message: 'Must be 63 chars or less',
    };
  }

  const match = value.match(REGEX_DNS1035LABEL);
  if (!match) {
    return {
      valid: false,
      message: 'Invalid DNS sub-domain name',
    };
  }

  return {
    valid: true,
  };
};

export const createAppBashCommand = (
  tokenId: string,
  appName: string,
  appUri: string
): string => {
  // encode uri so it can be passed around as URL query parameter
  const encoded = encodeURIComponent(appUri)
    // encode single quotes so they do not break the curl parameters
    .replace(/'/g, '%27');
  const bashUrl =
    cfg.baseUrl +
    cfg.api.appNodeScriptPath
      .replace(':token', tokenId)
      .replace(':name', appName)
      .replace(':uri', encoded);

  return `sudo bash -c "$(curl -fsSL '${bashUrl}')"`;
};

type Props = {
  onClose(): void;
  onCreate(name: string, uri: string): Promise<any>;
  token: State['token'];
  attempt: Attempt;
  labels: ResourceLabel[];
  setLabels(r: ResourceLabel[]): void;
};
