import { zodResolver } from '@hookform/resolvers/zod';
import { useCallback } from 'react';
import { useForm, type SubmitHandler } from 'react-hook-form';
import { z } from 'zod';

import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';

import { Form } from 'e-teleport/Integrations/SessionSummaries/fields/Form';
import { InferenceSecretForm } from 'e-teleport/Integrations/SessionSummaries/forms/InferenceSecretForm';
import { useSessionSummariesManagement } from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import { useUpdateInferenceSecret } from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

export const editInferenceSecretSchema = z.object({
  apiKey: z.string().min(1, 'API Key is required'),
});

type EditInferenceSecretSchema = z.infer<typeof editInferenceSecretSchema>;

interface EditInferenceSecretProps {
  name: string;
}

export function EditInferenceSecret({ name }: EditInferenceSecretProps) {
  const { clusterId } = useStickyClusterId();

  const { closeCurrentOverlay } = useSessionSummariesManagement();

  const form = useForm({
    defaultValues: {
      apiKey: '',
    },
    mode: 'onChange',
    resolver: zodResolver(editInferenceSecretSchema),
  });

  const edit = useUpdateInferenceSecret();

  const handleSubmit = useCallback<SubmitHandler<EditInferenceSecretSchema>>(
    async values => {
      try {
        await edit.mutateAsync({
          clusterId,
          name,
          secret: {
            name,
            value: values.apiKey,
          },
        });

        closeCurrentOverlay();
      } catch {
        // handled by TanStack Query
      }
    },
    [edit, clusterId, name, closeCurrentOverlay]
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
        <DialogTitle>Editing Inference Secret: {name}</DialogTitle>
      </DialogHeader>

      <Form form={form} onSubmit={handleSubmit}>
        <InferenceSecretForm
          clusterId={clusterId}
          isCreate={false}
          isError={edit.isError}
          isPending={edit.isPending}
          isValid={form.formState.isValid}
          error={edit.error}
          name={name}
          onClose={closeCurrentOverlay}
        />
      </Form>
    </Dialog>
  );
}
