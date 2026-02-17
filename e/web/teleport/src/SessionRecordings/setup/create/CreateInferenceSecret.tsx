import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useForm, type SubmitHandler } from 'react-hook-form';
import { z } from 'zod';

import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';

import {
  listInferenceSecretsQueryKey,
  useCreateInferenceSecret,
} from 'e-teleport/services/inference/hooks';
import { editInferenceSecretSchema } from 'e-teleport/SessionRecordings/setup/edit/EditInferenceSecret';
import { Form } from 'e-teleport/SessionRecordings/setup/fields/Form';
import { InferenceSecretForm } from 'e-teleport/SessionRecordings/setup/forms/InferenceSecretForm';
import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';
import useStickyClusterId from 'teleport/useStickyClusterId';

const createInferenceSecretSchema = editInferenceSecretSchema.extend({
  name: z.string().min(1, 'Name is required'),
});

type CreateInferenceSecretSchema = z.infer<typeof createInferenceSecretSchema>;

export function CreateInferenceSecret() {
  const { clusterId } = useStickyClusterId();

  const { closeCurrentOverlay, setPendingSelection } =
    useSessionSummariesManagement();

  const form = useForm({
    defaultValues: {
      apiKey: '',
      name: '',
    },
    mode: 'onChange',
    resolver: zodResolver(createInferenceSecretSchema),
  });

  const queryClient = useQueryClient();

  const create = useCreateInferenceSecret({
    onSuccess(data) {
      const queryKey = listInferenceSecretsQueryKey({
        clusterId,
        limit: 10,
      });

      void queryClient.refetchQueries({ queryKey });

      setPendingSelection({ entity: OverlayEntity.Secret, name: data.name });
    },
  });

  const handleSubmit = useCallback<SubmitHandler<CreateInferenceSecretSchema>>(
    async values => {
      try {
        await create.mutateAsync({
          clusterId,
          secret: {
            name: values.name,
            value: values.apiKey,
          },
        });

        closeCurrentOverlay();
      } catch {
        // error handling is done in the hook
        // Jest will error on unhandled promise rejections
      }
    },
    [create, clusterId, closeCurrentOverlay]
  );

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '700px',
        width: '100%',
      })}
      disableEscapeKeyDown={false}
      onClose={closeCurrentOverlay}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Create Inference Secret</DialogTitle>
      </DialogHeader>

      <Form form={form} onSubmit={handleSubmit}>
        <InferenceSecretForm
          clusterId={clusterId}
          isCreate={true}
          isError={create.isError}
          isPending={create.isPending}
          isValid={form.formState.isValid}
          error={create.error}
          onClose={closeCurrentOverlay}
        />
      </Form>
    </Dialog>
  );
}
