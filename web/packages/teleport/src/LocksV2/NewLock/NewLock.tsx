/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useRef, useState } from 'react';
import { Prompt } from 'react-router';
import { Link } from 'react-router-dom';
import { Transition } from 'react-transition-group';

import { Box, ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import { ArrowBack } from 'design/Icon';
import Select from 'shared/components/Select';
import useAttempt from 'shared/hooks/useAttemptNext';

import ErrorMessage from 'teleport/components/AgentErrorMessage';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';

import {
  baseResourceKindOpts,
  CommonListProps,
  getEmptyResourceMap,
  LockResource,
  LockResourceMap,
  LockResourceOption,
} from './common';
import { LockCheckout } from './LockCheckout/LockCheckout';
import {
  HybridList,
  HybridListOpts,
} from './ResourceList/HybridList/HybridList';
import { Logins } from './ResourceList/Logins';
import { ServerSideSupportedList } from './ResourceList/ServerSideSupportedList/ServerSideSupportedList';
import {
  SimpleList,
  SimpleListOpts,
} from './ResourceList/SimpleList/SimpleList';

const PAGE_SIZE = 10;

export type Props = {
  customResourceKindOpts?: LockResourceOption[];
  simpleListOpts?: SimpleListOpts;
  hybridListOpts?: HybridListOpts;
};

export default function NewLock() {
  return NewLockView({});
}

export function NewLockView(props: Props) {
  const { attempt, setAttempt } = useAttempt('processing');
  const [showCheckout, setShowCheckout] = useState(false);
  const [selectedResourceOpt, setSelectedResourceOpt] = useState(
    props.customResourceKindOpts?.length
      ? props.customResourceKindOpts[0]
      : baseResourceKindOpts[0]
  );

  const [selectedResources, setSelectedResources] = useState<LockResourceMap>(
    getEmptyResourceMap()
  );

  function clearSelectedResources() {
    setSelectedResources(getEmptyResourceMap());
  }

  // toggleSelectResource adds to selection map if it doesn't exist,
  // else removes it from the map.
  function toggleSelectResource(resource: LockResource) {
    const { kind, targetValue, friendlyName } = resource;
    const newMap = copySelectedResources();
    if (newMap[kind][targetValue]) {
      delete newMap[kind][targetValue];
    } else {
      newMap[kind][targetValue] = friendlyName || targetValue;
    }

    setSelectedResources(newMap);
  }

  function copySelectedResources() {
    const copy = {} as LockResourceMap;
    const kinds = Object.keys(selectedResources);
    kinds.forEach(kind => (copy[kind] = { ...selectedResources[kind] }));

    return copy;
  }

  function batchDeleteResources(resources: LockResource[]) {
    const newMap = copySelectedResources();
    resources.forEach(r => {
      const { kind, targetValue } = r;

      if (newMap[kind][targetValue]) {
        delete newMap[kind][targetValue];
      }
    });
    setSelectedResources(newMap);
  }

  function updateResourceOption(newOpt: LockResourceOption) {
    setSelectedResourceOpt(newOpt);

    // There is no fetching for logins, so turn off the attempt state.
    if (newOpt.value === 'login') {
      setAttempt({ status: '' });
      return;
    }

    // All others will require fetching on init, so reset the
    // attempt state to processing.
    if (newOpt.listKind !== selectedResourceOpt.listKind) {
      setAttempt({ status: 'processing' });
    }
  }

  const selectedResourceKind = selectedResourceOpt.value;
  const commonListProps: CommonListProps = {
    pageSize: PAGE_SIZE,
    attempt,
    setAttempt,
    selectedResourceKind: selectedResourceKind,
    selectedResources: selectedResources,
    toggleSelectResource,
  };

  let content;
  switch (selectedResourceOpt.listKind) {
    case 'simple':
      content = <SimpleList {...commonListProps} opts={props.simpleListOpts} />;
      break;
    case 'hybrid':
      content = <HybridList {...commonListProps} opts={props.hybridListOpts} />;
      break;
    case 'logins':
      content = (
        <Logins
          pageSize={PAGE_SIZE}
          selectedResources={selectedResources}
          toggleSelectResource={toggleSelectResource}
        />
      );
      break;
    case 'server-side':
      content = <ServerSideSupportedList {...commonListProps} />;
      break;
    default:
      console.error(
        `[NewLock.tsx] listKind ${selectedResourceOpt.listKind} not defined`
      );
      return; // don't render anything on error
  }

  const numAddedResources = getNumSelectedResources(selectedResources);
  const transitionRef = useRef<HTMLDivElement>();

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          <Flex alignItems="center">
            <ArrowBack
              as={Link}
              size="large"
              mr={2}
              title="Go Back"
              to={cfg.getLocksRoute()}
              style={{ cursor: 'pointer', textDecoration: 'none' }}
            />
            <Box>New Lock Request</Box>
          </Flex>
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Box>
        {attempt.status === 'failed' && (
          <ErrorMessage message={attempt.statusText} />
        )}
        <Box width="180px" mb={4} data-testid="resource-selector">
          <Select
            value={selectedResourceOpt}
            options={props.customResourceKindOpts || baseResourceKindOpts}
            onChange={o => updateResourceOption(o as LockResourceOption)}
            isDisabled={attempt.status === 'processing'}
            css={`
              text-transform: capitalize;
            `}
          />
        </Box>
        {content}
        <CheckoutFooter
          isProcessing={attempt.status === 'processing'}
          numAddedResources={numAddedResources}
          clearSelectedResources={clearSelectedResources}
          setShowCheckout={setShowCheckout}
        />
        <Transition
          in={showCheckout}
          nodeRef={transitionRef}
          timeout={300}
          mountOnEnter
          unmountOnExit
        >
          {transitionState => (
            <LockCheckout
              ref={transitionRef}
              selectedResources={selectedResources}
              onClose={() => setShowCheckout(false)}
              toggleResource={toggleSelectResource}
              transitionState={transitionState}
              reset={clearSelectedResources}
              selectedResourceKind={selectedResourceKind}
              batchDeleteResources={batchDeleteResources}
            />
          )}
        </Transition>
        {/* This is a react-router provided prompt when it detects route change.
         * Prompts user when user navigates away from route, to help avoid losign work.
         */}
        <Prompt
          when={numAddedResources > 0}
          message={() => {
            return `${numAddedResources} resource(s) selected for locking will be cleared if you leave this page. Are you sure you want to continue?`;
          }}
        />
      </Box>
    </FeatureBox>
  );
}

function CheckoutFooter({
  numAddedResources,
  clearSelectedResources,
  setShowCheckout,
  isProcessing,
}: {
  isProcessing: boolean;
  numAddedResources: number;
  clearSelectedResources(): void;
  setShowCheckout(show: boolean): void;
}) {
  return (
    <Flex
      data-testid="checkout-footer"
      alignItems="center"
      justifyContent="space-between"
      borderRadius={3}
      p={3}
      mt={5}
      css={`
        background: ${({ theme }) => theme.colors.spotBackground[0]};
      `}
    >
      <Text bold>Targets Added ({numAddedResources})</Text>
      <Box>
        {numAddedResources > 0 && (
          <ButtonSecondary
            mr={3}
            width="165px"
            onClick={() => clearSelectedResources()}
            disabled={isProcessing}
          >
            Clear Selections
          </ButtonSecondary>
        )}
        <ButtonPrimary
          width="182px"
          onClick={() => setShowCheckout(true)}
          disabled={!numAddedResources || isProcessing}
        >
          Proceed to Lock
        </ButtonPrimary>
      </Box>
    </Flex>
  );
}

function getNumSelectedResources(resourceMap: LockResourceMap) {
  const kinds = Object.keys(resourceMap);
  let count = 0;

  kinds.forEach(kind => (count += Object.keys(resourceMap[kind]).length));

  return count;
}
