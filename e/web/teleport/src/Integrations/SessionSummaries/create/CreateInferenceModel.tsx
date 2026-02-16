import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useForm, type SubmitHandler } from 'react-hook-form';
import { z } from 'zod';

import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';

import cfg from 'e-teleport/config';
import { Form } from 'e-teleport/Integrations/SessionSummaries/fields/Form';
import { InferenceModelForm } from 'e-teleport/Integrations/SessionSummaries/forms/InferenceModelForm';
import {
  convertModelSchemaToApi,
  modelSchema,
} from 'e-teleport/Integrations/SessionSummaries/schema/model';
import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import {
  listInferenceModelsQueryKey,
  useCreateInferenceModel,
} from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

const createInferenceModelSchema = modelSchema.and(
  z.object({
    name: z.string().min(1, 'Name is required'),
  })
);

type CreateInferenceModelSchema = z.infer<typeof createInferenceModelSchema>;

export function CreateInferenceModel() {
  const { clusterId } = useStickyClusterId();

  const { closeCurrentOverlay, setPendingSelection } =
    useSessionSummariesManagement();

  const isCloud = cfg.oss.isCloud;

  const form = useForm<CreateInferenceModelSchema>({
    defaultValues: {
      cloud: isCloud,
      accessMethod: 'openai_compatible',
      apiUrl: '',
      secretName: '',
      apiKeyMode: 'existing',
      name: '',
      model: '',
    },
    mode: 'onChange',
    resolver: zodResolver(createInferenceModelSchema),
  });

  const queryClient = useQueryClient();

  const create = useCreateInferenceModel({
    onSuccess(data) {
      const queryKey = listInferenceModelsQueryKey({
        clusterId,
        limit: 10,
      });

      void queryClient.refetchQueries({ queryKey });
      setPendingSelection({ entity: OverlayEntity.Model, name: data.name });
    },
  });

  const handleSubmit = useCallback<SubmitHandler<CreateInferenceModelSchema>>(
    async values => {
      const model = convertModelSchemaToApi(values.name, values);
      try {
        await create.mutateAsync({
          clusterId,
          model,
        });
        closeCurrentOverlay();
      } catch {
        // error handling is done in the hook
      }
    },
    [create, clusterId, closeCurrentOverlay]
  );

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '900px',
        width: '100%',
        overflow: 'unset',
      })}
      disableEscapeKeyDown={false}
      onClose={closeCurrentOverlay}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Create Inference Model</DialogTitle>
      </DialogHeader>
      <Form form={form} onSubmit={handleSubmit}>
        <InferenceModelForm
          clusterId={clusterId}
          isCloud={isCloud}
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
