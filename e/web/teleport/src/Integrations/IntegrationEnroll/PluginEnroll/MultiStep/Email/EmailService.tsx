import React, { useState } from 'react';

import { PluginEmailSpec } from 'teleport/services/integrations';
import {
  Alert,
  ButtonPrimary,
  ButtonSecondary,
  Text,
  Box,
  Flex,
  Link,
} from 'design';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import {
  requiredField,
  requiredPort,
} from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import { pluginsService } from 'e-teleport/services/plugins';

import { usePlugin } from '../usePlugin';
import { Header } from '../Shared';

import { FormDataField } from './types';
export function EmailService() {
  const { formData, nextStep, prevStep, setInstalledPlugin } =
    usePlugin<PluginEmailSpec>();
  const emailService = formData.get(FormDataField.Service)?.toString();

  // Mailgun configuration
  const [domain, setDomain] = useState('');
  const [privateKey, setPrivateKey] = useState('');

  // SMTP configuration
  const [host, setHost] = useState('');
  const [port, setPort] = useState('587');
  const [startTLSPolicy, setStartTLSPolicy] = useState<Option>({
    value: 'mandatory',
    label: 'Mandatory',
  });
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const { attempt, run } = useAttempt();

  function handleSubmit(validator: Validator) {
    let valid = true;

    if (!validator.validate()) {
      valid = false;
    }

    if (!valid) return;

    if (emailService == 'mailgun') {
      formData.set(FormDataField.Domain, domain);
      formData.set(FormDataField.PrivateKey, privateKey);
    }

    if (emailService == 'smtp') {
      formData.set(FormDataField.Host, host);
      formData.set(FormDataField.Port, port);
      formData.set(FormDataField.StartTLSPolicy, startTLSPolicy.value);
      formData.set(FormDataField.Username, username);
      formData.set(FormDataField.Password, password);
    }

    run(async () => {
      const resp = await pluginsService.createPlugin(formData);
      setInstalledPlugin(resp);
      nextStep();
    });
  }

  function mailgunCredentials() {
    return (
      <>
        <Text my={1} bold>
          Provide Mailgun API configuration
        </Text>
        <Text my={1}>
          Create a Mailgun private key by following the{' '}
          <Link
            target="_blank"
            href="https://documentation.mailgun.com/docs/mailgun/user-manual/get-started/#domain-sending-keys"
          >
            Domain Sending Keys
          </Link>{' '}
          documentation. Copy and paste the key into the "Mailgun Private Key"
          field on this screen.
        </Text>
        <FieldInput
          width="500px"
          label="Domain"
          name={FormDataField.Domain}
          rule={requiredField('Domain Required')}
          value={domain}
          onChange={e => setDomain(e.target.value)}
          placeholder="sandbox.mailgun.org"
          toolTipContent="Domain specifies the Mailgun sending domain"
          mb={3}
        />
        <FieldInput
          width="500px"
          label="Mailgun Private Key"
          name={FormDataField.PrivateKey}
          rule={requiredField('Private Key Required')}
          value={privateKey}
          type="password"
          onChange={e => setPrivateKey(e.target.value)}
          placeholder="abc-def...-123"
          toolTipContent="Private Key is used to access the Mailgun API"
          mb={3}
        />
      </>
    );
  }

  function smtpCredentials() {
    return (
      <>
        <Text my={1} bold>
          Provide SMTP service configuration
        </Text>
        <FieldInput
          width="500px"
          label="Host"
          name={FormDataField.Host}
          rule={requiredField('Host Required')}
          value={host}
          onChange={e => setHost(e.target.value)}
          placeholder="smtp.example.com"
          toolTipContent="Host specifies the SMTP service host name"
          mb={3}
        />
        <FieldInput
          width="500px"
          label="Port"
          name={FormDataField.Port}
          rule={requiredPort}
          value={port}
          onChange={e => setPort(e.target.value)}
          placeholder="587"
          toolTipContent="Port specifies the SMTP service port number"
          mb={3}
        />
        <FieldSelect
          width="500px"
          label="Start TLS Policy"
          name={FormDataField.StartTLSPolicy}
          rule={requiredField<Option>('Start TLS Policy required')}
          value={startTLSPolicy}
          onChange={o => setStartTLSPolicy(o as Option)}
          options={[
            {
              value: 'mandatory',
              label: 'Mandatory',
            },
            {
              value: 'opportunistic',
              label: 'Opportunistic',
            },
            {
              value: 'disabled',
              label: 'Disabled',
            },
          ]}
          placeholder="Mandatory"
          toolTipContent="Start TLS Policy specifies the SMTP TLS security level"
          isSearchable
          mb={3}
        />
        <FieldInput
          width="500px"
          label="Username"
          name={FormDataField.Username}
          rule={requiredField('Username Required')}
          value={username}
          onChange={e => setUsername(e.target.value)}
          placeholder="username@example.com"
          toolTipContent="Username specifies the SMTP service username credential"
          mb={3}
        />
        <FieldInput
          width="500px"
          label="Password"
          name={FormDataField.Password}
          rule={requiredField('Password Required')}
          value={password}
          type="password"
          onChange={e => setPassword(e.target.value)}
          placeholder="abc-def...-123"
          toolTipContent="Password specifies the SMTP service password credential"
          mb={3}
        />
      </>
    );
  }

  return (
    <Box width="800px">
      <Validation>
        {({ validator }) => (
          <>
            <Box mb={3}>
              <Header header="Set up Email Service" />
              <Text>
                Provide the required configuration to connect to your desired
                Email service. Teleport will interface with the Email service to
                send Access Requests as emails.
              </Text>
            </Box>
            <Flex mt={4} flexDirection="column" gap={1} width="100%">
              {attempt.status === 'failed' && (
                <Alert children={attempt.statusText} />
              )}
              {emailService === 'mailgun' && mailgunCredentials()}
              {emailService === 'smtp' && smtpCredentials()}
              <Flex mt={3} mb={4} gap={3}>
                <ButtonPrimary
                  disabled={attempt.status === 'processing'}
                  onClick={() => handleSubmit(validator)}
                >
                  Finish
                </ButtonPrimary>
                <ButtonSecondary onClick={() => prevStep()}>
                  Back
                </ButtonSecondary>
              </Flex>
            </Flex>
          </>
        )}
      </Validation>
    </Box>
  );
}
