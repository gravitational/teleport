import styled from 'styled-components';
import { useEffect, useState } from 'react';
import {
  Box,
  Text,
  Flex,
  Indicator,
  Label,
  Alert,
  ButtonWarning,
  ButtonSecondary,
  Link,
  ButtonPrimary,
  Menu,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Table, { Cell } from 'design/DataTable';
import { MagnifyingMinus, MagnifyingPlus, Trash, Warning } from 'design/Icon';
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { HoverTooltip } from 'shared/components/ToolTip';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import { useTeleport } from 'teleport';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { JoinToken } from 'teleport/services/joinToken';
import ResourceEditor from 'teleport/components/ResourceEditor';
import { Resource, KindJoinToken } from 'teleport/services/resources';
import useResources from 'teleport/components/useResources';
import { templates } from 'teleport/services/joinToken/makeJoinToken';
import { Dropdown } from 'teleport/components/Dropdown';
import FieldSelect from 'shared/components/FieldSelect';

function makeTokenResource(token: JoinToken): Resource<KindJoinToken> {
  return {
    id: token.id,
    name: token.id,
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
    joinTokensAttempt?.data?.items?.map(makeTokenResource) || [],
    templates
  );

  const title =
    resources.status === 'creating' ? 'Create a new join token' : 'Edit token';

  console.log({ resources });

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
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Join Tokens</FeatureHeaderTitle>
        <ButtonPrimary onClick={() => resources.create('join_token')}>
          Add
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
                render: ({ expiryText }) => (
                  <Cell>
                    <Flex alignItems="center" gap={2}>
                      <Text>{expiryText}</Text>
                      {expiryText === 'never' && (
                        <HoverTooltip tipContent="This token is statically configured in your teleport configuration file and cannot be deleted via the Web UI. Static tokens are inherently insecure because they never expire and, if stolen, can be used by an attacker to join any resource to your cluster.">
                          <Warning size="small" color="error.main" />
                        </HoverTooltip>
                      )}
                    </Flex>
                  </Cell>
                ),
              },
              {
                altKey: 'delete-btn',
                render: token => (
                  <Cell align="right">
                    <HoverTooltip
                      css={`
                        display: flex;
                        justify-content: end;
                      `}
                      tipContent={
                        token.isStatic
                          ? 'Cannot delete static tokens'
                          : 'Delete token'
                      }
                    >
                      <StyledTrashButton
                        size="small"
                        onClick={() => setTokenToDelete(token)}
                        disabled={
                          token.isStatic ||
                          deleteTokenAttempt.status === 'processing'
                        }
                        p={2}
                        isStatic={token.isStatic}
                      />
                    </HoverTooltip>
                  </Cell>
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
          docsURL="https://goteleport.com/docs/access-controls/guides/role-templates/"
          title={title}
          text={resources.item?.content}
          name={resources.item?.name}
          isNew={resources.status === 'creating'}
          onSave={(...args) => console.log(args)}
          onClose={resources.disregard}
          directions={<Directions />}
          kind={resources.item?.kind}
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

const NameCell = ({ id, safeName, method, isStatic }: JoinToken) => {
  // const [visible, setVisible] = useState(false);
  // const onVisible = () => {
  //   setVisible(true);
  //   setTimeout(() => {
  //     setVisible(false);
  //   }, 5000);
  // };
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
        {/* {method === 'token' && (
            <ToggleVisibilityButton
              visible={visible}
              onShow={onVisible}
              onHide={() => setVisible(false)}
            />
          )} */}
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

const ToggleVisibilityButton = ({
  visible,
  onShow,
  onHide,
}: {
  visible: boolean;
  onShow: () => void;
  onHide: () => void;
}) => {
  return (
    <HoverTooltip tipContent={visible ? 'Hide' : 'Show'}>
      {visible ? (
        <MagnifyingMinus
          css={`
            cursor: pointer;
          `}
          size="small"
          onClick={onHide}
        />
      ) : (
        <MagnifyingPlus
          css={`
            cursor: pointer;
          `}
          onClick={onShow}
          size="small"
        />
      )}
    </HoverTooltip>
  );
};

const StyledTrashButton = styled(Trash)`
  cursor: ${props => (props.isStatic ? 'not-allowed' : 'pointer')};
  opacity: ${props => (props.isStatic ? '0.5' : 1)};
  background-color: ${props =>
    props.isStatic
      ? props.theme.colors.action.disabled
      : props.theme.colors.buttons.trashButton.default};
  border-radius: 2px;
`;

const StyledLabel = styled(Label)`
  height: 20px;
  margin: 1px 0;
  margin-right: ${props => props.theme.space[2]}px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
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
          I understand, delete user
        </ButtonWarning>
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

function Directions() {
  return (
    <>
      <HoverTooltip tipContent={'Select a token template'}>
        <ButtonSecondary
          px={2}
          css={`
            border-color: ${props => props.theme.colors.spotBackground[0]};
          `}
          textTransform="none"
          size="small"
          onClick={handleOpen}
        >
          Types{' '}
          {kindsFromParams.length > 0 ? `(${kindsFromParams.length})` : ''}
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
          {kindsFromParams.length > 0 && <FiltersExistIndicator />}
        </ButtonSecondary>
      </HoverTooltip>
      <Menu
        popoverCss={() => `
          margin-top: ${showInput ? '40px' : '4px'}; 
          max-height: 265px; 
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
        onClose={handleClose}
      ></Menu>
      WARNING tokens are defined using{' '}
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
