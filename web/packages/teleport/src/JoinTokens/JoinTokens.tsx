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

import styled from 'styled-components';
import { useEffect, useState } from 'react';
import { isAfter, addHours } from 'date-fns';
import {
  Box,
  Text,
  Flex,
  Indicator,
  Label,
  Alert,
  Link,
  MenuItem,
  ButtonWarning,
  ButtonSecondary,
} from 'design';
import Table, { Cell } from 'design/DataTable';
import { Warning } from 'design/Icon';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { MenuButton } from 'shared/components/MenuAction';
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { HoverTooltip } from 'shared/components/ToolTip';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import { useTeleport } from 'teleport';
import useResources from 'teleport/components/useResources';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { JoinToken } from 'teleport/services/joinToken';
import { Resource, KindJoinToken } from 'teleport/services/resources';
import ResourceEditor from 'teleport/components/ResourceEditor';

function makeTokenResource(token: JoinToken): Resource<KindJoinToken> {
  return {
    id: token.id,
    name: token.safeName,
    kind: 'join_token',
    content: token.content,
  };
}

export const JoinTokens = () => {
  const ctx = useTeleport();
  const [tokenToDelete, setTokenToDelete] = useState<JoinToken | null>(null);
  const [joinTokensAttempt, runJoinTokensAttempt, setJoinTokensAttempt] =
    useAsync(async () => await ctx.joinTokenService.fetchJoinTokens());

  const resources = useResources(
    joinTokensAttempt.data?.items.map(makeTokenResource) || [],
    { join_token: '' } // we are only editing for now, so template can be empty
  );

  async function handleSave(content: string): Promise<void> {
    const token = await ctx.joinTokenService.upsertJoinToken({ content });
    let items = [...joinTokensAttempt.data.items];
    if (resources.status === 'creating') {
      items.push(token);
    } else {
      let tokenExistsInPreviousList = false;
      const newItems = items.map(item => {
        if (item.id === token.id) {
          tokenExistsInPreviousList = true;
          return token;
        }
        return item;
      });
      // in the edge case that someone only edits the name of the token, it will return
      // a "new" token via the upsert, and therefore should be treated as a new token
      if (!tokenExistsInPreviousList) {
        newItems.push(token);
      }
      items = newItems;
    }
    setJoinTokensAttempt({
      data: { ...joinTokensAttempt.data, items },
      status: 'success',
      statusText: '',
    });
  }

  const [deleteTokenAttempt, runDeleteTokenAttempt] = useAsync(
    async (token: string) => {
      await ctx.joinTokenService.deleteJoinToken(token);
      setJoinTokensAttempt({
        status: 'success',
        statusText: '',
        data: {
          items: joinTokensAttempt.data.items.filter(t => t.id !== token),
        },
      });
      setTokenToDelete(null);
    }
  );

  useEffect(() => {
    runJoinTokensAttempt();
  }, []);

  return (
    <FeatureBox>
      <FeatureHeader
        css={`
          // TODO (avatus) remove all border-bottom from FeatureHeader, requested by design
          border-bottom: none;
        `}
        alignItems="center"
      >
        <FeatureHeaderTitle>Join Tokens</FeatureHeaderTitle>
      </FeatureHeader>
      <Box>
        {joinTokensAttempt.status === 'error' && (
          <Alert kind="danger">{joinTokensAttempt.statusText}</Alert>
        )}
        {deleteTokenAttempt.status === 'error' && (
          <Alert kind="danger">{deleteTokenAttempt.statusText}</Alert>
        )}
        {joinTokensAttempt.status === 'success' && (
          <Table
            isSearchable
            data={joinTokensAttempt.data.items}
            columns={[
              {
                key: 'id',
                headerText: 'Name',
                isSortable: false,
                render: token => <NameCell token={token} />,
              },
              {
                key: 'method',
                headerText: 'Join Method',
                isSortable: true,
              },
              {
                key: 'roles',
                headerText: 'Roles',
                isSortable: false,
                render: renderRolesCell,
              },
              // expiryText is non render and used for searching
              {
                key: 'expiryText',
                isNonRender: true,
              },
              // expiry is used for sorting, but we display the expiryText value
              {
                key: 'expiry',
                headerText: 'Expires in',
                isSortable: true,
                render: ({ expiry, expiryText, isStatic, method }) => {
                  const now = new Date();
                  const isLongLived =
                    isAfter(expiry, addHours(now, 24)) && method === 'token';
                  return (
                    <Cell>
                      <Flex alignItems="center" gap={2}>
                        <Text>{expiryText}</Text>
                        {(isLongLived || isStatic) && (
                          <HoverTooltip tipContent="Long-lived and static tokens are insecure and will be deprecated. Use short-lived tokens or alternative join methods (gcp, iam) for long-lived access.">
                            <Warning size="small" color="error.main" />
                          </HoverTooltip>
                        )}
                      </Flex>
                    </Cell>
                  );
                },
              },
              {
                altKey: 'options-btn',
                render: (token: JoinToken) => (
                  <ActionCell
                    token={token}
                    onEdit={() => resources.edit(token.id)}
                    onDelete={() => setTokenToDelete(token)}
                  />
                ),
              },
            ]}
            emptyText="No active join tokens found"
            pagination={{ pageSize: 30, pagerPosition: 'top' }}
            customSearchMatchers={[searchMatcher]}
            initialSort={{
              key: 'expiry',
              dir: 'ASC',
            }}
          />
        )}
        {joinTokensAttempt.status === 'processing' && (
          <Flex justifyContent="center">
            <Indicator />
          </Flex>
        )}
      </Box>
      {tokenToDelete && (
        <TokenDelete
          token={tokenToDelete}
          onClose={() => setTokenToDelete(null)}
          onDelete={() => runDeleteTokenAttempt(tokenToDelete.id)}
          attempt={deleteTokenAttempt}
        />
      )}
      {resources.status === 'editing' && (
        <ResourceEditor
          docsURL="https://goteleport.com/docs/reference/join-methods"
          title={'Edit token'}
          text={resources.item.content}
          name={resources.item.name}
          isNew={false} // only editting is allowed
          onSave={handleSave}
          onClose={resources.disregard}
          directions={<Directions />}
          kind={'join_token'}
        />
      )}
    </FeatureBox>
  );
};

export function searchMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof JoinToken & string
) {
  if (propName === 'roles') {
    return targetValue.some((role: string) =>
      role.toUpperCase().includes(searchValue)
    );
  }
}

const renderRolesCell = ({ roles }: JoinToken) => {
  return (
    <Cell>
      {roles.map(role => (
        <StyledLabel key={role}>{role}</StyledLabel>
      ))}
    </Cell>
  );
};

const NameCell = ({ token }: { token: JoinToken }) => {
  const { id, safeName, method } = token;
  const [hovered, setHovered] = useState(false);
  return (
    <Cell
      align="left"
      style={{
        minWidth: '320px',
        fontFamily: 'monospace',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <Flex alignItems="center" gap={2}>
        <Text
          css={`
            text-overflow: clip;
            overflow-x: auto;
          `}
        >
          {method !== 'token' ? id : safeName}
        </Text>
        {hovered && <CopyButton name={id} />}
      </Flex>
    </Cell>
  );
};

const StyledLabel = styled(Label)`
  height: 20px;
  margin: 1px 0;
  margin-right: ${props => props.theme.space[2]}px;
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  color: ${props => props.theme.colors.text.main};
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 20px;
`;

function TokenDelete({
  token,
  onDelete,
  onClose,
  attempt,
}: {
  token: JoinToken;
  onDelete: (token: string) => Promise<any>;
  onClose: () => void;
  attempt: Attempt<void>;
}) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={close}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Delete Join Token?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'error' && <Alert children={attempt.statusText} />}
        <Text mb={4}>
          You are about to delete join token
          <Text bold as="span">
            {` ${token.safeName}`}
          </Text>
          . This will not remove any resources that used this token to join the
          cluster. This will remove the ability for any new resources to join
          with this token and any non-renewable resource from renewing.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onDelete}
        >
          I understand, delete token
        </ButtonWarning>
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

const ActionCell = ({
  onEdit,
  onDelete,
  token,
}: {
  onEdit(): void;
  onDelete(): void;
  token: JoinToken;
}) => {
  const buttonProps = { width: '100px' };
  if (token.isStatic) {
    return (
      <Cell align="right">
        <HoverTooltip
          justifyContentProps={{ justifyContent: 'end' }}
          tipContent="You cannot configure or delete static tokens via the web UI. Static tokens should be removed from your Teleport configuration file."
        >
          <MenuButton buttonProps={{ disabled: true, ...buttonProps }} />
        </HoverTooltip>
      </Cell>
    );
  }
  return (
    <Cell align="right">
      <MenuButton buttonProps={buttonProps}>
        <MenuItem onClick={onEdit}>View/Edit...</MenuItem>
        <MenuItem onClick={onDelete}>Delete...</MenuItem>
      </MenuButton>
    </Cell>
  );
};

function Directions() {
  return (
    <>
      WARNING Roles are defined using{' '}
      <Link
        color="text.main"
        target="_blank"
        href="https://en.wikipedia.org/wiki/YAML"
      >
        YAML format
      </Link>
      . YAML is sensitive to white space, so please be careful.
    </>
  );
}
