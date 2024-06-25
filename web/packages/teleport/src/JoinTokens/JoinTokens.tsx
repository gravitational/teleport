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
  ButtonPrimary,
  MenuItem,
  ButtonWarning,
  ButtonSecondary,
  Menu,
} from 'design';
import Table, { Cell } from 'design/DataTable';
import { ChevronDown, Warning } from 'design/Icon';
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
import { JoinMethod, JoinToken } from 'teleport/services/joinToken';
import ResourceEditor from 'teleport/components/ResourceEditor';
import { Resource, KindJoinToken } from 'teleport/services/resources';
import { templates } from 'teleport/services/joinToken/makeJoinToken';

function makeTokenResource(token: JoinToken): Resource<KindJoinToken> {
  return {
    id: token.id,
    name: token.safeName,
    kind: 'join_token',
    content: token.content,
  };
}

function getJoinMethodTemplate(method: JoinMethod): string {
  return templates[method] || templates.iam;
}

export const JoinTokens = () => {
  const ctx = useTeleport();
  const [selectedMethod, setSelectedMethod] = useState<JoinMethod>('iam');
  const [template, setTemplate] = useState(getJoinMethodTemplate('iam'));
  const [tokenToDelete, setTokenToDelete] = useState<JoinToken | null>(null);
  const [joinTokensAttempt, runJoinTokensAttempt, setJoinTokensAttempt] =
    useAsync(async () => await ctx.joinTokenService.fetchJoinTokens());

  const resources = useResources(
    joinTokensAttempt.data?.items.map(makeTokenResource) || [],
    { join_token: template }
  );

  async function handleSave(content: string): Promise<void> {
    const token = await ctx.joinTokenService.createJoinToken({ content });
    let items = [...joinTokensAttempt.data.items];
    if (resources.status === 'creating') {
      items.push(token);
    } else {
      items = items.map(item => {
        if (item.id === token.id) {
          return token;
        }
        return item;
      });
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

  const onTemplateChange = (method: JoinMethod) => {
    setSelectedMethod(method);
    setTemplate(getJoinMethodTemplate(method));
  };

  useEffect(() => {
    runJoinTokensAttempt();
  }, []);

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Join Tokens</FeatureHeaderTitle>
        <ButtonPrimary ml="auto" onClick={() => resources.create('join_token')}>
          Create new token
        </ButtonPrimary>
      </FeatureHeader>
      <Box>
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
                render: NameCell,
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
              {
                key: 'expiry',
                headerText: 'Expires in',
                isSortable: true,
                render: ({ expiry, expiryText, isStatic, id, method }) => {
                  const now = new Date();
                  const isLongLived =
                    isAfter(expiry, addHours(now, 4)) && method === 'token';
                  return (
                    <Cell>
                      <Flex alignItems="center" gap={2}>
                        <Text>{expiryText}</Text>
                        {(isLongLived || isStatic) && (
                          <HoverTooltip tipContent="Long-lived and static tokens are less secure. We recommend using a different join method other than token for long-lived access.">
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
      {(resources.status === 'creating' || resources.status === 'editing') && (
        <ResourceEditor
          docsURL="https://goteleport.com/docs/reference/join-methods"
          title={'create token'}
          key={template} // reset the editor if the template changes
          text={
            resources.status === 'creating' ? template : resources.item.content
          }
          name={resources.item.name}
          isNew={resources.status === 'creating'}
          onSave={handleSave}
          onClose={resources.disregard}
          directions={
            <Directions
              selected={selectedMethod}
              options={Object.keys(templates) as JoinMethod[]}
              creating={resources.status === 'creating'}
              onTemplateChange={onTemplateChange}
            />
          }
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

const NameCell = ({ id, safeName, method }: JoinToken) => {
  return (
    <Cell
      align="left"
      style={{
        minWidth: '200px',
        fontFamily: 'monospace',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
    >
      <Flex alignItems="center" gap={2}>
        <CopyButton name={id}></CopyButton>
        <Text
          css={`
            text-overflow: clip;
            overflow-x: auto;
          `}
        >
          {method !== 'token' ? id : safeName}
        </Text>
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
          . This will not remove any resource that used this join token to join
          the cluster. It will only prevent this token from being used to join
          any more resources.
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

function Directions({
  selected,
  options,
  creating,
  onTemplateChange,
}: {
  selected: JoinMethod;
  options: JoinMethod[];
  creating: boolean;
  onTemplateChange: (method: JoinMethod) => void;
}) {
  const [anchorEl, setAnchorEl] = useState(null);
  const handleOpen = event => {
    setAnchorEl(event.currentTarget);
  };

  function onChangeOption(method: JoinMethod) {
    onTemplateChange(method);
    setAnchorEl(null);
  }

  return (
    <>
      {creating && (
        <Box mb={3}>
          <Text mb={2}>Select join method template</Text>
          <HoverTooltip
            tipContent={
              'Select an existing template for your preferred join method.'
            }
          >
            <ButtonSecondary
              width="100px"
              px={2}
              size="large"
              css={`
                border-color: ${props => props.theme.colors.spotBackground[0]};
              `}
              textTransform="none"
              onClick={handleOpen}
            >
              {selected}
              <ChevronDown ml="auto" size="small" color="text.slightlyMuted" />
            </ButtonSecondary>
          </HoverTooltip>
          <Menu
            popoverCss={() => `
              margin-top: 36px;
              overflow: hidden; 
            `}
            transformOrigin={{
              vertical: 'top',
              horizontal: 'left',
            }}
            anchorOrigin={{
              vertical: 'bottom',
              horizontal: 'left',
            }}
            anchorEl={anchorEl}
            open={Boolean(anchorEl)}
            onClose={() => setAnchorEl(null)}
          >
            {options.map(method => (
              <MenuItem
                px={2}
                key={method}
                onClick={() => onChangeOption(method)}
              >
                <Text ml={2} fontSize={2}>
                  {method}
                </Text>
              </MenuItem>
            ))}
          </Menu>
        </Box>
      )}
      <Box mt={4}>
        WARNING: Tokens are defined using{' '}
        <Link
          color="text.main"
          target="_blank"
          href="https://en.wikipedia.org/wiki/YAML"
        >
          YAML format
        </Link>
        . YAML is sensitive to white space, so please be careful.
      </Box>
    </>
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
  if (token.isStatic) {
    return (
      <Cell align="right">
        <HoverTooltip
          justifyContentProps={{ justifyContent: 'end' }}
          tipContent="Statically configured tokens cannot be editted or deleted via the web UI. Static tokens are less secure and it is recommended that you remove them from your teleport configuration file."
        >
          <MenuButton buttonProps={{ disabled: true }} />
        </HoverTooltip>
      </Cell>
    );
  }
  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={onEdit}>View/Edit...</MenuItem>
        <MenuItem onClick={onDelete}>Delete...</MenuItem>
      </MenuButton>
    </Cell>
  );
};
