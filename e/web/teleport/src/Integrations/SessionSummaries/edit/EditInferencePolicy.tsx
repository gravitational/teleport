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

import { Form } from 'e-teleport/Integrations/SessionSummaries/fields/Form';
import { InferencePolicyForm } from 'e-teleport/Integrations/SessionSummaries/forms/InferencePolicyForm';
import { inferencePolicySchema } from 'e-teleport/Integrations/SessionSummaries/schema/policy';
import type { ResourceKind } from 'e-teleport/Integrations/SessionSummaries/schema/types';
import { useSessionSummariesManagement } from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import {
  getInferencePolicyQueryKey,
  listInferencePoliciesQueryKey,
  useSuspenseGetInferencePolicy,
  useUpdateInferencePolicy,
} from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

type EditInferencePolicySchema = z.infer<typeof inferencePolicySchema>;

interface EditInferencePolicyProps {
  name: string;
}

export function EditInferencePolicy({ name }: EditInferencePolicyProps) {
  const { closeCurrentOverlay } = useSessionSummariesManagement();

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '700px',
        width: '100%',
        overflow: 'unset',
      })}
      disableEscapeKeyDown={false}
      onClose={closeCurrentOverlay}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Editing Inference Policy: {name}</DialogTitle>
      </DialogHeader>

      <ErrorSuspenseWrapper
        errorComponent={LoadingInferencePolicyFailed}
        loadingComponent={LoadingInferencePolicy}
      >
        <EditInferencePolicyInner name={name} />
      </ErrorSuspenseWrapper>
    </Dialog>
  );
}

function LoadingInferencePolicy() {
  return (
    <DialogContent alignItems="center" justifyContent="center">
      <Indicator delay="none" />
    </DialogContent>
  );
}

function LoadingInferencePolicyFailed({ error }: FallbackProps) {
  const { closeCurrentOverlay } = useSessionSummariesManagement();

  return (
    <DialogContent>
      <Alert kind="danger">
        Failed to load inference policy: {String(error)}
      </Alert>

      <div>
        <ButtonSecondary onClick={closeCurrentOverlay}>Close</ButtonSecondary>
      </div>
    </DialogContent>
  );
}

function EditInferencePolicyInner({ name }: EditInferencePolicyProps) {
  const { closeCurrentOverlay } = useSessionSummariesManagement();
  const { clusterId } = useStickyClusterId();

  const policy = useSuspenseGetInferencePolicy({ clusterId, name });

  const form = useForm<EditInferencePolicySchema>({
    defaultValues: {
      kinds: [...policy.data.kinds] as ResourceKind[],
      model: policy.data.model,
    },
    mode: 'onChange',
    resolver: zodResolver(inferencePolicySchema),
  });

  const queryClient = useQueryClient();

  const edit = useUpdateInferencePolicy({
    onSuccess(data) {
      const listPoliciesQueryKey = listInferencePoliciesQueryKey({
        clusterId,
        limit: 10,
      });

      const getPolicyQueryKey = getInferencePolicyQueryKey({
        clusterId,
        name,
      });

      void queryClient.refetchQueries({ queryKey: listPoliciesQueryKey });
      void queryClient.setQueryData(getPolicyQueryKey, data);
    },
  });

  const handleSubmit = useCallback<SubmitHandler<EditInferencePolicySchema>>(
    async values => {
      try {
        await edit.mutateAsync({
          clusterId,
          name,
          policy: {
            name,
            kinds: values.kinds,
            model: values.model,
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
    <Form form={form} onSubmit={handleSubmit}>
      <InferencePolicyForm
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
  );
}
