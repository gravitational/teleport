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

import React, { forwardRef, useImperativeHandle } from 'react';
import styled from 'styled-components';

import { Alert, Info } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { ButtonSecondary } from 'design/Button/Button';
import Flex from 'design/Flex/Flex';
import { Indicator } from 'design/Indicator/Indicator';
import Text from 'design/Text';
import { SortMenu } from 'shared/components/Controls/SortMenu';

import { Instance } from 'teleport/Bots/Details/Instance';
import { BotInstanceSummary } from 'teleport/services/bot/types';

export const BotInstancesList = forwardRef(InternalBotInstancesList);

export type BotInstancesListControls = {
  scrollToTop: () => void;
};

function InternalBotInstancesList(
  props: {
    data: BotInstanceSummary[] | null | undefined;
    isLoading: boolean;
    isFetchingNextPage: boolean;
    error: Error | null | undefined;
    hasNextPage: boolean;
    sortField: string;
    sortDir: 'ASC' | 'DESC';
    selectedItem: string | null;
    onSortChanged: (sortField: string, sortDir: 'ASC' | 'DESC') => void;
    onLoadNextPage: () => void;
    onItemSelected: (item: BotInstanceSummary) => void;
    isFiltering: boolean;
  },
  ref: React.RefObject<BotInstancesListControls | null>
) {
  const {
    data,
    isLoading,
    isFetchingNextPage,
    error,
    hasNextPage,
    sortField,
    sortDir,
    selectedItem,
    onSortChanged,
    onLoadNextPage,
    onItemSelected,
    isFiltering,
  } = props;

  const contentRef = React.useRef<HTMLDivElement>(null);
  useImperativeHandle(ref, () => {
    return {
      scrollToTop() {
        contentRef.current?.scrollTo({ top: 0, behavior: 'instant' });
      },
    };
  }, [contentRef]);

  const hasError = !!error;
  const hasData = !hasError && !isLoading;
  const hasUnsupportedSortError = isUnsupportedSortError(error);

  const makeOnSelectedCallback = (instance: BotInstanceSummary) => () => {
    onItemSelected(instance);
  };

  return (
    <Container>
      <TitleContainer>
        <TitleText>
          {isFiltering ? 'Filtered Instances' : 'Active Instances'}
        </TitleText>
        <SortMenu
          current={{
            fieldName: sortField,
            dir: sortDir,
          }}
          fields={sortFields}
          onChange={value => {
            onSortChanged(value.fieldName, value.dir);
          }}
        />
      </TitleContainer>

      <Divider />

      {isLoading ? (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {hasError && hasUnsupportedSortError ? (
        <Alert
          m={3}
          kind="warning"
          primaryAction={{
            content: 'Reset sort',
            onClick: () => {
              onSortChanged('bot_name', 'ASC');
            },
          }}
        >
          {error.message}
        </Alert>
      ) : undefined}

      {hasError && !hasUnsupportedSortError ? (
        <Alert m={3} kind="danger" details={error.message}>
          Failed to fetch instances
        </Alert>
      ) : undefined}

      {hasData ? (
        <>
          {data && data.length > 0 ? (
            <ContentContainer ref={contentRef} data-scrollbar="default">
              {data.map((instance, i) => (
                <React.Fragment key={`${instance.instance_id}`}>
                  {i === 0 ? undefined : <Divider />}
                  <Instance
                    isSelectable
                    onSelected={makeOnSelectedCallback(instance)}
                    isSelected={
                      `${instance.bot_name}/${instance.instance_id}` ==
                      selectedItem
                    }
                    data={{
                      id: instance.instance_id,
                      botName: instance.bot_name,
                      version: instance.version_latest,
                      hostname: instance.host_name_latest,
                      activeAt: instance.active_at_latest,
                      method: instance.join_method_latest,
                      os: instance.os_latest,
                    }}
                  />
                </React.Fragment>
              ))}

              <Divider />

              <LoadMoreContainer>
                <ButtonSecondary
                  onClick={() => onLoadNextPage()}
                  disabled={!hasNextPage || isFetchingNextPage}
                >
                  Load More
                </ButtonSecondary>
              </LoadMoreContainer>
            </ContentContainer>
          ) : (
            <Box p={3}>
              <EmptyText>
                {isFiltering
                  ? 'No instances matching filter'
                  : 'No active instances'}
              </EmptyText>
              {!isFiltering ? (
                <Info mt={3}>
                  Bot instances are ephemeral, and disappear once all issued
                  credentials have expired.
                </Info>
              ) : undefined}
            </Box>
          )}
        </>
      ) : undefined}
    </Container>
  );
}

const Container = styled.section`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 300px;
  max-width: 400px;
`;

const TitleContainer = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  gap: ${p => p.theme.space[2]}px;
  padding-left: ${p => p.theme.space[3]}px;
  padding-right: ${p => p.theme.space[3]}px;
  min-height: ${p => p.theme.space[8]}px;
`;

export const TitleText = styled(Text).attrs({
  as: 'h2',
  typography: 'h2',
})``;

const ContentContainer = styled.div`
  overflow: auto;
`;

const LoadMoreContainer = styled(Flex)`
  justify-content: center;
  padding: ${props => props.theme.space[3]}px;
`;

const Divider = styled.div`
  height: 1px;
  flex-shrink: 0;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;

const EmptyText = styled(Text)`
  color: ${p => p.theme.colors.text.muted};
`;

const isUnsupportedSortError = (error: Error | null | undefined) => {
  return !!error && error.message.includes('unsupported sort');
};

const sortFields = [
  {
    value: 'bot_name' as const,
    label: 'Bot name',
  },
  {
    value: 'active_at_latest' as const,
    label: 'Recent',
  },
  {
    value: 'version_latest' as const,
    label: 'Version',
  },
  {
    value: 'host_name_latest' as const,
    label: 'Hostname',
  },
];
