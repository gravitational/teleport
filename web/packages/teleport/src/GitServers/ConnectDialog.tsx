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

import { useEffect, useState } from 'react';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
  Indicator,
  Link,
  Text,
} from 'design';
import { CircleCheck } from 'design/Icon';
import { Danger } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import cfg from 'teleport/config';
import { generateTshLoginCommand } from 'teleport/lib/util';
import api from 'teleport/services/api';
import { AuthType } from 'teleport/services/user';

export function ConnectDialog({
  username,
  clusterId,
  organization,
  gitServerName,
  sshEnabled,
  httpEnabled,
  onClose,
  authType,
  accessRequestId,
}: {
  organization: string;
  gitServerName: string;
  sshEnabled?: boolean;
  httpEnabled?: boolean;
  onClose: () => void;
  username: string;
  clusterId: string;
  authType: AuthType;
  accessRequestId?: string;
}) {
  const repoURL = `https://github.com/orgs/${organization}/repositories`;
  const title = `Connect to GitHub Organization '${organization}'`;

  const [credStatus, setCredStatus] = useState<{
    loading: boolean;
    valid?: boolean;
    githubUsername?: string;
    error?: string;
  }>({ loading: httpEnabled });

  const [revoking, setRevoking] = useState(false);
  const [oauthStarted, setOauthStarted] = useState(false);

  // Listen for OAuth completion from the callback page via BroadcastChannel.
  useEffect(() => {
    if (!oauthStarted) {
      return;
    }
    const channel = new BroadcastChannel('github-oauth-complete');
    channel.onmessage = () => refreshCredStatus();
    return () => channel.close();
  }, [oauthStarted]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!httpEnabled) {
      return;
    }
    api
      .get(
        cfg.getGitServerCredentialsUrl({
          clusterId,
          name: gitServerName,
        })
      )
      .then((resp: { valid?: boolean; githubUsername?: string }) => {
        setCredStatus({
          loading: false,
          valid: resp.valid,
          githubUsername: resp.githubUsername,
        });
      })
      .catch(err => {
        setCredStatus({
          loading: false,
          error: err.message || 'Failed to check credentials',
        });
      });
  }, [httpEnabled, clusterId, gitServerName]);

  function handleRevoke() {
    setRevoking(true);
    api
      .delete(
        cfg.getGitServerCredentialsUrl({
          clusterId,
          name: gitServerName,
        })
      )
      .then(() => {
        setCredStatus({ loading: false, valid: false });
        setRevoking(false);
        setOauthStarted(false);
      })
      .catch(err => {
        setCredStatus({
          loading: false,
          error: err.message || 'Failed to revoke credentials',
        });
        setRevoking(false);
      });
  }

  function handleGitHubLogin() {
    window.open(`/web/github/integration/login/${encodeURIComponent(organization)}`, '_blank');
    setOauthStarted(true);
  }

  function refreshCredStatus() {
    setCredStatus({ loading: true });
    api
      .get(
        cfg.getGitServerCredentialsUrl({
          clusterId,
          name: gitServerName,
        })
      )
      .then((resp: { valid?: boolean; githubUsername?: string }) => {
        setCredStatus({
          loading: false,
          valid: resp.valid,
          githubUsername: resp.githubUsername,
        });
      })
      .catch(err => {
        setCredStatus({
          loading: false,
          error: err.message || 'Failed to check credentials',
        });
      });
  }

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '600px',
        width: '100%',
      })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>{title}</DialogTitle>
      </DialogHeader>
      <DialogContent gap={4}>
        {httpEnabled && (
          <Box
            mb={3}
            p={3}
            borderRadius={2}
            css={`
              border: 1px solid ${props => props.theme.colors.spotBackground[1]};
            `}
          >
            {credStatus.loading && <Indicator size="small" />}
            {credStatus.error && <Danger>{credStatus.error}</Danger>}
            {!credStatus.loading && !credStatus.error && (
              <>
                {credStatus.valid ? (
                  <Box css={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Box css={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                      <CircleCheck size="small" color="success.main" mr={1} />
                      <Text>
                        Connected to GitHub
                        {credStatus.githubUsername
                          ? <Text as="span" bold>{` ${credStatus.githubUsername}`}</Text>
                          : ''}
                      </Text>
                    </Box>
                    <ButtonWarning
                      size="small"
                      onClick={handleRevoke}
                      disabled={revoking}
                      title="Revoke stored GitHub credentials. You will need to re-authorize with GitHub to use HTTPS git operations and the gh CLI."
                    >
                      {revoking ? 'Revoking...' : 'Revoke'}
                    </ButtonWarning>
                  </Box>
                ) : (
                  <Box>
                    <Text color="text.slightlyMuted" mb={2}>
                      {oauthStarted
                        ? 'Complete the authorization in the new tab, then click below.'
                        : 'Authorize with GitHub to enable HTTPS git operations and the gh CLI.'}
                    </Text>
                    <Box css={{ display: 'flex', gap: '8px' }}>
                      {oauthStarted ? (
                        <ButtonPrimary
                          size="small"
                          onClick={refreshCredStatus}
                        >
                          I've Completed Authorization
                        </ButtonPrimary>
                      ) : (
                        <ButtonPrimary size="small" onClick={handleGitHubLogin}>
                          Connect GitHub
                        </ButtonPrimary>
                      )}
                    </Box>
                  </Box>
                )}
              </>
            )}
          </Box>
        )}

        <Box>
          <Text bold as="span">
            Step 1
          </Text>
          {' - Log in to Teleport'}
          <TextSelectCopy
            mt="1"
            mb="2"
            text={generateTshLoginCommand({
              authType,
              clusterId,
              username,
              accessRequestId,
            })}
          />
        </Box>
        <Box>
          <Text bold as="span">
            Step 2
          </Text>
          {' - Use tsh to access GitHub'}
          <br />
          {'Clone a repository from '}
          <Link href={repoURL} target="_blank">
            github.com
          </Link>
          {':'}

          {httpEnabled && (
            <TextSelectCopy
              mt="2"
              mb="2"
              text={`tsh git clone https://github.com/${organization}/<repo>.git`}
            />
          )}

          {sshEnabled && (
            <>
              {httpEnabled && (
                <Text mb="1" color="text.slightlyMuted" fontSize="12px">
                  Or using SSH:
                </Text>
              )}
              <TextSelectCopy
                mb="2"
                text={`tsh git clone git@github.com:${organization}/<repo>.git`}
              />
            </>
          )}

          {httpEnabled && (
            <>
              <Text mb="1" color="text.slightlyMuted" fontSize="12px">
                Use the GitHub CLI:
              </Text>
              <TextSelectCopy mb="2" text="tsh gh -- api /user" />
            </>
          )}

          Configure an existing repository:
          <TextSelectCopy mt="1" mb="2" text="tsh git config update" />

          Once configured, use 'git' as normal.
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
