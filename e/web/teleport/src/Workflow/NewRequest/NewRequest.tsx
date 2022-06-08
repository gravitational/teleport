import React, { useState } from 'react';
import { Prompt } from 'react-router';
import { Transition } from 'react-transition-group';
import {
  Indicator,
  Box,
  Flex,
  ButtonPrimary,
  ButtonSecondary,
  Text,
} from 'design';
import { StyledPanel } from 'design/DataTable/StyledTable';
import { StyledArrowBtn } from 'design/DataTable/Pager/StyledPager';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';
import Select from 'shared/components/Select';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import useTeleportE from 'e-teleport/useTeleportE';
import { SearchPanel } from './SearchPanel';
import { ResourceList } from './ResourceList';
import { RequestCheckout } from './RequestCheckout';
import { useNewRequest, State, ResourceKind } from './useNewRequest';

const agentOptions: ResourceOption[] = [
  {
    value: 'app',
    label: 'applications',
  },
  {
    value: 'db',
    label: 'databases',
  },
  {
    value: 'windows_desktop',
    label: 'desktops',
  },
  {
    value: 'kube_cluster',
    label: 'kubernetes',
  },
  // Order matters. On initial render
  // the last element in the options array
  // will be used. Which can either be 'node' or 'role'.
  {
    value: 'node',
    label: 'servers',
  },
];

// roleOption is separate because role based
// and search based access requests cannot
// be combined.
const roleOption: ResourceOption = {
  value: 'role',
  label: 'roles',
};

export default function Container() {
  const teleCtx = useTeleportE();
  const state = useNewRequest(teleCtx);

  return <NewRequest {...state} />;
}

export function NewRequest(props: State) {
  const {
    isLeafCluster,
    attempt,
    agents,
    agentFilter,
    updateQuery,
    updateSearch,
    fetchStatus,
    onAgentLabelClick,
    selectedResource,
    addedResources,
    addOrRemoveResource,
    pageSize,
    pageCount,
    customSort,
    nextPage,
    prevPage,
    updateResourceKind,
    clearAddedResources,
    requestableRoles,
  } = props;

  const [showCheckout, setShowCheckout] = useState(false);

  // warningConfirm holds the next resource option that will be applied
  // after user agrees to the warning dialogue.
  const [warningConfirm, setWarningConfirm] = useState<ResourceOption>();

  // Role based access requests are only allowed in root cluster.
  const resourceOptions = [...agentOptions];
  if (!isLeafCluster) {
    resourceOptions.push(roleOption);
  }

  // Load the last option which is either:
  //  - option role if at a root cluster
  //  - option node if at a leaf cluster
  const [currResourceOpt, setCurrResourceOpt] = useState(
    resourceOptions[resourceOptions.length - 1]
  );

  const numAddedAgents =
    Object.keys(addedResources.node).length +
    Object.keys(addedResources.db).length +
    Object.keys(addedResources.app).length +
    Object.keys(addedResources.kube_cluster).length +
    Object.keys(addedResources.windows_desktop).length;

  const numAddedRoles = Object.keys(addedResources.role).length;

  const numAddedResources = numAddedAgents + numAddedRoles;

  // 'confirmed' parameter is only true when user agrees to the warning dialogue.
  function handleOnChangeResourceOption(o: ResourceOption, confirmed = false) {
    // Warn users when user is switching between search based requests (AgentKinds) and
    // role based requests when items were selected.
    if (
      !confirmed &&
      ((o.value === 'role' && numAddedAgents > 0) ||
        (o.value !== 'role' && numAddedRoles > 0))
    ) {
      setWarningConfirm(o);
      return;
    }

    setCurrResourceOpt(o);
    updateResourceKind(o.value);
  }

  /* This is a warning prompt when user switches between role based and search based requests. */
  if (warningConfirm) {
    const msg = `Search based access request cannot be combined with role based access request. Current items selected will be cleared. Are you sure you want to continue?`;
    if (window.confirm(msg)) {
      clearAddedResources();
      handleOnChangeResourceOption(warningConfirm, true);
    }
    setWarningConfirm(null);
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>New Request</FeatureHeaderTitle>
      </FeatureHeader>
      <Box>
        {attempt.status === 'failed' && (
          <ErrorMessage message={attempt.statusText} />
        )}
        {attempt.status !== 'processing' && (
          <Box width="150px" mb={4} data-testid="resource-selector">
            <Select
              value={currResourceOpt}
              options={resourceOptions}
              onChange={o => handleOnChangeResourceOption(o as ResourceOption)}
              isDisabled={fetchStatus === 'loading'}
            />
          </Box>
        )}
        {attempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
        {attempt.status !== 'processing' && (
          <>
            <SearchPanel
              updateQuery={updateQuery}
              updateSearch={updateSearch}
              pageCount={pageCount}
              filter={agentFilter}
              showSearchBar={currResourceOpt.value !== 'role'}
              disableSearch={fetchStatus === 'loading'}
            />
            <ResourceList
              agents={agents}
              selectedResource={selectedResource}
              pageSize={pageSize}
              customSort={customSort}
              onLabelClick={onAgentLabelClick}
              addedResources={addedResources}
              addOrRemoveResource={addOrRemoveResource}
              requestableRoles={requestableRoles}
              disableRows={fetchStatus === 'loading'}
            />
            <StyledPanel borderBottomLeftRadius={3} borderBottomRightRadius={3}>
              <Flex justifyContent="flex-end" width="100%">
                <Flex alignItems="center" mr={2}></Flex>
                <Flex>
                  <StyledArrowBtn
                    onClick={prevPage}
                    title="Previous page"
                    disabled={!prevPage || fetchStatus === 'loading'}
                    mx={0}
                  >
                    <CircleArrowLeft fontSize="3" />
                  </StyledArrowBtn>
                  <StyledArrowBtn
                    ml={0}
                    onClick={nextPage}
                    title="Next page"
                    disabled={!nextPage || fetchStatus === 'loading'}
                  >
                    <CircleArrowRight fontSize="3" />
                  </StyledArrowBtn>
                </Flex>
              </Flex>
            </StyledPanel>
            <Flex
              data-testid="checkout-footer"
              alignItems="center"
              justifyContent="space-between"
              borderRadius={3}
              p={3}
              mt={5}
              css={`
                background: ${({ theme }) => theme.colors.primary.main};
              `}
            >
              <Text bold>Resources Added ({numAddedResources})</Text>
              <Box>
                {numAddedResources > 0 && (
                  <ButtonSecondary
                    mr={3}
                    width="165px"
                    onClick={() => clearAddedResources()}
                  >
                    Clear Selections
                  </ButtonSecondary>
                )}
                <ButtonPrimary
                  width="182px"
                  onClick={() => setShowCheckout(true)}
                  disabled={
                    numAddedResources === 0 || fetchStatus === 'loading'
                  }
                >
                  Proceed to Request
                </ButtonPrimary>
              </Box>
            </Flex>
          </>
        )}
        <Transition in={showCheckout} timeout={300} mountOnEnter unmountOnExit>
          {transitionState => (
            <RequestCheckout
              addedResources={addedResources}
              onClose={() => setShowCheckout(false)}
              toggleResource={addOrRemoveResource}
              transitionState={transitionState}
              reset={clearAddedResources}
              selectedResource={selectedResource}
            />
          )}
        </Transition>
        {/* This is a react-router provided prompt when it detects route change.
         * Used when user navigates away or changes cluster (which changes the route).
         */}
        <Prompt
          when={numAddedResources > 0}
          message={location => {
            if (location.pathname.endsWith('/requests/new')) {
              return `Resources from different clusters cannot be combined in an access request. Current items selected will be cleared. Are you sure you want to continue?`;
            } else {
              return `${numAddedResources} item(s) selected for a new access request will be cleared if you leave this page. Are you sure you want to continue?`;
            }
          }}
        />
      </Box>
    </FeatureBox>
  );
}

type ResourceOption = {
  value: ResourceKind;
  label: string;
};
