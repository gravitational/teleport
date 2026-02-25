/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useQuery } from '@tanstack/react-query';
import { Suspense, useMemo, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, H2, Indicator, Text } from 'design';
import InputSearch from 'design/DataTable/InputSearch';
import { Cross } from 'design/Icon';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel';
import * as discoveryService from 'teleport/services/discovery/discovery';
import {
  DiscoveryConfigLog,
  DiscoveryInstance,
  DiscoveryIssue,
} from 'teleport/services/discovery/types';
import useTeleport from 'teleport/useTeleport';

export function IssueList() {
  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const [search, setSearch] = useState('');
  const [selectedLog, setSelectedLog] = useState<DiscoveryConfigLog | null>(
    null
  );

  const { data } = useQuery({
    queryKey: ['discoveryConfigLogs', clusterId],
    queryFn: () => {
      return discoveryService.fetchDiscoveryConfigLogs(clusterId);
    },
  });

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return data?.items ?? [];

    return (data?.items ?? []).filter(log => {
      const instanceCount = log.instances.length;
      const issueCount = log.instances.reduce(
        (sum, instance) => sum + instance.issues.length,
        0
      );
      const summary = `${instanceCount} instances with ${issueCount} issues`;

      return (
        log.account_id.toLowerCase().includes(q) ||
        log.region.toLowerCase().includes(q) ||
        summary.toLowerCase().includes(q)
      );
    });
  }, [search, data]);

  return (
    <Suspense fallback={<Loading />}>
      <Box maxWidth="1400px" mx="auto" px={3}>
        <Box mb={3} maxWidth="560px">
          {/* <InputSearch searchValue={search} setSearchValue={setSearch} /> */}
        </Box>

        {filtered.length === 0 ? (
          <Box p={4} textAlign="center" color="text.slightlyMuted">
            {search ? 'No matching issues found' : 'No issues found'}
          </Box>
        ) : (
          <ListSurface>
            <HeaderRow>
              <HeaderCell>Account ID</HeaderCell>
              <HeaderCell>Region</HeaderCell>
              <HeaderCell>Summary</HeaderCell>
            </HeaderRow>

            {filtered.map((log, idx) => {
              const instanceCount = log.instances.length;
              const issueCount = log.instances.reduce(
                (sum, instance) => sum + instance.issues.length,
                0
              );
              const summary = `${instanceCount} instance${
                instanceCount !== 1 ? 's' : ''
              } with ${issueCount} issue${issueCount !== 1 ? 's' : ''}`;

              return (
                <IssueRow
                  key={`${log.account_id}-${log.region}-${idx}`}
                  isActive={selectedLog === log}
                  onClick={() => setSelectedLog(log)}
                >
                  <IssueCell>{log.account_id}</IssueCell>
                  <IssueCell>{log.region}</IssueCell>
                  <IssueCell>{summary}</IssueCell>
                </IssueRow>
              );
            })}
          </ListSurface>
        )}

        <Backdrop
          isVisible={!!selectedLog}
          onClick={() => setSelectedLog(null)}
        />

        <FullHeightSlidingSidePanel
          isVisible={!!selectedLog}
          slideFrom="right"
          skipAnimation={false}
          panelWidth={620}
          zIndex={1000}
        >
          {selectedLog && (
            <IssueLogDetails
              log={selectedLog}
              onClose={() => setSelectedLog(null)}
            />
          )}
        </FullHeightSlidingSidePanel>
      </Box>
    </Suspense>
  );
}

function IssueLogDetails({
  log,
  onClose,
}: {
  log: DiscoveryConfigLog;
  onClose: () => void;
}) {
  const totalIssues = log.instances.reduce(
    (sum, instance) => sum + instance.issues.length,
    0
  );

  return (
    <PanelContainer>
      <Flex alignItems="flex-start" justifyContent="space-between" p={3}>
        <Box>
          <H2>AWS Discovery Issues</H2>
          <Text typography="body3" color="text.slightlyMuted">
            {log.account_id} • {log.region}
          </Text>
        </Box>
        <ButtonIcon onClick={onClose} aria-label="Close">
          <Cross size="medium" />
        </ButtonIcon>
      </Flex>

      <PanelBody>
        <SectionTitle>Summary</SectionTitle>
        <InfoRow label="Account ID" value={log.account_id} />
        <InfoRow label="Region" value={log.region} />
        <InfoRow label="Instances" value={log.instances.length.toString()} />
        <InfoRow label="Total Issues" value={totalIssues.toString()} />

        <SectionTitle>Instances ({log.instances.length})</SectionTitle>
        {log.instances.length === 0 ? (
          <Box p={2} color="text.slightlyMuted">
            No instances found
          </Box>
        ) : (
          log.instances.map(instance => (
            <InstanceDetails key={instance.instance_id} instance={instance} />
          ))
        )}
      </PanelBody>
    </PanelContainer>
  );
}

function InstanceDetails({ instance }: { instance: DiscoveryInstance }) {
  return (
    <Box mb={3} p={2} bg="levels.sunken" borderRadius={2}>
      <Flex alignItems="center" gap={2} mb={2}>
        <Text typography="body2" bold>
          {instance.instance_id}
        </Text>
        <Box
          as="span"
          px={2}
          py={0.5}
          borderRadius={10}
          bg="interactive.tonal.danger[0]"
          color="error.main"
          css={{ fontSize: '12px', fontWeight: 600 }}
        >
          {instance.issues.length}
        </Box>
      </Flex>

      {instance.issues.map((issue, idx) => (
        <Box
          key={idx}
          mb={2}
          pl={2}
          borderLeft="2px solid"
          borderColor={getConfidenceColor(issue.confidence)}
        >
          <Flex alignItems="center" gap={2} mb={1}>
            <ConfidenceBadge confidence={issue.confidence} />
            {issue.count > 1 && (
              <Text typography="body3" color="text.slightlyMuted">
                ({issue.count} occurrences)
              </Text>
            )}
          </Flex>

          <Text typography="body2" bold mb={1}>
            {issue.error_summary}
          </Text>

          <Box pl={2}>
            <Text typography="body3" color="text.slightlyMuted" mb={1}>
              Remediation:
            </Text>
            <Text typography="body3">{issue.remediation}</Text>
          </Box>
        </Box>
      ))}
    </Box>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <Flex gap={2} mb={1}>
      <Text
        typography="body3"
        color="text.slightlyMuted"
        css={{ minWidth: '120px' }}
      >
        {label}:
      </Text>
      <Text typography="body3">{value}</Text>
    </Flex>
  );
}

function ConfidenceBadge({ confidence }: { confidence: string }) {
  const getColor = () => {
    switch (confidence) {
      case 'high':
        return 'error.main';
      case 'medium':
        return 'warning.main';
      case 'low':
        return 'info.main';
      default:
        return 'text.muted';
    }
  };

  return (
    <Box
      as="span"
      px={2}
      py={1}
      borderRadius={1}
      bg={getColor()}
      color="text.primaryInverse"
      css={{ fontSize: '11px', fontWeight: 600, textTransform: 'uppercase' }}
    >
      {confidence}
    </Box>
  );
}

function getConfidenceColor(confidence: string): string {
  switch (confidence) {
    case 'high':
      return 'error.main';
    case 'medium':
      return 'warning.main';
    case 'low':
      return 'info.main';
    default:
      return 'text.muted';
  }
}

export function Loading() {
  return (
    <Flex alignItems="center" justifyContent="center" height="100%">
      <Indicator />
    </Flex>
  );
}

// Styled Components
const ListSurface = styled.div`
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 10px;
  overflow: auto;
`;

const HeaderRow = styled.div`
  display: grid;
  grid-template-columns: minmax(200px, 1fr) 140px minmax(300px, 2fr);
  align-items: center;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const HeaderCell = styled.div`
  padding: 10px 12px;
  font-size: 12px;
  font-weight: 600;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const IssueRow = styled.div<{ isActive: boolean }>`
  display: grid;
  grid-template-columns: minmax(200px, 1fr) 140px minmax(300px, 2fr);
  align-items: center;
  min-height: 52px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
  cursor: pointer;
  background: ${p =>
    p.isActive ? p.theme.colors.interactive.tonal.primary[0] : 'transparent'};

  &:hover {
    background: ${p => p.theme.colors.interactive.tonal.primary[0]};
  }
`;

const IssueCell = styled.div`
  padding: 8px 12px;
  font-size: 14px;
  overflow: hidden;
`;

const Backdrop = styled.div<{ isVisible: boolean }>`
  position: fixed;
  inset: 0;
  z-index: 999;
  background: rgba(0, 0, 0, 0.55);
  transition: opacity 0.15s ease-in-out;
  opacity: ${p => (p.isVisible ? 1 : 0)};
  pointer-events: ${p => (p.isVisible ? 'auto' : 'none')};
`;

const FullHeightSlidingSidePanel = styled(SlidingSidePanel)`
  top: 0;
`;

const PanelContainer = styled.div`
  height: 100%;
  display: flex;
  flex-direction: column;
  background: ${p => p.theme.colors.levels.sunken};
`;

const PanelBody = styled.div`
  overflow: auto;
  padding: 0 16px 16px;
`;

const SectionTitle = styled.h3`
  font-size: 18px;
  margin: 18px 0 10px;
`;
