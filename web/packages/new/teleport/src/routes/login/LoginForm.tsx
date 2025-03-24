import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation } from '@tanstack/react-query';
import React, { useRef, type FormEvent } from 'react';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import { useHistory, useLocation } from 'react-router';
import { z } from 'zod';

import {
  Box,
  ButtonLink,
  ButtonPrimary,
  Card,
  FieldInput,
  FieldInputPassword,
  FieldSelect,
  Flex,
  Heading,
  VStack,
  type SelectInstance,
} from 'design-new';

import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';

import { LogoHero } from '../../components/LogoHero/LogoHero';
import { cfg } from '../../config';
import { auth } from '../../services/auth/auth';
import { session } from '../../services/websession/websession';
import {
  getRedirectUri,
  useCheckSessionAndRedirect,
} from './useCheckSessionAndRedirect';

export enum RecoverType {
  Password,
  Mfa,
}

interface NewLoginFormWithRecoveryProps {
  isRecoveryEnabled: boolean;
  onRecover: (type: RecoverType) => void;
}

interface NewLoginFormWithoutRecoveryEnabledProps {
  isRecoveryEnabled: never;
  onRecover: never;
}

type NewLoginFormProps =
  | NewLoginFormWithRecoveryProps
  | NewLoginFormWithoutRecoveryEnabledProps;

const mfaEnum = z.enum(['passkey', 'authenticator'], {
  errorMap: () => ({ message: 'Multi-factor type is required' }),
});

type MfaType = z.infer<typeof mfaEnum>;

const base = z.object({
  username: z.string().min(1, { message: 'Username is required' }),
  password: z.string().min(1, { message: 'Password is required' }),
  mfa: mfaEnum,
});

const totpSchema = base.extend({
  mfa: z.literal('authenticator'),
  totp: z
    .string({
      required_error: 'Authenticator code is required',
    })
    .length(6, {
      message: 'Authenticator code must be 6 digits',
    }),
});

const passkeySchema = base.extend({
  mfa: z.literal('passkey'),
});

const schema = z.discriminatedUnion('mfa', [passkeySchema, totpSchema]);

type Schema = z.infer<typeof schema>;

interface Option {
  label: string;
  value: string;
}

const options: Option[] = [
  {
    label: 'Passkey or Security Key',
    value: 'passkey',
  },
  {
    label: 'Authenticator App',
    value: 'authenticator',
  },
];

function TotpInput() {
  const mfa = useWatch<Schema>({
    name: 'mfa',
  });

  if (mfa !== 'authenticator') {
    return null;
  }

  return (
    <FieldInput
      autoFocus
      autoComplete="one-time-code"
      name="totp"
      placeholder="123 456"
      label="Authenticator Code"
      type="text"
    />
  );
}

function LocalLoginForm({ isRecoveryEnabled, onRecover }: NewLoginFormProps) {
  const history = useHistory();
  const location = useLocation();

  const form = useForm<Schema>({
    resolver: zodResolver(schema),
    defaultValues: {
      username: '',
      password: '',
      mfa: 'passkey',
    },
  });

  const login = useMutation({
    mutationFn: async (data: Schema) => {
      switch (data.mfa) {
        case 'authenticator':
          return auth.login(data.username, data.password, data.totp);

        case 'passkey':
          return auth.loginWithWebauthn({
            username: data.username,
            password: data.password,
          });
      }
    },
    onSuccess({ deviceWebToken, trustedDeviceRequirement }) {
      if (trustedDeviceRequirement === TrustedDeviceRequirement.REQUIRED) {
        session.setDeviceTrustRequired();
      }

      if (deviceWebToken && cfg.edition === 'ent') {
        const redirect = getRedirectUri(location.search);

        const authorize = cfg.getDeviceTrustAuthorizeRoute(
          deviceWebToken.id,
          deviceWebToken.token,
          redirect
        );

        history.push(authorize, true);

        return;
      }

      const redirect = getRedirectUri(location.search);

      history.push(redirect, /* withPageRefresh */ true);
    },
  });

  function handleSubmit(event: FormEvent) {
    void form.handleSubmit(data => login.mutateAsync(data))(event);
  }

  const ref = useRef<SelectInstance<Option>>(null);

  return (
    <FormProvider {...form}>
      {login.isError && (
        <Alerts.Danger m={4}>{login.error.message}</Alerts.Danger>
      )}

      <VStack as="form" onSubmit={handleSubmit} gap={5}>
        <FieldInput
          autoComplete="username"
          autoFocus
          name="username"
          placeholder="Username"
          label="Username"
        />

        <FieldInputPassword
          autoComplete="current-password"
          name="password"
          label="Password"
          helperText={
            isRecoveryEnabled && (
              <ButtonLink
                p={0}
                minH={0}
                onClick={() => {
                  onRecover(RecoverType.Password);
                }}
              >
                Forgot Password?
              </ButtonLink>
            )
          }
        />

        <Flex w="100%" gap={4}>
          <Box maxW="50%" w="100%">
            <FieldSelect
              label="Multi-factor Type"
              ref={ref}
              name="mfa"
              options={options}
              helperText={
                isRecoveryEnabled && (
                  <ButtonLink
                    p={0}
                    minH={0}
                    onClick={() => {
                      onRecover(RecoverType.Mfa);
                    }}
                  >
                    Lost Two-Factor Device?
                  </ButtonLink>
                )
              }
            />
          </Box>

          <TotpInput />
        </Flex>

        <ButtonPrimary
          loading={form.formState.isSubmitting}
          size="xl"
          type="submit"
          block
        >
          Sign In
        </ButtonPrimary>
      </VStack>
    </FormProvider>
  );
}

export function NewLoginForm(props: NewLoginFormProps) {
  const checkingValidSession = useCheckSessionAndRedirect();

  if (checkingValidSession) {
    return null;
  }

  return (
    <VStack>
      <LogoHero my="48px" />

      <Card.Root w={500}>
        <Card.Body>
          <Heading mb="24px" size="2xl" textAlign="center">
            Sign in to Teleport
          </Heading>

          <LocalLoginForm {...props} />
        </Card.Body>
      </Card.Root>
    </VStack>
  );
}
