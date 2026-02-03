import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import styled from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import { DialogContent, DialogFooter } from 'design/Dialog';
import Flex from 'design/Flex';
import { getErrorMessage } from 'shared/utils/error';

import { FieldAPIKey } from 'e-teleport/Integrations/SessionSummaries/fields/FieldAPIKey';
import { FieldInput } from 'e-teleport/Integrations/SessionSummaries/fields/FieldInput';
import { DeleteButton } from 'e-teleport/Integrations/SessionSummaries/forms/DeleteButton';
import {
  listInferenceSecretsQueryKey,
  useDeleteInferenceSecret,
} from 'e-teleport/services/inference/hooks';

interface CreateInferenceSecretFormProps {
  clusterId: string;
  error: unknown;
  isCreate: true;
  isError: boolean;
  isPending: boolean;
  isValid: boolean;
  name?: never;
  onClose(): void;
}

interface EditInferenceSecretFormProps extends Omit<
  CreateInferenceSecretFormProps,
  'isCreate' | 'name'
> {
  isCreate: false;
  name: string;
}

type InferenceSecretFormProps =
  | CreateInferenceSecretFormProps
  | EditInferenceSecretFormProps;

export function InferenceSecretForm({
  clusterId,
  error,
  isCreate,
  isError,
  isPending,
  isValid,
  name,
  onClose,
}: InferenceSecretFormProps) {
  const queryClient = useQueryClient();

  const deleteSecret = useDeleteInferenceSecret({
    onSuccess() {
      const queryKey = listInferenceSecretsQueryKey({
        clusterId,
        limit: 10,
      });

      void queryClient.refetchQueries({ queryKey });
    },
  });

  const handleDeleteSecret = useCallback(async () => {
    try {
      await deleteSecret.mutateAsync({
        clusterId,
        name,
      });

      onClose();
    } catch {
      // handled by TanStack Query
    }
  }, [deleteSecret, clusterId, name, onClose]);

  return (
    <>
      <DialogContent>
        {isError && <Alert kind="danger">{getErrorMessage(error)}</Alert>}
        {deleteSecret.isError && (
          <Alert kind="danger">{getErrorMessage(deleteSecret.error)}</Alert>
        )}

        <Flex flexDirection="column" gap={4} width="100%">
          <FieldAPIKey
            autoFocus={true}
            label={isCreate ? 'API Key' : 'Update API Key'}
            helperText={
              isCreate
                ? undefined
                : 'This will replace the existing API key with the new value.'
            }
          />

          {isCreate && (
            <FieldInput
              helperText="Enter a unique name for this inference secret."
              label="Name"
              name="name"
              placeholder="my-inference-secret"
              required={true}
            />
          )}
        </Flex>
      </DialogContent>

      <DialogFooter>
        <Flex alignItems="center" width="100%">
          <ButtonPrimary mr={3} disabled={!isValid || isPending} type="submit">
            {isCreate ? 'Create' : 'Update'}
          </ButtonPrimary>

          <ButtonSecondary disabled={isPending} onClick={onClose} type="button">
            Cancel
          </ButtonSecondary>

          <Spacer />

          {!isCreate && (
            <DeleteButton
              buttonText="Delete Inference Secret"
              confirmText={
                <Box>
                  Are you sure you want to delete the inference secret{' '}
                  <strong>{name}</strong>?
                </Box>
              }
              headerText="Confirm Delete Inference Secret"
              isPending={deleteSecret.isPending}
              onDelete={handleDeleteSecret}
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
