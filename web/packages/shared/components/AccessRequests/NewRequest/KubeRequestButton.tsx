import React, { useState } from 'react';
import styled from 'styled-components';
import { ButtonPrimary, ButtonBorder, Flex } from 'design';

import { StyledSelect as BaseStyledSelect } from 'shared/components/Select/Select';
import { makeSuccessAttempt, useAsync } from 'shared/hooks/useAsync';
import {
  ResourceKind,
  ResourceMap,
} from 'shared/components/AccessRequests/NewRequest';
import {
  extractKubeNamspaceFromId,
  getKubeNamespaceId,
  KubeNamespaceRequest,
} from 'shared/components/AccessRequests/NewRequest/kube';
import {
  Option,
  RequestableResourceKind,
} from 'shared/components/AccessRequests/NewRequest/resource';
import Validation from 'shared/components/Validation';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';
import { FieldSelectAsync } from 'shared/components/FieldSelect';

import { CheckableOption } from './CheckableOption';
import {
  extractResourceRequestProperties,
  ResourceRequest,
  toResourceRequest,
} from 'teleterm/ui/services/workspacesService/accessRequestsService';
import { RequestButton } from 'e-teleport/Workflow/NewRequest/RequestButton';

export function KubeRequestButton({
  resource,
  addOrRemoveResource,
  fetchNamespaces,
  RequestButton,
  addedResources,
  clusterUri,
}: {
  resource: ResourceRequest;

  addOrRemoveResource: (p: ResourceRequest) => void;
  fetchNamespaces(r: KubeNamespaceRequest): Promise<Option[]>;
  RequestButton(p: { onClick: () => any }): JSX.Element;
  addedResources: Map<string, ResourceRequest>;
  clusterUri: string;
}) {
  const { id: currKubeCluster } = extractResourceRequestProperties(resource);

  const [showSelector, setShowSelector] = useState(false);

  const selectorRef = useRefClickOutside<HTMLDivElement>({
    open: showSelector,
    setOpen: setShowSelector,
  });

  const [
    namespacesFetchAttempt,
    runInitNamespacesFetchAttempt,
    setNamespacesFetchAttempt,
  ] = useAsync(async () => {
    try {
      const options = await fetchNamespaces({
        search: '',
        kubeCluster: currKubeCluster,
      });
      setShowSelector(true);
      return options;
    } catch (err) {
      // Default to just adding the kube_cluster
      // instead of showing namespace errors.
      // The error will most likely from kube perm issues:
      //  - missing kubernetes_groups or kubernetes_users
      //  - more than one kubernetes_users (impersonation error)
      //  - incorrect kubernetes_groups
      addOrRemoveResource(resource);
      throw new Error(err);
    }
  });

  const namespacesMap: Record<string, ResourceRequest> = {};
  addedResources.forEach(val => {
    if (val.kind === 'namespace') {
      const { name: kubeCluster } = extractResourceRequestProperties(val);
      if (kubeCluster === currKubeCluster) {
        namespacesMap[val.resource.uri] = val;
      }
    }
  });

  const currNamespaceIds = Object.keys(namespacesMap);
  const addedNamespaces = currNamespaceIds.length;

  if (namespacesFetchAttempt.status === '') {
    return <RequestButton onClick={runInitNamespacesFetchAttempt} />;
  }

  if (namespacesFetchAttempt.status === 'error' && !addedNamespaces) {
    return <RequestButton onClick={() => addOrRemoveResource(resource)} />;
  }

  if (namespacesFetchAttempt.status === 'processing') {
    return '...loading';
  }

  const selectedValues: Option[] = currNamespaceIds.map(namespaceId => {
    const { name: kubeCluster, id: namespace } =
      extractResourceRequestProperties(namespacesMap[namespaceId]);

    return {
      label: getKubeNamespaceId({ namespace, kubeCluster }),
      value: namespaceId,
      isAdded: true,
      kind: 'namespace',
    };
  });

  function handleSelect(options: Option[]) {
    const selectedNamespaceIds = options.map(o => o.value);
    const toKeep = selectedNamespaceIds.filter(id =>
      currNamespaceIds.includes(id)
    );

    const toInsert = selectedNamespaceIds.filter(o => !toKeep.includes(o));
    const toRemove = currNamespaceIds.filter(n => !toKeep.includes(n));

    [...toInsert, ...toRemove].forEach(id => {
      const { id: namespace, name: kubeCluster } =
        extractResourceRequestProperties({
          kind: 'namespace',
          resource: { uri: id },
        });

      addOrRemoveResource(
        toResourceRequest({
          kind: 'namespace',
          clusterUri,
          resourceId: namespace,
          resourceName: kubeCluster,
        })
      );
    });
  }

  return (
    <Validation>
      {addedNamespaces ? (
        <ButtonPrimary
          size="small"
          onClick={() => setShowSelector(!showSelector)}
          fill="filled"
        >
          Edit Namespaces
        </ButtonPrimary>
      ) : (
        <ButtonBorder
          size="small"
          onClick={() => setShowSelector(!showSelector)}
        >
          Select Namespaces
        </ButtonBorder>
      )}
      <Flex ref={selectorRef} mt={1} css={{ position: 'relative' }}>
        {showSelector && (
          <StyledSelect className={addedNamespaces ? 'hasSelectedGroups' : ''}>
            <FieldSelectAsync
              autoFocus
              inputId="roles"
              width="100%"
              placeholder="Start typing a namespace and press enter"
              noOptionsMessage={() =>
                'Start typing a namespace and press enter'
              }
              isMulti
              isClearable={false} // todo?
              isSearchable
              menuIsOpen={true}
              closeMenuOnSelect={false}
              hideSelectedOptions={false}
              components={{
                Option: CheckableOption,
              }}
              options={[]}
              loadOptions={async input => {
                const options = await fetchNamespaces({
                  kubeCluster: currKubeCluster,
                  search: input,
                });
                setNamespacesFetchAttempt(makeSuccessAttempt(options));
                return options;
              }}
              onChange={handleSelect}
              value={selectedValues}
              defaultOptions={namespacesFetchAttempt.data}
              menuPlacement="auto"
            />
          </StyledSelect>
        )}
      </Flex>
    </Validation>
  );
}

const StyledSelect = styled(BaseStyledSelect)`
  position: absolute;
  right: 0;
  top: 10px;
  input[type='checkbox'] {
    cursor: pointer;
  }

  .react-select__control {
    z-index: 1000;
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
