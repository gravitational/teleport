import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, type MouseEvent } from 'react';
import { useFormContext, useWatch } from 'react-hook-form';
import { Link } from 'react-router';
import type { SingleValueProps } from 'react-select';
import styled from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import { DialogContent, DialogFooter } from 'design/Dialog';
import Flex from 'design/Flex';
import { Database, Desktop, Kubernetes, Server } from 'design/Icon';
import type { Option } from 'shared/components/Select';
import { getErrorMessage } from 'shared/utils/error';

import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import {
  listInferencePoliciesQueryKey,
  useDeleteInferencePolicy,
  useListInferenceModels,
} from 'e-teleport/services/inference/hooks';
import { FieldCheckboxGroup } from 'e-teleport/SessionRecordings/setup/fields/FieldCheckboxGroup';
import { FieldInput } from 'e-teleport/SessionRecordings/setup/fields/FieldInput';
import { FieldSelect } from 'e-teleport/SessionRecordings/setup/fields/FieldSelect';
import { DeleteButton } from 'e-teleport/SessionRecordings/setup/forms/DeleteButton';

interface CreateInferencePolicyFormProps {
  clusterId: string;
  error: unknown;
  isCreate: true;
  isError: boolean;
  isPending: boolean;
  isValid: boolean;
  name?: never;
  onClose(): void;
}

interface EditInferencePolicyFormProps extends Omit<
  CreateInferencePolicyFormProps,
  'isCreate' | 'name'
> {
  isCreate: false;
  name: string;
}

type InferencePolicyFormProps =
  | CreateInferencePolicyFormProps
  | EditInferencePolicyFormProps;

export function InferencePolicyForm({
  clusterId,
  error,
  isCreate,
  isError,
  isPending,
  isValid,
  name,
  onClose,
}: InferencePolicyFormProps) {
  const queryClient = useQueryClient();

  const { createNewOverlayLink, clearPendingSelection, pendingSelection } =
    useSessionSummariesManagement();

  const form = useFormContext();

  const deletePolicy = useDeleteInferencePolicy({
    onSuccess() {
      const queryKey = listInferencePoliciesQueryKey({
        clusterId,
        limit: 10,
      });

      void queryClient.refetchQueries({ queryKey });
    },
  });

  const handleDeletePolicy = useCallback(async () => {
    try {
      await deletePolicy.mutateAsync({
        clusterId,
        name,
      });

      onClose();
    } catch {
      // handled by TanStack Query
    }
  }, [deletePolicy, clusterId, name, onClose]);

  const inferenceModels = useListInferenceModels({ clusterId });

  useEffect(() => {
    if (pendingSelection?.entity !== OverlayEntity.Model) {
      return;
    }

    void inferenceModels.refetch();

    form.setValue('model', pendingSelection.name, { shouldValidate: true });

    clearPendingSelection();
  }, [
    pendingSelection,
    clearPendingSelection,
    form,
    clusterId,
    queryClient,
    inferenceModels,
  ]);

  const options = useMemo(
    () =>
      inferenceModels.data?.items?.map(model => ({
        value: model.name,
        label: model.name,
      })) ?? [],
    [inferenceModels.data?.items]
  );

  const addInferenceModelLink = useMemo(
    () => createNewOverlayLink(OverlayEntity.Model),
    [createNewOverlayLink]
  );

  return (
    <>
      <DialogContent>
        {isError && <Alert kind="danger">{getErrorMessage(error)}</Alert>}
        {deletePolicy.isError && (
          <Alert kind="danger">{getErrorMessage(deletePolicy.error)}</Alert>
        )}

        <Flex flexDirection="column" gap={4} width="100%">
          <FieldSelect
            name="model"
            components={{
              SingleValue: CustomSingleValue,
            }}
            options={options}
            label="Inference model to use"
            labelButton={
              <AddLink to={addInferenceModelLink}>
                Add a new inference model
              </AddLink>
            }
            required={true}
          />

          <FieldCheckboxGroup
            name="kinds"
            label="What types of sessions should be summarized?"
            options={[
              {
                label: 'Kubernetes',
                value: 'k8s',
                icon: <Kubernetes size="small" />,
              },
              {
                label: 'Database',
                value: 'db',
                icon: <Database size="small" />,
              },
              { label: 'SSH', value: 'ssh', icon: <Server size="small" /> },
              {
                label: 'Desktop',
                value: 'desktop',
                icon: <Desktop size="small" />,
              },
            ]}
            required={true}
          />

          {isCreate && (
            <FieldInput
              helperText="Enter a unique name for this inference policy."
              label="Name"
              name="name"
              placeholder="my-inference-policy"
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
              buttonText="Delete Inference Policy"
              confirmText={
                <Box>
                  Are you sure you want to delete the inference policy{' '}
                  <strong>{name}</strong>?
                </Box>
              }
              headerText="Confirm Delete Inference Policy"
              isPending={deletePolicy.isPending}
              onDelete={handleDeletePolicy}
            />
          )}
        </Flex>
      </DialogFooter>
    </>
  );
}

const SingleValueContainer = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  grid-area: 1 / 1 / 2 / 3;
  margin-inline-end: 0.125rem;
  margin-inline-start: 0.125rem;
`;

function CustomSingleValue(props: SingleValueProps<Option>) {
  const model = useWatch<{ model: string }>({ name: 'model' });

  const { openEditOverlay } = useSessionSummariesManagement();

  const handleEditModel = useCallback(
    (event: MouseEvent) => {
      event.preventDefault();
      event.stopPropagation();

      openEditOverlay(OverlayEntity.Model, model);
    },
    [model, openEditOverlay]
  );

  return (
    <SingleValueContainer {...props.innerProps}>
      <Box>{props.data.label}</Box>

      <Spacer />

      <EditModel
        onClick={handleEditModel}
        onMouseDown={e => {
          e.preventDefault();
          e.stopPropagation();
        }}
      >
        Edit Model
      </EditModel>
    </SingleValueContainer>
  );
}

const EditModel = styled.div`
  color: ${p => p.theme.colors.brand};
  cursor: pointer;
  font-size: ${p => p.theme.fontSizes[0]};
  line-height: 1;
  position: relative;
  z-index: 100;

  &:hover {
    text-decoration: underline;
  }
`;

const AddLink = styled(Link)`
  color: ${p => p.theme.colors.brand};
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
`;

const Spacer = styled.div`
  flex: 1;
`;
