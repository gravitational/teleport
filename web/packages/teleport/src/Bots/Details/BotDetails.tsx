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

import React, { useRef, useState } from 'react';
import { useHistory, useLocation, useParams } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { Button, ButtonSecondary } from 'design/Button/Button';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import { CardTile } from 'design/CardTile/CardTile';
import Flex, { Stack } from 'design/Flex/Flex';
import { ArrowLeft } from 'design/Icon/Icons/ArrowLeft';
import { FingerprintSimple } from 'design/Icon/Icons/FingerprintSimple';
import { LockKey } from 'design/Icon/Icons/LockKey';
import { MoreVert } from 'design/Icon/Icons/MoreVert';
import { NewTab } from 'design/Icon/Icons/NewTab';
import { Pencil } from 'design/Icon/Icons/Pencil';
import { Question } from 'design/Icon/Icons/Question';
import { Trash } from 'design/Icon/Icons/Trash';
import { Unlock } from 'design/Icon/Icons/Unlock';
import { Indicator } from 'design/Indicator/Indicator';
import { SecondaryOutlined } from 'design/Label/Label';
import Menu from 'design/Menu/Menu';
import MenuItem from 'design/Menu/MenuItem';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';
import { traitDescriptions } from 'shared/components/TraitsEditor/TraitsEditor';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import cfg from 'teleport/config';
import { ResourceLockDialog } from 'teleport/lib/locks/ResourceLockDialog';
import { ResourceLockIndicator } from 'teleport/lib/locks/ResourceLockIndicator';
import { ResourceUnlockDialog } from 'teleport/lib/locks/ResourceUnlockDialog';
import { useResourceLock } from 'teleport/lib/locks/useResourceLock';
import { isAdminActionRequiresMfaError } from 'teleport/services/api/api';
import useTeleport from 'teleport/useTeleport';

import { DeleteDialog } from '../Delete/DeleteDialog';
import { EditDialog } from '../Edit/EditDialog';
import { formatDuration } from '../formatDuration';
import { useGetBot, useListBotTokens } from '../hooks';
import { InfoGuide } from '../InfoGuide';
import { InstancesPanel } from './InstancesPanel';
import { JoinMethodIcon } from './JoinMethodIcon';
import { Panel } from './Panel';

export function BotDetails() {
  const ctx = useTeleport();
  const history = useHistory();
  const location = useLocation();
  const params = useParams<{
    botName: string;
  }>();
  const [isEditing, setEditing] = useState(false);
  const [showLockConfirmation, setShowLockConfirmation] = useState(false);
  const [showUnlockConfirmation, setShowUnlockConfirmation] = useState(false);
  const [showDeleteConfirmation, setShowDeleteConfirmation] = useState(false);

  const flags = ctx.getFeatureFlags();
  const hasReadPermission = flags.readBots;
  const hasEditPermission = flags.editBots;
  const hasDeletePermission = flags.removeBots;

  const { data, error, isSuccess, isError, isLoading } = useGetBot(params, {
    enabled: hasReadPermission,
    staleTime: 30_000, // Keep data in the cache for 30 seconds
  });

  const targetKind = 'user';
  const targetName = `bot-${params.botName}`;

  const {
    isLocked,
    error: lockError,
    canLock,
    canUnlock,
  } = useResourceLock({
    targetKind,
    targetName,
  });

  const handleBackPress = () => {
    // If location.key is unset, or 'default', this is the first history entry in-app in the session.
    if (!location.key || location.key === 'default') {
      history.push(cfg.getBotsRoute());
    } else {
      history.goBack();
    }
  };

  const handleEdit = () => {
    setEditing(true);
  };

  const handleEditSuccess = () => {
    setEditing(false);
  };

  const handleViewAllTokensClicked = () => {
    history.push(cfg.getJoinTokensRoute());
  };

  const handleLock = () => {
    setShowLockConfirmation(true);
  };

  const handleUnlock = () => {
    setShowUnlockConfirmation(true);
  };

  const handleDelete = () => {
    setShowDeleteConfirmation(true);
  };

  const handleDeleteComplete = () => {
    setShowDeleteConfirmation(false);
    history.replace(cfg.getBotsRoute());
  };

  return (
    <FeatureBox>
      <FeatureHeader gap={2} data-testid="page-header">
        <HoverTooltip placement="bottom" tipContent={'Go back'}>
          <ButtonIcon onClick={handleBackPress} aria-label="back">
            <ArrowLeft size="medium" />
          </ButtonIcon>
        </HoverTooltip>
        <Flex flex={1} gap={2} justifyContent="space-between" overflow="hidden">
          {isSuccess && data ? (
            <>
              <Flex gap={3}>
                <FeatureHeaderTitle>
                  <TitleText>{data.name}</TitleText>
                </FeatureHeaderTitle>

                {isLocked ? (
                  <ResourceLockIndicator
                    targetKind={targetKind}
                    targetName={targetName}
                  />
                ) : undefined}
              </Flex>

              <Flex gap={2}>
                <EditButton onClick={handleEdit} disabled={!hasEditPermission}>
                  <Pencil size="medium" /> Edit Bot
                </EditButton>

                <OverflowMenu
                  isLocked={isLocked}
                  canLock={canLock}
                  onLock={handleLock}
                  canUnlock={canUnlock}
                  onUnlock={handleUnlock}
                  canDelete={hasDeletePermission}
                  onDelete={handleDelete}
                />
              </Flex>
            </>
          ) : (
            <FeatureHeaderTitle>Bot details</FeatureHeaderTitle>
          )}
        </Flex>
        <InfoGuideButton config={{ guide: <InfoGuide /> }} />
      </FeatureHeader>

      {isLoading ? (
        <Box data-testid="loading-bot" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {isError ? (
        <Alert kind="danger" details={error.message}>
          Failed to fetch bot
        </Alert>
      ) : undefined}

      {lockError ? (
        <Alert kind="danger" details={lockError.message}>
          Failed to get lock status
        </Alert>
      ) : undefined}

      {isSuccess && data === null ? (
        <Alert kind="warning">Bot {params.botName} does not exist</Alert>
      ) : undefined}

      {!hasReadPermission ? (
        <Alert kind="info">
          You do not have permission to view this bot. Missing role permissions:{' '}
          <code>bots.read</code>
        </Alert>
      ) : undefined}

      {hasReadPermission && isSuccess && data ? (
        <Container>
          <ColumnContainer>
            <Panel
              title="Bot Details"
              action={{
                label: 'Edit',
                onClick: handleEdit,
                iconLeft: <Pencil size={'medium'} />,
                disabled: !hasEditPermission,
              }}
            />
            <Divider />

            <Panel title="Metadata" isSubPanel>
              <PanelContentContainer>
                <Grid>
                  <GridLabel>Bot name</GridLabel>
                  <Flex
                    inline
                    alignItems={'center'}
                    gap={1}
                    overflow={'hidden'}
                  >
                    <MonoText>{data.name}</MonoText>
                    <CopyButton name={data.name} />
                  </Flex>
                  <GridLabel>Max session duration</GridLabel>
                  {data.max_session_ttl
                    ? formatDuration(data.max_session_ttl, {
                        separator: ' ',
                      })
                    : '-'}
                </Grid>
              </PanelContentContainer>
            </Panel>

            <PaddedDivider />

            <Panel title="Roles" isSubPanel>
              <PanelContentContainer>
                {data.roles.length ? (
                  <LabelsContainer>
                    {data.roles.toSorted().map(r => (
                      <SecondaryOutlined key={r}>
                        <LabelText>{r}</LabelText>
                      </SecondaryOutlined>
                    ))}
                  </LabelsContainer>
                ) : (
                  <EmptyText>No roles assigned</EmptyText>
                )}
              </PanelContentContainer>
            </Panel>

            <PaddedDivider />

            <Panel title="Traits" isSubPanel>
              <PanelContentContainer>
                {data.traits.length ? (
                  <Grid>
                    {data.traits
                      .toSorted((a, b) => a.name.localeCompare(b.name))
                      .map(r => (
                        <React.Fragment key={r.name}>
                          <GridLabel>
                            <Trait traitName={r.name} />
                          </GridLabel>
                          <LabelsContainer>
                            {r.values.length > 0
                              ? r.values.toSorted().map(v => (
                                  <SecondaryOutlined key={v}>
                                    <LabelText>{v}</LabelText>
                                  </SecondaryOutlined>
                                ))
                              : 'no values'}
                          </LabelsContainer>
                        </React.Fragment>
                      ))}
                  </Grid>
                ) : (
                  <EmptyText>No traits set</EmptyText>
                )}
              </PanelContentContainer>
            </Panel>

            <Divider />

            <JoinTokens
              botName={data.name}
              onViewAllClicked={handleViewAllTokensClicked}
            />
          </ColumnContainer>
          <ColumnContainer maxWidth={400}>
            <InstancesPanel botName={params.botName} />
          </ColumnContainer>

          {isEditing ? (
            <EditDialog
              botName={data.name}
              onCancel={() => setEditing(false)}
              onSuccess={handleEditSuccess}
            />
          ) : undefined}

          {showLockConfirmation ? (
            <ResourceLockDialog
              onCancel={() => setShowLockConfirmation(false)}
              onComplete={() => setShowLockConfirmation(false)}
              targetKind={targetKind}
              targetName={targetName}
            />
          ) : undefined}

          {showUnlockConfirmation ? (
            <ResourceUnlockDialog
              onCancel={() => setShowUnlockConfirmation(false)}
              onComplete={() => setShowUnlockConfirmation(false)}
              targetKind={targetKind}
              targetName={targetName}
            />
          ) : undefined}

          {showDeleteConfirmation ? (
            <DeleteDialog
              onCancel={() => setShowDeleteConfirmation(false)}
              onComplete={handleDeleteComplete}
              canLockBot={canLock}
              onLockRequest={() => {
                setShowLockConfirmation(true);
                setShowDeleteConfirmation(false);
              }}
              botName={params.botName}
              showLockAlternative={!isLocked}
            />
          ) : undefined}
        </Container>
      ) : undefined}
    </FeatureBox>
  );
}

const Container = styled(Flex).attrs({ gap: 2 })`
  flex: 1;
  overflow: auto;
`;

const ColumnContainer = styled(CardTile)`
  flex-direction: column;
  overflow: auto;
  padding: 0;
  gap: 0;
  margin: ${props => props.theme.space[1]}px;
`;

const PanelContentContainer = styled(Flex)`
  flex-direction: column;
  padding: ${props => props.theme.space[3]}px;
  padding-top: 0;
`;

const LabelsContainer = styled(Flex)`
  flex-wrap: wrap;
  overflow: hidden;
  gap: ${props => props.theme.space[1]}px;
`;

const Divider = styled.div`
  height: 1px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  flex-shrink: 0;
`;

const PaddedDivider = styled(Divider)`
  margin-left: ${props => props.theme.space[3]}px;
  margin-right: ${props => props.theme.space[3]}px;
`;

const Grid = styled(Box)`
  align-self: flex-start;
  display: grid;
  grid-template-columns: repeat(2, auto);
  gap: ${({ theme }) => theme.space[2]}px;
  overflow: hidden;
`;

const GridLabel = styled(Text)`
  color: ${({ theme }) => theme.colors.text.muted};
  font-weight: ${({ theme }) => theme.fontWeights.regular};
  padding-right: ${({ theme }) => theme.space[2]}px;
`;

const MonoText = styled(Text)`
  font-family: ${({ theme }) => theme.fonts.mono};
`;

const EditButton = styled(ButtonSecondary)`
  gap: ${props => props.theme.space[2]}px;
  white-space: nowrap;
`;

const TitleText = styled(Text)`
  white-space: nowrap;
`;

const LabelText = styled(Text).attrs({
  typography: 'body3',
})`
  white-space: nowrap;
`;

const EmptyText = styled(Text)`
  color: ${p => p.theme.colors.text.muted};
`;

function Trait(props: { traitName: string }) {
  const theme = useTheme();

  const description = traitDescriptions[props.traitName];

  return description ? (
    <Flex gap={1}>
      {props.traitName}
      <HoverTooltip placement="top" tipContent={description}>
        <Question
          size={'small'}
          color={theme.colors.interactive.tonal.neutral[3]}
        />
      </HoverTooltip>
    </Flex>
  ) : (
    props.traitName
  );
}

function JoinTokens(props: { botName: string; onViewAllClicked: () => void }) {
  const { botName, onViewAllClicked } = props;

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasListPermission = flags.listTokens;

  const [skipAuthnRetry, setSkipAuthnRetry] = useState(true);

  const { data, error, isSuccess, isError, isLoading } = useListBotTokens(
    { botName, skipAuthnRetry },
    {
      enabled: hasListPermission,
      staleTime: 30_000, // Keep data in the cache for 30 seconds
    }
  );

  const requiresMfa = isError && isAdminActionRequiresMfaError(error);

  const handleVerifyClick = () => {
    setSkipAuthnRetry(false);
  };

  return (
    <Panel
      title="Join Tokens"
      isSubPanel
      action={{
        label: 'View All',
        onClick: onViewAllClicked,
        iconRight: <NewTab size={'medium'} />,
        disabled: !hasListPermission,
      }}
    >
      <PanelContentContainer>
        {isLoading ? (
          <Box data-testid="loading-tokens" textAlign="center" m={10}>
            <Indicator />
          </Box>
        ) : undefined}

        {!hasListPermission ? (
          <Alert kind="info">
            You do not have permission to view join tokens. Missing role
            permissions: <code>tokens.list</code>
          </Alert>
        ) : undefined}

        {requiresMfa ? (
          <MfaContainer>
            <MfaText fontWeight={'regular'}>
              Multi-factor authentication is required to view join tokens
            </MfaText>
            <MfaVerifyButton onClick={handleVerifyClick}>
              <FingerprintSimple size="medium" /> Authenticate
            </MfaVerifyButton>
          </MfaContainer>
        ) : undefined}

        {isError && !requiresMfa ? (
          <Alert kind="danger" details={error.message}>
            Failed to fetch join tokens
          </Alert>
        ) : undefined}

        {isSuccess ? (
          <>
            {data.items.length ? (
              <LabelsContainer>
                {data.items
                  .toSorted((a, b) => a.safeName.localeCompare(b.safeName))
                  .map(t => {
                    return (
                      <HoverTooltip
                        key={t.id}
                        placement="top"
                        tipContent={t.method}
                      >
                        <SecondaryOutlined padding={0}>
                          <Flex
                            alignItems={'center'}
                            gap={1}
                            padding={1}
                            paddingRight={2}
                          >
                            <JoinMethodIcon
                              method={t.method}
                              size={'large'}
                              includeTooltip={false}
                            />
                            <LabelText>{t.safeName}</LabelText>
                          </Flex>
                        </SecondaryOutlined>
                      </HoverTooltip>
                    );
                  })}
              </LabelsContainer>
            ) : (
              <EmptyText>No join tokens</EmptyText>
            )}
          </>
        ) : undefined}
      </PanelContentContainer>
    </Panel>
  );
}

const MfaContainer = styled(Stack)`
  align-items: center;
  gap: ${props => props.theme.space[2]}px;
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  padding: ${props => props.theme.space[4]}px;
  border: ${props => props.theme.borders[1]}
    ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${props => props.theme.radii[2]}px;
`;

const MfaText = styled(Text)`
  max-width: 216px;
  text-align: center;
`;

const MfaVerifyButton = styled(ButtonSecondary)`
  gap: ${props => props.theme.space[2]}px;
`;

function OverflowMenu(props: {
  isLocked: boolean;
  canLock: boolean;
  onLock: () => void;
  canUnlock: boolean;
  onUnlock: () => void;
  canDelete: boolean;
  onDelete: () => void;
}) {
  const {
    isLocked,
    canLock,
    onLock,
    canUnlock,
    onUnlock,
    canDelete,
    onDelete,
  } = props;
  const [isOpen, setIsOpen] = useState(false);
  const anchorElRef = useRef<HTMLButtonElement>(null);

  const handleLock = () => {
    // Disabled attribute on MenuItem is for styling only, so check if the user can lock the bot
    if (!canLock) return;
    onLock();
    setIsOpen(false);
  };

  const handleUnlock = () => {
    // Disabled attribute on MenuItem is for styling only, so check if the user can unlock the bot
    if (!canUnlock) return;
    onUnlock();
    setIsOpen(false);
  };

  const handleDelete = () => {
    // Disabled attribute on MenuItem is for styling only, so check if the user can delete the bot
    if (!canDelete) return;
    onDelete();
    setIsOpen(false);
  };

  return (
    <div>
      <FilledButtonIcon
        ref={anchorElRef}
        intent="neutral"
        onClick={() => {
          setIsOpen(true);
        }}
        data-testid="overflow-btn-open"
      >
        <MoreVert size="medium" />
      </FilledButtonIcon>

      <Menu
        anchorEl={anchorElRef.current}
        open={isOpen}
        onClose={() => setIsOpen(false)}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
      >
        {isLocked ? (
          <MenuItem disabled={!canUnlock} onClick={handleUnlock}>
            <Flex gap={2}>
              <Unlock size="small" />
              Unlock Bot...
            </Flex>
          </MenuItem>
        ) : (
          <MenuItem disabled={!canLock} onClick={handleLock}>
            <Flex gap={2}>
              <LockKey size="small" />
              Lock Bot...
            </Flex>
          </MenuItem>
        )}

        <MenuItem disabled={!canDelete} onClick={handleDelete}>
          <Flex gap={2}>
            <StyledTrashIcon size="small" />
            Delete Bot...
          </Flex>
        </MenuItem>
      </Menu>
    </div>
  );
}

const StyledTrashIcon = styled(Trash)`
  color: ${({ theme }) => theme.colors.interactive.solid.danger.default};
`;

const FilledButtonIcon = styled(Button)`
  width: 32px;
  height: 32px;
  padding: 0;
`;
