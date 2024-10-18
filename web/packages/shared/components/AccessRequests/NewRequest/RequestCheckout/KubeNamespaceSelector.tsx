/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useState } from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import { ActionMeta } from 'react-select';

import { Option } from 'shared/components/Select';
import { FieldSelectAsync } from 'shared/components/FieldSelect';

import { requiredField } from 'shared/components/Validation/rules';

import { CheckableOptionComponent } from '../CheckableOption';

import { PendingListItem, PendingKubeResourceItem } from './RequestCheckout';

import type { KubeNamespaceRequest } from '../kube';

export function KubeNamespaceSelector({
  kubeClusterItem,
  fetchKubeNamespaces,
  savedResourceItems,
  toggleResource,
  bulkToggleKubeResources,
  namespaceRequired,
}: {
  kubeClusterItem: PendingListItem;
  fetchKubeNamespaces(p: KubeNamespaceRequest): Promise<Option[]>;
  savedResourceItems: PendingListItem[];
  toggleResource: (resource: PendingListItem) => void;
  bulkToggleKubeResources: (
    resources: PendingKubeResourceItem[],
    resource: PendingListItem
  ) => void;
  namespaceRequired: boolean;
}) {
  // Flag is used to determine if we need to perform batch action
  // eg: When menu is open, we want to apply changes only after
  // user closes the menu. Actions performed when menu is closed
  // requires immediate changes such as clicking on delete or clear
  // all button.
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  // This is required to support loading options after a user has
  // clicked open a dropdown, and supports saving this initial
  // options for future (clicking the dropdown again).
  const [initOptions, setInitOptions] = useState<Option[]>([]);

  const currKubeClustersNamespaceItems = savedResourceItems.filter(
    resource =>
      resource.kind === 'namespace' && resource.id === kubeClusterItem.id
  ) as PendingKubeResourceItem[];

  const [selectedOpts, setSelectedOpts] = useState<Option[]>(() =>
    currKubeClustersNamespaceItems.map(namespace => ({
      label: namespace.subResourceName,
      value: namespace.subResourceName,
    }))
  );

  function handleChange(options: Option[], actionMeta: ActionMeta<Option>) {
    if (isMenuOpen) {
      setSelectedOpts(options);
      return;
    }

    switch (actionMeta.action) {
      case 'clear':
        bulkToggleKubeResources(
          currKubeClustersNamespaceItems,
          kubeClusterItem
        );
        return;
      case 'remove-value':
        toggleResource({
          kind: 'namespace',
          id: kubeClusterItem.id,
          subResourceName: actionMeta.removedValue.value,
          clusterName: kubeClusterItem.clusterName,
          name: actionMeta.removedValue.value,
        });
        return;
    }
  }

  const handleMenuClose = () => {
    setIsMenuOpen(false);

    const currNamespaces = currKubeClustersNamespaceItems.map(
      n => n.subResourceName
    );
    const selectedNamespaceIds = selectedOpts.map(o => o.value);
    const toKeep = selectedNamespaceIds.filter(id =>
      currNamespaces.includes(id)
    );

    const toInsert = selectedNamespaceIds.filter(o => !toKeep.includes(o));
    const toRemove = currNamespaces.filter(n => !toKeep.includes(n));

    if (!toInsert.length && !toRemove.length) {
      return;
    }

    bulkToggleKubeResources(
      [...toRemove, ...toInsert].map(namespace => ({
        kind: 'namespace',
        id: kubeClusterItem.id,
        subResourceName: namespace,
        clusterName: kubeClusterItem.clusterName,
        name: namespace,
      })),
      kubeClusterItem
    );
  };

  async function handleLoadOptions(input: string) {
    const options = await fetchKubeNamespaces({
      kubeCluster: kubeClusterItem.id,
      search: input,
    });

    return options;
  }

  return (
    <Box width="100%" mb={-3}>
      <StyledSelect
        label={`Namespaces ${namespaceRequired ? '(required)' : ''}:`}
        inputId={kubeClusterItem.id}
        width="100%"
        placeholder="Start typing a namespace and press enter"
        isMulti
        isClearable={false}
        isSearchable
        closeMenuOnSelect={false}
        hideSelectedOptions={false}
        onMenuClose={handleMenuClose}
        onMenuOpen={() => setIsMenuOpen(true)}
        components={{
          Option: CheckableOptionComponent,
        }}
        loadOptions={handleLoadOptions}
        onChange={handleChange}
        value={selectedOpts}
        menuPosition="fixed" /* required to render dropdown out of its row */
        rule={
          namespaceRequired
            ? requiredField('namespace selection required')
            : undefined
        }
        initOptionsOnMenuOpen={(opts: Option[]) => setInitOptions(opts)}
        defaultOptions={initOptions}
      />
    </Box>
  );
}

const StyledSelect = styled(FieldSelectAsync)`
  input[type='checkbox'] {
    cursor: pointer;
  }

  .react-select__control {
    font-size: ${p => p.theme.fontSizes[1]}px;
    width: 350px;
    background: ${p => p.theme.colors.levels.elevated};

    &:hover {
      background: ${p => p.theme.colors.levels.elevated};
    }
  }

  .react-select__menu {
    font-size: ${p => p.theme.fontSizes[1]}px;
    width: 350px;
    right: 0;
    margin-bottom: 0;
  }

  .react-select__option {
    padding: 0;
    font-size: ${p => p.theme.fontSizes[1]}px;
  }

  .react-select__value-container {
    position: static;
  }

  .react-select__placeholder {
    color: ${p => p.theme.colors.text.main};
  }
`;
