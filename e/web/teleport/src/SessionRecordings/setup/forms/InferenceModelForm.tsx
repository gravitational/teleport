import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { useFormContext, useWatch } from 'react-hook-form';
import styled from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import { DialogContent, DialogFooter } from 'design/Dialog';
import Flex from 'design/Flex';
import { HoverTooltip } from 'design/Tooltip';
import { getErrorMessage } from 'shared/utils/error';

import {
  listInferenceModelsQueryKey,
  listInferenceSecretsQueryKey,
  useDeleteInferenceModel,
} from 'e-teleport/services/inference/hooks';
import { FieldInput } from 'e-teleport/SessionRecordings/setup/fields/FieldInput';
import { FieldModelProvider } from 'e-teleport/SessionRecordings/setup/fields/FieldModelProvider';
import {
  TestConnectionButton,
  TestStatus,
} from 'e-teleport/SessionRecordings/setup/fields/TestConnectionButton';
import { CredentialsForm } from 'e-teleport/SessionRecordings/setup/forms/CredentialsForm';
import { DeleteButton } from 'e-teleport/SessionRecordings/setup/forms/DeleteButton';
import type { InferenceModelSchema } from 'e-teleport/SessionRecordings/setup/schema/model';
import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';

interface CreateInferenceModelFormProps {
  clusterId: string;
  error: unknown;
  isCloud: boolean;
  isCreate: true;
  isError: boolean;
  isPending: boolean;
  isValid: boolean;
  name?: never;
  onClose(): void;
}

interface EditInferenceModelFormProps extends Omit<
  CreateInferenceModelFormProps,
  'isCreate' | 'name'
> {
  isCreate: false;
  name: string;
}

type InferenceModelFormProps =
  | CreateInferenceModelFormProps
  | EditInferenceModelFormProps;

export function InferenceModelForm({
  clusterId,
  error,
  isCloud,
  isCreate,
  isError,
  isPending,
  isValid,
  name,
  onClose,
}: InferenceModelFormProps) {
  const queryClient = useQueryClient();

  const [failedMessage, setFailedMessage] = useState<string | null>(null);
  const [testStatus, setTestStatus] = useState(TestStatus.Idle);

  const { clearPendingSelection, pendingSelection } =
    useSessionSummariesManagement();

  const form = useFormContext();

  const [previousValues, setPreviousValues] = useState<null | string>(null);

  const values = useWatch<InferenceModelSchema>({
    name: [
      'accessMethod',
      'apiUrl',
      'apiKey',
      'secretName',
      'region',
      'bedrockMode',
      'model',
    ],
  });

  const currentValues = JSON.stringify(values);

  if (previousValues !== currentValues) {
    setTestStatus(TestStatus.Idle);
    setFailedMessage(null);
    setPreviousValues(currentValues);
  }

  const deleteModel = useDeleteInferenceModel({
    onSuccess() {
      const queryKey = listInferenceModelsQueryKey({
        clusterId,
        limit: 10,
      });

      void queryClient.refetchQueries({ queryKey });
    },
  });

  const handleDeleteModel = useCallback(async () => {
    try {
      await deleteModel.mutateAsync({
        clusterId,
        name,
      });

      onClose();
    } catch {
      // handled by TanStack Query
    }
  }, [deleteModel, clusterId, name, onClose]);

  useEffect(() => {
    if (pendingSelection?.entity !== OverlayEntity.Secret) {
      return;
    }

    void queryClient.invalidateQueries({
      queryKey: listInferenceSecretsQueryKey({ clusterId }),
    });

    form.setValue('secretName', pendingSelection.name, {
      shouldValidate: true,
    });

    clearPendingSelection();
  }, [pendingSelection, clearPendingSelection, form, clusterId, queryClient]);

  return (
    <>
      <DialogContent>
        {isError && <Alert kind="danger">{getErrorMessage(error)}</Alert>}
        {deleteModel.isError && (
          <Alert kind="danger">{getErrorMessage(deleteModel.error)}</Alert>
        )}
        <Flex flexDirection="column" gap={4} width="100%">
          <FieldModelProvider />

          <CredentialsForm isCloud={isCloud} />

          {isCreate && (
            <FieldInput
              helperText="Enter a unique name for this inference model."
              label="Name"
              name="name"
              placeholder="my-inference-model"
              required={true}
            />
          )}
          {failedMessage && (
            <Alert kind="danger">Test connection failed: {failedMessage}</Alert>
          )}
        </Flex>
      </DialogContent>

      <DialogFooter>
        <Flex alignItems="center" width="100%" gap={3}>
          <HoverTooltip
            tipContent={
              isCreate
                ? 'Test the connection before creating the model'
                : 'Test the connection before updating the model'
            }
            disabled={testStatus === TestStatus.Success}
          >
            <ButtonPrimary
              disabled={
                !isValid || isPending || testStatus !== TestStatus.Success
              }
              type="submit"
            >
              {isCreate ? 'Create' : 'Update'}
            </ButtonPrimary>
          </HoverTooltip>

          <TestConnectionButton
            onFailedMessageChange={setFailedMessage}
            onTestStatusChange={setTestStatus}
            testStatus={testStatus}
          />
          <ButtonSecondary disabled={isPending} onClick={onClose} type="button">
            Cancel
          </ButtonSecondary>
          <Spacer />
          {!isCreate && (
            <DeleteButton
              buttonText="Delete Inference Model"
              confirmText={
                <Box>
                  Are you sure you want to delete the inference model{' '}
                  <strong>{name}</strong>?
                </Box>
              }
              headerText="Confirm Delete Inference Model"
              isPending={deleteModel.isPending}
              onDelete={handleDeleteModel}
            />
          )}
        </Flex>
      </DialogFooter>
    </>
  );
}

const Spacer = styled.div`
  flex: 1;
`;
