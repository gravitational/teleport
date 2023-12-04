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

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  AnimatedTerminal,
  TerminalColor,
} from 'shared/components/AnimatedTerminal';
import { TerminalLine } from 'shared/components/AnimatedTerminal/content';

import { KeywordHighlight } from 'shared/components/AnimatedTerminal/TerminalContent';

import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';

import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { ResourceKind } from 'teleport/Discover/Shared';

import type { WindowsDesktopService } from 'teleport/services/desktops';

const startLines = [
  {
    text: 'sudo systemctl start teleport',
    isCommand: true,
  },
  {
    text: 'sudo systemctl status teleport',
    isCommand: true,
  },
  {
    text: `• teleport.service - Teleport Service
   Loaded: loaded
   Active: active (running)`,
    isCommand: false,
    delay: 100,
  },
  {
    text: "logout # We'll take it from here",
    isCommand: true,
  },
  {
    text: '\n',
    isCommand: false,
    delay: 30,
  },
];

const flip = ['_', '_', '_', '-', '`', '`', "'", '´', '-', '_', '_', '_'];

const highlights: KeywordHighlight[] = [
  {
    key: 'keyword',
    color: TerminalColor.Keyword,
    keywords: [
      'sudo',
      'systemctl',
      'active',
      '\\(running\\)',
      '•',
      'wait',
      'logout',
      '✔',
    ],
  },
  {
    key: 'error',
    color: TerminalColor.Error,
    keywords: ['✖', 'Oh', 'no!'],
  },
  {
    key: 'label',
    color: TerminalColor.Label,
    keywords: ['Hostname:', 'Address:'],
  },
];

export function StartTeleportTerminalAnimation() {
  const [animationFinished, setAnimationFinished] = useState(false);
  const [lines, setLines] = useState<TerminalLine[]>([...startLines]);

  const { joinToken } = useJoinTokenSuspender([ResourceKind.Desktop]);
  const { active, result } = usePingTeleport<WindowsDesktopService>(joinToken);

  const [ranConnectingAnimation, setRanConnectingAnimation] = useState(false);
  const [ranConnectedAnimation, setRanConnectedAnimation] = useState(false);

  useEffect(() => {
    if (result && !ranConnectedAnimation) {
      setLines(lines => [
        ...lines,
        {
          isCommand: false,
          text: '',
        },
        {
          isCommand: false,
          text: `✔ Found your Teleport node`,
        },
        {
          isCommand: false,
          text: `  Hostname: ${result.hostname}`,
        },
        {
          isCommand: false,
          text: `   Address: ${result.addr}`,
        },
      ]);

      setRanConnectedAnimation(true);

      return;
    }

    if (ranConnectedAnimation) {
      return;
    }

    if (animationFinished && active && !ranConnectingAnimation) {
      setLines(lines => [
        ...lines,
        {
          text: 'wait your.teleport.instance',
          isCommand: true,
        },
        {
          isCommand: false,
          text: '',
        },
        {
          isCommand: false,
          text: '- Waiting to hear from your Teleport node',
          frames: flip.map(spinner => {
            return () => {
              return {
                text: `${spinner} Waiting to hear from your Teleport node`,
                delay: 70,
              };
            };
          }),
        },
      ]);
    }

    if (animationFinished) {
      setRanConnectingAnimation(active);
    }
  }, [
    result,
    active,
    ranConnectedAnimation,
    ranConnectingAnimation,
    animationFinished,
  ]);

  return (
    <AnimationContainer>
      <AnimatedTerminal
        stopped={result !== null}
        lines={lines}
        startDelay={800}
        highlights={highlights}
        onCompleted={() => setAnimationFinished(true)}
      />
    </AnimationContainer>
  );
}

const AnimationContainer = styled.div`
  --content-height: 400px;
`;
