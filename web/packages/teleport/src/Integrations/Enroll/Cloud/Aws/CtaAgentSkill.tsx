/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Text, Flex, H3, Box, Stack } from '@gravitational/design-system';
import { useState } from 'react';

import { ButtonSecondary, Link } from 'design';
import { Info } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import { ArrowSquareOut, Sparkle } from 'design/Icon';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import cfg from 'teleport/config';

import { CircleNumber } from '../Shared';

export function CtaAgentSkill() {
  const [showDialog, setShowDialog] = useState(false);
  return (
    <>
      <Info
        primaryAction={{
          content: (
            <Flex gap={2} alignItems={'center'}>
              <Sparkle size="small" /> Use Your AI Agent
            </Flex>
          ),
          onClick: () => setShowDialog(true),
        }}
      >
        <Text>Did you know?</Text>
        <Text textStyle="body2">
          You can do this task using an AI Skill for your AI Agent
        </Text>
      </Info>
      {showDialog && <AgentSkillDialog onClose={() => setShowDialog(false)} />}
    </>
  );
}

function AgentSkillDialog({ onClose }: { onClose(): void }) {
  return (
    <Dialog onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>
          <Sparkle /> AI Skill for Your AI Agent
        </DialogTitle>
      </DialogHeader>
      <DialogContent width={'550px'}>
        <Stack gap={2}>
          <H3>Teleport Discovery Skill</H3>
          <Text fontWeight={400}>
            What it does: Guides you through configuring AWS discovery and
            generates the Terraform needed to discover and enroll resources. It
            never applies changes without your approval—you can run the
            Terraform yourself or ask your AI agent to run it.
          </Text>

          <Text color="text.slightlyMuted" mb={-2}>
            Works with coding agents that support skills
          </Text>
          <Flex gap={3}>
            <Link
              target="_blank"
              href="https://agentskills.io/home#what-are-agent-skills"
            >
              <Flex gap={1}>
                What are Skills <ArrowSquareOut size="small" />
              </Flex>
            </Link>
            <Link
              target="_blank"
              href="https://github.com/gravitational/teleport/tree/master/skills#teleport-discovery"
            >
              <Flex gap={1}>
                View on GitHub
                <ArrowSquareOut size="small" />
              </Flex>
            </Link>
          </Flex>

          <Flex alignItems="start" fontWeight="medium" my={3}>
            <CircleNumber css={{ marginTop: '4px' }}>1</CircleNumber>
            <Box>
              Install the skill for your agent
              <Box>
                <TextSelectCopyMulti
                  lines={[
                    {
                      text: 'npx skills add gravitational/teleport/skills/teleport-discovery',
                    },
                  ]}
                />
              </Box>
            </Box>
          </Flex>

          <Flex alignItems="start" fontWeight="medium" mb={1}>
            <CircleNumber css={{ marginTop: '4px' }}>2</CircleNumber>
            <Box>
              Ask your agent to use the skill
              <Text
                color="text.slightlyMuted"
                fontSize={1}
                fontWeight={'regular'}
              >
                Open your agent in the folder where you keep Terraform, on a
                machine with AWS access.
              </Text>
              <Box>
                <TextSelectCopyMulti
                  lines={[
                    {
                      text: `Help me set up AWS resource discovery in Teleport.\n\nTeleport cluster: ${cfg.proxyCluster}`,
                    },
                  ]}
                />
              </Box>
            </Box>
          </Flex>
        </Stack>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
