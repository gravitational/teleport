import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import type { FallbackProps } from 'react-error-boundary';
import { useForm, type SubmitHandler } from 'react-hook-form';
import { z } from 'zod';

import { Alert } from 'design/Alert';
import { ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { Indicator } from 'design/Indicator';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';

import cfg from 'e-teleport/config';
import { Form } from 'e-teleport/Integrations/SessionSummaries/fields/Form';
import { InferenceModelForm } from 'e-teleport/Integrations/SessionSummaries/forms/InferenceModelForm';
import {
  convertModelSchemaToApi,
  modelSchema,
} from 'e-teleport/Integrations/SessionSummaries/schema/model';
import { useSessionSummariesManagement } from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import type { InferenceModel } from 'e-teleport/services/inference';
import {
  getInferenceModelQueryKey,
  listInferenceModelsQueryKey,
  useSuspenseGetInferenceModel,
  useUpdateInferenceModel,
} from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

type EditInferenceModelSchema = z.infer<typeof modelSchema>;

interface EditInferenceModelProps {
  name: string;
}

export function EditInferenceModel({ name }: EditInferenceModelProps) {
  const { closeCurrentOverlay } = useSessionSummariesManagement();

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
        <DialogTitle>Editing Inference Model: {name}</DialogTitle>
      </DialogHeader>
      <ErrorSuspenseWrapper
        errorComponent={LoadingInferenceModelFailed}
        loadingComponent={LoadingInferenceModel}
      >
        <EditInferenceModelInner name={name} />
      </ErrorSuspenseWrapper>
    </Dialog>
  );
}

function LoadingInferenceModel() {
  return (
    <DialogContent alignItems="center" justifyContent="center">
      <Indicator delay="none" />
    </DialogContent>
  );
}

function LoadingInferenceModelFailed({ error }: FallbackProps) {
  const { closeCurrentOverlay } = useSessionSummariesManagement();

  return (
    <DialogContent>
      <Alert kind="danger">
        Failed to load inference model: {String(error)}
      </Alert>
      <div>
        <ButtonSecondary onClick={closeCurrentOverlay}>Close</ButtonSecondary>
      </div>
    </DialogContent>
  );
}

function createDefaultValues(
  model: InferenceModel,
  isCloud: boolean
): EditInferenceModelSchema {
  if (model.openai) {
    return {
      cloud: isCloud,
      accessMethod: 'openai_compatible',
      apiKeyMode: 'existing',
      apiUrl: model.openai.base_url ?? '',
      secretName: model.openai.api_key_secret_ref ?? '',
      model: model.openai.modelId,
    };
  }

  if (model.bedrock) {
    // will be fully fleshed out in the next PR with the Bedrock form
    return {} as EditInferenceModelSchema;
  }

  throw new Error('Unsupported inference model configuration');
}

function EditInferenceModelInner({ name }: EditInferenceModelProps) {
  const { closeCurrentOverlay } = useSessionSummariesManagement();
  const { clusterId } = useStickyClusterId();

  const isCloud = cfg.oss.isCloud;

  const policy = useSuspenseGetInferenceModel({ clusterId, name });

  const form = useForm<EditInferenceModelSchema>({
    defaultValues: createDefaultValues(policy.data, cfg.oss.isCloud),
    mode: 'onChange',
    resolver: zodResolver(modelSchema),
  });

  const queryClient = useQueryClient();

  const edit = useUpdateInferenceModel({
    onSuccess(data) {
      const listModelsQueryKey = listInferenceModelsQueryKey({
        clusterId,
        limit: 10,
      });
      const getModelQueryKey = getInferenceModelQueryKey({
        clusterId,
        name,
      });

      void queryClient.refetchQueries({ queryKey: listModelsQueryKey });
      void queryClient.setQueryData(getModelQueryKey, data);
    },
  });

  const handleSubmit = useCallback<SubmitHandler<EditInferenceModelSchema>>(
    async values => {
      const model = convertModelSchemaToApi(name, values);
      try {
        await edit.mutateAsync({
          clusterId,
          name,
          model,
        });
        closeCurrentOverlay();
      } catch {
        // handled by TanStack Query
      }
    },
    [edit, clusterId, name, closeCurrentOverlay]
  );

  return (
    <Form form={form} onSubmit={handleSubmit}>
      <InferenceModelForm
        clusterId={clusterId}
        isCloud={isCloud}
        isCreate={false}
        isError={edit.isError}
        isPending={edit.isPending}
        isValid={form.formState.isValid}
        error={edit.error}
        name={name}
        onClose={closeCurrentOverlay}
      />
    </Form>
  );
}
