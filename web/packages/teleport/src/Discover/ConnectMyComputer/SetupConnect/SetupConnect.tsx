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

import { useCallback, useEffect, useRef, useState, type JSX } from 'react';
import { Link } from 'react-router-dom';

import { Alert, Box, Flex, H3, Subtitle3, Text } from 'design';
import { ButtonSecondary } from 'design/Button';
import * as Icons from 'design/Icon';
import { getPlatform } from 'design/platform';
import { P } from 'design/Text/Text';
import * as connectMyComputer from 'shared/connectMyComputer';
import { makeDeepLinkWithSafeInput } from 'shared/deepLinks';

import {
  DownloadConnect,
  getConnectDownloadLinks,
} from 'teleport/components/DownloadConnect/DownloadConnect';
import cfg from 'teleport/config';
import { ActionButtons, Header, StyledBox } from 'teleport/Discover/Shared';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import { Node } from 'teleport/services/nodes';
import useTeleport from 'teleport/useTeleport';

import type { AgentStepProps } from '../../types';

export function SetupConnect(
  props: AgentStepProps & {
    pingInterval?: number;
    showHintTimeout?: number;
  }
) {
  const pingInterval = props.pingInterval || 1000 * 3; // 3 seconds
  const showHintTimeout = props.showHintTimeout || 1000 * 60 * 5; // 5 minutes

  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const { cluster, username } = ctx.storeUser.state;
  const platform = getPlatform();
  const downloadLinks = getConnectDownloadLinks(platform, cluster.proxyVersion);
  const connectMyComputerDeepLink = makeDeepLinkWithSafeInput({
    proxyHost: cluster.publicURL,
    username,
    path: '/connect_my_computer',
    searchParams: {},
  });
  const [showHint, setShowHint] = useState(false);

  const { node, isPolling } = usePollForConnectMyComputerNode({
    username,
    clusterId,
    // If reloadUser is set to true, the polling callback takes longer to finish so let's increase
    // the polling interval as well.
    pingInterval: showHint ? pingInterval * 2 : pingInterval,
    // Completing the Connect My Computer setup in Connect causes the user to gain a new role. That
    // role grants access to nodes labeled with `teleport.dev/connect-my-computer/owner:
    // <current-username>`.
    //
    // In certain cases, that role might be the only role which grants the user the visibility of
    // the Connect My Computer node. For example, if the user doesn't have a role like the built-in
    // access which gives blanket access to all nodes, the user won't be able to see the node until
    // they have the Connect My Computer role in their cert.
    //
    // As such, if we don't reload the cert during polling, it might never see the node. So let's
    // flip it to true after a timeout.
    reloadUser: showHint,
  });

  // TODO(ravicious): Take these from the context rather than from props.
  const { agentMeta, updateAgentMeta, nextStep } = props;
  const handleNextStep = () => {
    if (!node) {
      return;
    }

    updateAgentMeta({
      ...agentMeta,
      // Node is an oddity in that the hostname is the more
      // user identifiable resource name and what user expects
      // as the resource name.
      resourceName: node.hostname,
      node,
    });
    nextStep();
  };

  useEffect(() => {
    if (isPolling) {
      const id = window.setTimeout(() => setShowHint(true), showHintTimeout);

      return () => window.clearTimeout(id);
    }
  }, [isPolling, showHintTimeout]);

  let pollingStatus: JSX.Element;
  if (showHint && !node) {
    const details = (
      <Flex flexDirection="column" gap={3}>
        <P>
          There are a couple of possible reasons for why we haven&apos;t been
          able to detect your computer.
        </P>

        <ul
          css={`
            margin: 0;
            padding-left: ${p => p.theme.space[3]}px;
          `}
        >
          <li>
            <Text>
              You did not start Connect My Computer in Teleport Connect yet.
            </Text>
          </li>
          <li>
            <Text>
              The Teleport agent started by Teleport Connect could not join this
              Teleport cluster. Check if the Connect My Computer tab in Teleport
              Connect shows any error messages.
            </Text>
          </li>
          <li>
            <Text>
              The computer you are trying to add has already joined the Teleport
              cluster before you entered this page. If that&apos;s the case, you
              can go back to the{' '}
              <Link to={cfg.getUnifiedResourcesRoute(clusterId)}>
                resources page
              </Link>{' '}
              and connect to it.
            </Text>
          </li>
        </ul>

        <P>
          We&apos;ll continue to look for the computer while you diagnose the
          issue.
        </P>
      </Flex>
    );
    pollingStatus = (
      <Alert
        alignItems="flex-start"
        kind="warning"
        dismissible={false}
        details={details}
      >
        We&apos;re still looking for your computer
      </Alert>
    );
  } else if (node) {
    pollingStatus = (
      <Alert kind="success" dismissible={false}>
        Your computer, <strong>{node.hostname}</strong>, has been detected!
      </Alert>
    );
  } else {
    pollingStatus = (
      <Alert kind="neutral" icon={Icons.Restore} dismissible={false}>
        After your computer is connected to the cluster, we’ll automatically
        detect it.
      </Alert>
    );
  }

  return (
    <Flex flexDirection="column" alignItems="flex-start" mb={2} gap={4}>
      <Header>Set Up Teleport Connect</Header>

      <StyledBox>
        <header>
          <H3>Step 1</H3>
          <Subtitle3 mb={3}>Download and Install Teleport Connect</Subtitle3>
        </header>

        <P>
          Teleport Connect is a native desktop application for browsing and
          accessing your resources. It can also connect your computer to the
          cluster as an SSH resource.
        </P>
        <P mb={3}>
          Once you’ve downloaded Teleport Connect, run the installer to add it
          to your computer’s applications.
        </P>

        <Flex flexWrap="wrap" alignItems="baseline" gap={2}>
          <DownloadConnect downloadLinks={downloadLinks} />
          <P>Already have Teleport Connect? Skip to the next step.</P>
        </Flex>
      </StyledBox>

      <StyledBox>
        <header>
          <H3>Step 2</H3>
          <Subtitle3 mb={3}>Sign In and Connect My Computer</Subtitle3>
        </header>

        <P mb={3}>
          The button below will open Teleport Connect. Once you are logged in,
          Teleport Connect will prompt you to connect your computer.
        </P>

        <ButtonSecondary as="a" href={connectMyComputerDeepLink}>
          Sign In & Connect My Computer
        </ButtonSecondary>
      </StyledBox>

      <Box width="100%">{pollingStatus}</Box>

      <ActionButtons
        onProceed={handleNextStep}
        disableProceed={!node}
        onPrev={props.prevStep}
      />
    </Flex>
  );
}

/**
 * usePollForConnectMyComputerNode polls for a Connect My Computer node that joined the cluster
 * after starting opening the SetupConnect step.
 *
 * The first polling request fills out a set of node IDs (initialNodeIdsRef). Subsequent requests
 * check the returned nodes against this set. The hook stops polling as soon as a node that is not
 * in the set was found.
 *
 * There can be multiple nodes matching the search criteria and we want the one that was added only
 * after the user has started the guided flow, hence why we need to keep track of the IDs in a set.
 *
 * Unlike the DownloadScript step responsible for adding a server, we don't have a unique ID that
 * identifies the node that the user added after following the steps from the guided flow. In
 * theory, we could make the deep link button pass such ID to Connect, but the user would still be
 * able to just launch the app directly and not use the link.
 *
 * Because of that, we must depend on comparing the list of nodes against the initial set of IDs.
 */
export const usePollForConnectMyComputerNode = (args: {
  username: string;
  clusterId: string;
  reloadUser: boolean;
  pingInterval: number;
}): {
  node: Node | undefined;
  isPolling: boolean;
} => {
  const ctx = useTeleport();
  const [isPolling, setIsPolling] = useState(true);
  const initialNodeIdsRef = useRef<Set<string>>(null);

  const node = usePoll(
    useCallback(
      async signal => {
        if (args.reloadUser) {
          await ctx.userService.reloadUser(signal);
        }

        const request = {
          query: `labels["${connectMyComputer.NodeOwnerLabel}"] == "${args.username}"`,
          // An arbitrary limit where we bank on the fact that no one is going to have 50 Connect My
          // Computer nodes assigned to them running at the same time.
          limit: 50,
        };

        const response = await ctx.nodeService.fetchNodes(
          args.clusterId,
          request,
          signal
        );

        // Fill out the set with node IDs if it's empty.
        if (initialNodeIdsRef.current === null) {
          initialNodeIdsRef.current = new Set(
            response.agents.map(agent => agent.id)
          );
          return null;
        }

        // On subsequent requests, compare the nodes from the response against the set.
        const node = response.agents.find(
          agent => !initialNodeIdsRef.current.has(agent.id)
        );

        if (node) {
          setIsPolling(false);
          return node;
        }
      },
      [
        ctx.nodeService,
        ctx.userService,
        args.clusterId,
        args.username,
        args.reloadUser,
      ]
    ),
    isPolling,
    args.pingInterval
  );

  return { node, isPolling };
};
