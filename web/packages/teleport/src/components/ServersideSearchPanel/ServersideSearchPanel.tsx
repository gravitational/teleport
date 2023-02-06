/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { Text, Box, Flex, Link } from 'design';
import { StyledPanel } from 'design/DataTable';
import InputSearch from 'design/DataTable/InputSearch';
import { PageIndicatorText } from 'design/DataTable/Pager/Pager';

import Toggle from 'teleport/components/Toggle';

import Tooltip from './Tooltip';
import useServersideSearchPanel, {
  State,
  Props,
} from './useServerSideSearchPanel';

const GUIDE_URL =
  'https://goteleport.com/docs/setup/reference/predicate-language/#resource-filtering';

export default function Container(props: Props) {
  const state = useServersideSearchPanel(props);
  return <ServersideSearchPanel {...state} />;
}

export function ServersideSearchPanel({
  searchString,
  setSearchString,
  isAdvancedSearch,
  setIsAdvancedSearch,
  onSubmitSearch,
  from,
  to,
  count,
}: State) {
  function onToggle() {
    setIsAdvancedSearch(!isAdvancedSearch);
  }

  return (
    <StyledPanel
      as="form"
      onSubmit={onSubmitSearch}
      borderTopLeftRadius={3}
      borderTopRightRadius={3}
    >
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        <Flex style={{ width: '70%' }} alignItems="center">
          <Box width="100%" mr={3}>
            <InputSearch
              searchValue={searchString}
              setSearchValue={setSearchString}
            >
              <ToggleWrapper>
                <Toggle isToggled={isAdvancedSearch} onToggle={onToggle} />
                <Text typography="paragraph2">Advanced</Text>
              </ToggleWrapper>
            </InputSearch>
          </Box>
          <Tooltip>
            <PredicateDoc />
          </Tooltip>
        </Flex>
        <Flex>
          <PageIndicatorText from={from} to={to} count={count} />
        </Flex>
      </Flex>
    </StyledPanel>
  );
}

export const PredicateDoc = () => (
  <>
    <Text typography="paragraph2">
      Advanced search allows you to perform more sophisticated searches using
      the predicate language. The language supports the basic operators:{' '}
      <Text as="span" bold>
        <code>==</code>{' '}
      </Text>
      ,{' '}
      <Text as="span" bold>
        <code>!=</code>
      </Text>
      ,{' '}
      <Text as="span" bold>
        <code>&&</code>
      </Text>
      , and{' '}
      <Text as="span" bold>
        <code>||</code>
      </Text>
    </Text>
    <Text typography="h4" mt={2} mb={1}>
      Usage Examples
    </Text>
    <Text typography="paragraph2">
      Label Matching:{' '}
      <Text ml={1} as="span" bold>
        <code>labels["key"] == "value" && labels["key2"] != "value2"</code>{' '}
      </Text>
      <br />
      Fuzzy Searching:{' '}
      <Text ml={1} as="span" bold>
        <code>search("foo", "bar", "some phrase")</code>
      </Text>
      <br />
      Combination:{' '}
      <Text ml={1} as="span" bold>
        <code>labels["key1"] == "value1" && search("foo")</code>
      </Text>
    </Text>
    <Text typography="paragraph2" mt={2}>
      Check out our{' '}
      <Link href={GUIDE_URL} target="_blank">
        predicate language guide
      </Link>{' '}
      for a more in-depth explanation of the language.
    </Text>
  </>
);

const ToggleWrapper = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding-right: 16px;
  padding-left: 16px;
  width: 120px;
`;
