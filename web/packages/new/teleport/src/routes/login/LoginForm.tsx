import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Button, Card, Field, Heading, Input, VStack } from 'design-new';

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
  username: z.string().min(1),
  password: z.string().min(1),
});

type Schema = z.infer<typeof schema>;

function LocalLoginForm() {
  const { formState, handleSubmit, register } = useForm<Schema>({
    resolver: zodResolver(schema),
  });

  function onSubmit(data: Schema) {
    console.log('data', data);
  }

  return (
    <form
      onSubmit={() => {
        void handleSubmit(onSubmit);
      }}
    >
      <Field.Root>
        <Field.Label>Username</Field.Label>

        <Input placeholder="Username" {...register('username')} />

        <Field.ErrorText>This is an error text</Field.ErrorText>
      </Field.Root>

      <Field.Root mb={4}>
        <Field.Label>Password</Field.Label>

        <Input
          placeholder="Password"
          type="password"
          hasError={true}
          {...register('password')}
        />

        <Field.ErrorText>This is an error text</Field.ErrorText>
      </Field.Root>

      <Button
        loading={formState.isSubmitting}
        disabled={!formState.isValid}
        type="submit"
      >
        hello
      </Button>
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
