import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useForm, type SubmitHandler } from 'react-hook-form';
import { z } from 'zod';

import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';

import {
  listInferencePoliciesQueryKey,
  useCreateInferencePolicy,
} from 'e-teleport/services/inference/hooks';
import { Form } from 'e-teleport/SessionRecordings/setup/fields/Form';
import { InferencePolicyForm } from 'e-teleport/SessionRecordings/setup/forms/InferencePolicyForm';
import { TELEPORT_CLOUD_MODEL } from 'e-teleport/SessionRecordings/setup/schema/accessMethods';
import { inferencePolicySchema } from 'e-teleport/SessionRecordings/setup/schema/policy';
import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';
import useStickyClusterId from 'teleport/useStickyClusterId';

const createInferencePolicySchema = inferencePolicySchema.and(
  z.object({
    name: z.string().min(1, 'Name is required'),
  })
);

type CreateInferencePolicySchema = z.infer<typeof createInferencePolicySchema>;

interface CreateInferencePolicyProps {
  isCloud: boolean;
}

function getDefaultValues(isCloud: boolean): CreateInferencePolicySchema {
  if (isCloud) {
    return {
      name: '',
      kinds: [],
      model: TELEPORT_CLOUD_MODEL,
      providedByTeleportCloud: true,
      acceptedTerms: false,
    };
  }

  return {
    name: '',
    kinds: [],
    model: '',
    providedByTeleportCloud: false,
  };
}

export function CreateInferencePolicy({ isCloud }: CreateInferencePolicyProps) {
  const { clusterId } = useStickyClusterId();

  const { closeCurrentOverlay, setPendingSelection } =
    useSessionSummariesManagement();

  const form = useForm<CreateInferencePolicySchema>({
    defaultValues: getDefaultValues(isCloud),
    mode: 'onChange',
    resolver: zodResolver(createInferencePolicySchema),
  });

  const queryClient = useQueryClient();

  const create = useCreateInferencePolicy({
    onSuccess(data) {
      const queryKey = listInferencePoliciesQueryKey({
        clusterId,
        limit: 10,
      });

      void queryClient.refetchQueries({ queryKey });

      setPendingSelection({ entity: OverlayEntity.Policy, name: data.name });
    },
  });

  const handleSubmit = useCallback<SubmitHandler<CreateInferencePolicySchema>>(
    async values => {
      try {
        await create.mutateAsync({
          clusterId,
          policy: {
            name: values.name,
            kinds: values.kinds,
            model: values.model,
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
        maxWidth: '900px',
        width: '100%',
        overflow: 'unset',
      })}
      disableEscapeKeyDown={false}
      onClose={closeCurrentOverlay}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Create Inference Policy</DialogTitle>
      </DialogHeader>

      <Form form={form} onSubmit={handleSubmit}>
        <InferencePolicyForm
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
