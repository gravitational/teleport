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
import { ActionMeta } from 'react-select';
import styled from 'styled-components';

import { Box } from 'design';
import { FieldSelectAsync } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';

import { CheckableOptionComponent } from '../CheckableOption';
import { PendingKubeResourceItem, PendingListItem } from './RequestCheckout';

export function KubeNamespaceSelector({
  kubeClusterItem,
  fetchKubeNamespaces,
  savedResourceItems,
  updateNamespacesForKubeCluster,
}: {
  kubeClusterItem: PendingListItem;
  fetchKubeNamespaces(
    search: string,
    kubeCluster: PendingListItem
  ): Promise<string[]>;
  savedResourceItems: PendingListItem[];
  updateNamespacesForKubeCluster: (
    resources: PendingKubeResourceItem[],
    resource: PendingListItem
  ) => void;
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
      resource.kind === 'namespace' &&
      resource.id === kubeClusterItem.id &&
      resource.clusterName === kubeClusterItem.clusterName
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
        updateNamespacesForKubeCluster([], kubeClusterItem);
        return;
      case 'remove-value':
        const selectedOptions = options || [];
        updateNamespacesForKubeCluster(
          selectedOptions.map(o => optionToKubeNamespace(o)),
          kubeClusterItem
        );
        return;
    }
  }

  const handleMenuClose = () => {
    const selectedOptions = selectedOpts || [];
    setIsMenuOpen(false);
    updateNamespacesForKubeCluster(
      selectedOptions.map(o => optionToKubeNamespace(o)),
      kubeClusterItem
    );
  };

  function optionToKubeNamespace(
    selectedOption: Option
  ): PendingKubeResourceItem {
    const namespace = selectedOption.value;
    return {
      kind: 'namespace',
      id: kubeClusterItem.id,
      subResourceName: namespace,
      clusterName: kubeClusterItem.clusterName,
      name: namespace,
    };
  }

  async function handleLoadOptions(input: string) {
    const namespaces = await fetchKubeNamespaces(input, kubeClusterItem);

    return namespaces.map(namespace => ({
      kind: 'namespace',
      value: namespace,
      label: namespace,
    }));
  }

  return (
    <Box width="100%" mb={-3} mt={2}>
      <StyledSelect
        label={`Namespaces`}
        inputId={`${kubeClusterItem.id}-${kubeClusterItem.clusterName}`}
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
