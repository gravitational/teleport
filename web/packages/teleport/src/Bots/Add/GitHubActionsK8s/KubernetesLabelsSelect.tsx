/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query';
import { ReactNode, useEffect, useMemo, useState } from 'react';
import styled from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box, { BoxProps } from 'design/Box/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button/Button';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import { Dialog } from 'design/Dialog/Dialog';
import DialogContent from 'design/Dialog/DialogContent';
import DialogFooter from 'design/Dialog/DialogFooter';
import DialogHeader from 'design/Dialog/DialogHeader';
import DialogTitle from 'design/Dialog/DialogTitle';
import Flex from 'design/Flex/Flex';
import { CheckThick } from 'design/Icon/Icons/CheckThick';
import { Cross } from 'design/Icon/Icons/Cross';
import { Minus } from 'design/Icon/Icons/Minus';
import { Pencil } from 'design/Icon/Icons/Pencil';
import { Plus } from 'design/Icon/Icons/Plus';
import { Indicator } from 'design/Indicator/Indicator';
import { Primary, Secondary } from 'design/Label/Label';
import { LabelContent, LabelInput } from 'design/LabelInput/LabelInput';
import { ResourceIcon } from 'design/ResourceIcon';
import Text, { H3 } from 'design/Text';
import FieldInput, {
  HelperTextLine,
} from 'shared/components/FieldInput/FieldInput';
import Validation, { useRule, Validator } from 'shared/components/Validation';
import { Rule } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { ResourceLabel } from 'teleport/services/agents';
import { Kube } from 'teleport/services/kube';
import {
  IntegrationEnrollSection,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

import {
  KubernetesLabel,
  makeKubernetesAccessChecker,
} from '../Shared/kubernetes';
import { useTracking } from '../Shared/useTracking';

export function KubernetesLabelsSelect(
  props: {
    selected: KubernetesLabel[];
    onChange: (labels: KubernetesLabel[]) => void;
  } & BoxProps
) {
  const [showPicker, setShowPicker] = useState(false);
  const { selected, onChange, ...styles } = props;

  const { valid, message } = useRule(() => {
    if (selected.length === 0) {
      return {
        valid: false,
        message:
          'At least one label must be selected. Use wildcard *:* to match any label.',
      };
    }
    return {
      valid: true,
    };
  });

  const handleChanged = (labels: KubernetesLabel[]) => {
    setShowPicker(false);
    onChange(labels);
  };

  const handleCancel = () => {
    setShowPicker(false);
  };

  return (
    <Box {...styles}>
      <LabelInput mb={0} aria-label="Teleport Labels">
        <LabelContent>Teleport Labels</LabelContent>

        <LabelsContainer aria-describedby={'labels-select-helper-text'} mt={1}>
          {selected.map(l => (
            <Label key={l.name} label={formatLabel(l)} style="secondary" />
          ))}

          {selected.length === 0 && <EmptyText>No labels selected.</EmptyText>}

          <ButtonIcon
            onClick={() => {
              setShowPicker(true);
            }}
            aria-label="edit"
          >
            <Pencil size="medium" />
          </ButtonIcon>
        </LabelsContainer>
      </LabelInput>

      <HelperTextLine
        hasError={!valid}
        helperTextId={'labels-select-helper-text'}
        errorMessage={message}
      />

      {showPicker && (
        <Picker
          selected={selected}
          onChange={handleChanged}
          onCancel={handleCancel}
        />
      )}
    </Box>
  );
}

const LabelsContainer = styled(Flex)`
  flex-wrap: wrap;
  overflow: hidden;
  gap: ${props => props.theme.space[2]}px;
  align-items: center;
`;

const LabelText = styled(Text).attrs({
  typography: 'body2',
})`
  white-space: nowrap;
`;

function Picker(props: {
  selected: KubernetesLabel[];
  onChange: (labels: KubernetesLabel[]) => void;
  onCancel: () => void;
}) {
  const { selected: initial, onChange, onCancel } = props;

  const [selected, setSelected] = useState(initial);
  const [manualName, setManualName] = useState('');
  const [manualValue, setManualValue] = useState('');

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasListPermission = flags.kubernetes;

  const tracking = useTracking();

  // This effect sends a tracking event when the component mounts
  useEffect(() => {
    tracking.section(
      IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
      IntegrationEnrollSection.MWIGHAK8SKubernetesLabelPicker
    );
  }, [tracking]);

  const {
    data,
    isLoading,
    isFetchingNextPage,
    error,
    hasNextPage,
    fetchNextPage,
  } = useInfiniteQuery({
    enabled: hasListPermission,
    queryKey: [
      'list',
      'unified_resources',
      'paged',
      cfg.proxyCluster,
      ['kube_cluster'],
    ],
    queryFn: ({ pageParam, signal }) =>
      ctx.resourceService.fetchUnifiedResources(
        cfg.proxyCluster,
        {
          kinds: ['kube_cluster'],
          startKey: pageParam,
          limit: 20,
        },
        signal
      ),
    initialPageParam: '',
    getNextPageParam: data => data?.startKey || undefined,
    placeholderData: keepPreviousData,
    staleTime: 30_000, // Cached pages are valid for 30 seconds
  });

  const handleAdd = (name: string, value: string) => {
    setSelected(prev => {
      // Clear other labels when the wildcard is added
      if (isWildcardPair(name, value)) {
        return [WILDCARD_LABEL];
      }

      const index = prev.findIndex(l => l.name === name);
      let next: KubernetesLabel[];
      if (index == -1) {
        next = [...prev, { name, values: [value] }];
      } else {
        next = prev.map(l => {
          if (l.name === name) {
            const vs = new Set([...l.values, value]);
            return {
              ...l,
              values: [...vs],
            };
          }
          return l;
        });
      }

      return stripWildcard(next);
    });
  };

  const handleManualAdd = (validator: Validator) => {
    if (!validator.validate()) {
      return;
    }

    const name = manualName.trim();
    const value = manualValue.trim();
    handleAdd(name, value);

    validator.reset();
    setManualName('');
    setManualValue('');
  };

  const handleRemove = (name: string, value?: string) => {
    setSelected(prev => {
      let next: KubernetesLabel[];
      if (value === undefined) {
        next = prev.filter(l => l.name !== name);
      } else {
        next = prev.flatMap(l => {
          if (l.name === name) {
            l.values = l.values.filter(v => v != value);

            if (l.values.length < 1) {
              return [];
            }
          }

          return [l];
        });
      }

      // Add wildcard if all labels are removed
      return next.length === 0 ? [WILDCARD_LABEL] : next;
    });
  };

  const accessChecker = useMemo(
    () => makeKubernetesAccessChecker(selected),
    [selected]
  );

  const kubeClusters = data?.pages.flatMap(p =>
    p.agents.filter(a => a.kind === 'kube_cluster')
  );

  return (
    <Dialog open onClose={onCancel}>
      <DialogHeader>
        <Box width={'100%'}>
          <DialogTitle>Teleport Labels</DialogTitle>
          <Text>Select one or more labels to configure access.</Text>

          {!hasListPermission ? (
            <Alert mt={3} mb={0} kind="info">
              You do not have permission to view resources. Missing role
              permissions: <code>kube_server.read</code>,{' '}
              <code>kube_server.list</code>
            </Alert>
          ) : undefined}

          {error ? (
            <Alert mt={3} mb={0} kind="danger" details={error.message}>
              Failed to fetch resources
            </Alert>
          ) : undefined}
        </Box>
      </DialogHeader>

      <DialogContent
        minWidth={600}
        maxWidth={1024}
        minHeight={360}
        maxHeight={600}
      >
        <PickerContainer>
          <ColumnContainer>
            <ColumnHeading>Enrolled clusters</ColumnHeading>

            {isLoading ? (
              <Box data-testid="loading-resources" textAlign="center" m={10}>
                <Indicator />
              </Box>
            ) : undefined}

            {!isLoading && !kubeClusters?.length && (
              <Box p={3}>
                <EmptyText>No clusters found</EmptyText>
              </Box>
            )}

            <Flex overflow={'auto'} flexDirection={'column'} flex={1}>
              {kubeClusters?.map(item => (
                <ClusterItem
                  key={item.name}
                  resource={item}
                  selectedLabels={selected}
                  addLabel={handleAdd}
                  removeLabel={handleRemove}
                  checkAccess={accessChecker.check}
                />
              ))}

              {hasNextPage && (
                <LoadMoreContainer>
                  <ButtonSecondary
                    onClick={() => fetchNextPage()}
                    disabled={!hasNextPage || isFetchingNextPage}
                  >
                    Load More
                  </ButtonSecondary>
                </LoadMoreContainer>
              )}
            </Flex>
          </ColumnContainer>
          <ColumnContainer>
            <ColumnHeading>Selected Labels ({selected.length})</ColumnHeading>

            <Flex
              overflow={'auto'}
              flexDirection={'column'}
              flex={1}
              p={3}
              gap={2}
              alignItems={'flex-start'}
            >
              {selected.map(l => (
                <Label
                  key={l.name}
                  label={formatLabel(l)}
                  actionAriaLabel="remove"
                  actionIcon={<Cross size="small" />}
                  onAction={() => handleRemove(l.name)}
                  style="secondary"
                />
              ))}
            </Flex>
            <Validation>
              {({ validator }) => (
                <form
                  onSubmit={e => {
                    e.preventDefault();
                    handleManualAdd(validator);
                  }}
                >
                  <div>
                    <ManualInputContainer>
                      <FieldInput
                        flex={1}
                        m={0}
                        size="small"
                        rule={requireValidLabelName}
                        label="Name"
                        placeholder="e.g. env"
                        value={manualName}
                        onChange={e => setManualName(e.target.value)}
                      />
                      <FieldInput
                        flex={1}
                        m={0}
                        size="small"
                        rule={requireValidLabelValue}
                        label="Value"
                        placeholder="e.g. prod"
                        value={manualValue}
                        onChange={e => setManualValue(e.target.value)}
                        toolTipContent="e.g. prod, us-*, ^kube-(a|b).+$"
                      />
                      <ButtonIcon type="submit" aria-label="add label">
                        <Plus size="small" />
                      </ButtonIcon>
                    </ManualInputContainer>
                  </div>
                </form>
              )}
            </Validation>
          </ColumnContainer>
        </PickerContainer>
      </DialogContent>

      <DialogFooter>
        <Flex gap={3}>
          <ButtonPrimary onClick={() => onChange(selected)}>Done</ButtonPrimary>
          <ButtonSecondary onClick={onCancel}>Cancel</ButtonSecondary>
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}

const PickerContainer = styled(Flex)`
  flex: 1;
  overflow: auto;
  gap: ${({ theme }) => theme.space[3]}px;
`;

const ColumnContainer = styled(Flex)`
  min-width: 360px;
  flex: 1;
  flex-direction: column;
  overflow: auto;
  background-color: ${({ theme }) => theme.colors.levels.elevated};
  border-radius: ${({ theme }) => theme.radii[3]}px;
`;

const ColumnHeading = styled(H3)`
  padding: ${({ theme }) => theme.space[2] + theme.space[1]}px ${({ theme }) => theme.space[3]}px;
  border-bottom: 1px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[0]}};
`;

const ManualInputContainer = styled(Flex)`
  padding: ${({ theme }) => theme.space[3]}px;
  gap: ${({ theme }) => theme.space[2]}px;
  align-items: flex-end;
  border-top: 1px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[0]}};
`;

const LoadMoreContainer = styled(Flex)`
  justify-content: center;
  padding: ${props => props.theme.space[3]}px;
`;

const EmptyText = styled(Text)`
  color: ${p => p.theme.colors.text.muted};
`;

function formatLabel({ name, values }: KubernetesLabel) {
  if (values.length === 1) {
    return `${name}: ${values[0]}`;
  }
  return `${name}: ( ${values.join(' | ')} )`;
}

const WILDCARD_LABEL_NAME = '*';
const WILDCARD_LABEL_VALUE = '*';
const WILDCARD_LABEL: KubernetesLabel = {
  name: WILDCARD_LABEL_NAME,
  values: [WILDCARD_LABEL_VALUE],
};

function isWildcardPair(name: string, value: string) {
  return name === WILDCARD_LABEL_NAME && value === WILDCARD_LABEL_VALUE;
}

function stripWildcard(labels: KubernetesLabel[]) {
  return labels.filter(l => l.name !== WILDCARD_LABEL_NAME);
}

const labelNameRegex = /^([a-z0-9:]+|\*)$/; // 1-n alphanumerics or colon, or a single asterisk
export const requireValidLabelName: Rule = value => () => {
  const match = labelNameRegex.test(value.trim());
  return {
    valid: match,
    message: match ? undefined : 'Alphanumeric or * is required',
  };
};

export const requireValidLabelValue: Rule = value => () => {
  const trimmed = value.trim();
  return {
    valid: trimmed.length > 0,
    message: trimmed.length > 0 ? undefined : 'Value is required',
  };
};

function ClusterItem(props: {
  resource: Kube;
  selectedLabels: KubernetesLabel[];
  addLabel: (name: string, value: string) => void;
  removeLabel: (name: string, value: string) => void;
  checkAccess: (labels: ResourceLabel[]) => boolean;
}) {
  const { resource, selectedLabels, addLabel, removeLabel, checkAccess } =
    props;

  const hasAccess = checkAccess(resource.labels);

  return (
    <ClusterItemContainer data-testid={`cluster-item-${resource.name}`}>
      <div style={{ position: 'relative' }}>
        <ResourceIcon name="kube" size={40} />
        {hasAccess && (
          <AccessCheckBackground>
            <CheckThick size={'small'} />
          </AccessCheckBackground>
        )}
      </div>

      <ClusterItemInnerContainer>
        <ClusterItemNameContainer>
          <ClusterItemNameText>{resource.name}</ClusterItemNameText>
        </ClusterItemNameContainer>

        <ClusterItemLabelsContainer>
          {resource.labels.map(l => {
            const isSelected = selectedLabels.reduce((acc, cur) => {
              if (acc) {
                return true;
              }
              return cur.name === l.name && cur.values.some(v => v === l.value);
            }, false);
            return (
              <Label
                key={l.name}
                label={formatLabel({ name: l.name, values: [l.value] })}
                style="secondary"
                actionAriaLabel={isSelected ? 'remove' : 'add'}
                actionIcon={
                  isSelected ? (
                    <Minus size={'small'} />
                  ) : (
                    <Plus size={'small'} />
                  )
                }
                onAction={() =>
                  isSelected
                    ? removeLabel(l.name, l.value)
                    : addLabel(l.name, l.value)
                }
              />
            );
          })}

          {resource.labels.length === 0 && <EmptyText>No labels</EmptyText>}
        </ClusterItemLabelsContainer>
      </ClusterItemInnerContainer>
    </ClusterItemContainer>
  );
}

const ClusterItemContainer = styled(Flex)`
  align-items: flex-start;
  gap: ${({ theme }) => theme.space[2]}px;
  padding: ${({ theme }) => theme.space[2]}px ${({ theme }) => theme.space[3]}px;
  padding-left: ${({ theme }) => theme.space[2]}px;
`;

const ClusterItemInnerContainer = styled(Flex)`
  flex-direction: column;
`;

const ClusterItemNameContainer = styled(Flex)`
  align-items: center;
  min-height: ${({ theme }) => theme.space[5]}px;
`;

const ClusterItemNameText = styled(Text).attrs({
  typography: 'subtitle2',
})`
  white-space: nowrap;
`;

const ClusterItemLabelsContainer = styled(Flex)`
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space[2]}px;
`;

const AccessCheckBackground = styled.div`
  position: absolute;
  right: 0;
  bottom: 0;
  border-radius: 999px;
  background-color: ${({ theme }) =>
    theme.colors.interactive.solid.success.default};
  line-height: 0;
  color: white;
  border: 2px solid white;
`;

function Label(props: {
  label: string;
  style?: 'primary' | 'secondary';
  onAction?: () => void;
  actionIcon?: ReactNode;
  actionAriaLabel?: string;
}) {
  const {
    label,
    style = 'primary',
    onAction,
    actionIcon,
    actionAriaLabel,
  } = props;

  const Component = style === 'primary' ? Primary : Secondary;

  return (
    <Component p={0}>
      <Flex
        alignItems={'center'}
        gap={1}
        padding={1}
        paddingLeft={3}
        paddingRight={onAction ? 1 : 3}
      >
        <LabelText>{label}</LabelText>
        {onAction && (
          <ButtonIcon
            size={0}
            onClick={onAction}
            aria-label={actionAriaLabel}
            data-testid={`label-action-${label}-${actionAriaLabel}`}
          >
            {actionIcon}
          </ButtonIcon>
        )}
      </Flex>
    </Component>
  );
}
