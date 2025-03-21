import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import {
  Box,
  ButtonPrimary,
  Card,
  Field,
  Flex,
  Heading,
  Input,
  Select,
  VStack,
} from 'design-new';

import { LogoHero } from '../../components/LogoHero/LogoHero';
import { useCheckSessionAndRedirect } from './useCheckSessionAndRedirect';

interface NewLoginFormWithRecoveryProps {
  isRecoveryEnabled: boolean;
  onRecover: (isRecoverPassword: boolean) => void;
}

interface NewLoginFormWithoutRecoveryEnabledProps {
  isRecoveryEnabled: never;
  onRecover: never;
}

type NewLoginFormProps =
  | NewLoginFormWithRecoveryProps
  | NewLoginFormWithoutRecoveryEnabledProps;

const schema = z.object({
  username: z.string().min(1, { message: 'Username is required' }),
  password: z.string().min(1, { message: 'Password is required' }),
});

type Schema = z.infer<typeof schema>;

const options = [
  {
    label: 'Passkey or Security Key',
    value: 'passkey',
  },
  {
    label: 'Authenticator App',
    value: 'authenticator',
  },
];

function LocalLoginForm() {
  const { formState, handleSubmit, register } = useForm<Schema>({
    resolver: zodResolver(schema),
  });

  function onSubmit(data: Schema) {
    console.log('data', data);
  }

  console.log(formState.errors);

  return (
    <form
      onSubmit={e => {
        void handleSubmit(onSubmit)(e);
      }}
    >
      <VStack gap={5}>
        <Field.Root invalid={!!formState.errors.username}>
          <Field.Label>Username</Field.Label>

          <Input placeholder="Username" {...register('username')} />

          <Field.ErrorText>
            {formState.errors.username?.message}
          </Field.ErrorText>
        </Field.Root>

        <Field.Root invalid={!!formState.errors.password}>
          <Field.Label>Password</Field.Label>

          <Input
            placeholder="Password"
            type="password"
            {...register('password')}
          />

          <Field.ErrorText>
            {formState.errors.password?.message}
          </Field.ErrorText>
        </Field.Root>

        <Flex w="100%">
          <Box maxW="50%" w="100%">
            <Field.Root>
              <Field.Label>Multi-factor Type</Field.Label>

              <Select options={options} value={options[0]} />
            </Field.Root>
          </Box>
        </Flex>

        <ButtonPrimary
          loading={formState.isSubmitting}
          size="xl"
          type="submit"
          w="100%"
        >
          Sign In
        </ButtonPrimary>
      </VStack>
    </form>
  );
}

export function NewLoginForm({
  isRecoveryEnabled,
  onRecover,
}: NewLoginFormProps) {
  const checkingValidSession = useCheckSessionAndRedirect();

  if (checkingValidSession) {
    return null;
  }

  console.log(Card);

  return (
    <VStack>
      <LogoHero my="48px" />

      <Card.Root w={500}>
        <Card.Body>
          <Heading mb="24px" size="2xl" textAlign="center">
            Sign in to Teleport
          </Heading>

          <LocalLoginForm />
        </Card.Body>
      </Card.Root>
    </VStack>
  );
}
