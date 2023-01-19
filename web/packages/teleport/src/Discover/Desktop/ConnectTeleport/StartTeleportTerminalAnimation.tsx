import React, { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import {
  AnimatedTerminal,
  TerminalColor,
} from 'shared/components/AnimatedTerminal';
import { TerminalLine } from 'shared/components/AnimatedTerminal/content';

import { KeywordHighlight } from 'shared/components/AnimatedTerminal/TerminalContent';

import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';

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
    text: `• teleport.service - Teleport SSH Service
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

  const { active, result, timedOut, timeout } =
    usePingTeleport<WindowsDesktopService>();

  const savedTimeout = useRef(0);
  useEffect(() => {
    if (result) {
      savedTimeout.current = null;

      return;
    }

    savedTimeout.current = timeout;
  }, [timeout, result]);

  const [ranConnectingAnimation, setRanConnectingAnimation] = useState(false);
  const [ranTimedOutAnimation, setRanTimedOutAnimation] = useState(false);
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
              if (Date.now() > savedTimeout.current) {
                return { text: '- Waiting to hear from your Teleport node' };
              }

              const { minutes, seconds } = millisecondsToMinutesSeconds(
                savedTimeout.current - Date.now()
              );

              return {
                text: `${spinner} Waiting to hear from your Teleport node (${minutes}:${seconds} remaining)`,
                delay: 70,
              };
            };
          }),
        },
      ]);
    }

    if (timedOut && !ranTimedOutAnimation) {
      setLines(lines => [
        ...lines,
        {
          isCommand: false,
          text: '',
        },
        {
          isCommand: false,
          text: "✖ Oh no! We couldn't find your Teleport node.",
        },
      ]);
    }

    if (animationFinished) {
      setRanConnectingAnimation(active);
    }

    setRanTimedOutAnimation(timedOut);
  }, [
    result,
    timedOut,
    active,
    ranConnectedAnimation,
    ranTimedOutAnimation,
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

function millisecondsToMinutesSeconds(ms: number) {
  if (ms < 0) {
    return { minutes: 0, seconds: 0 };
  }

  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000)
    .toFixed(0)
    .padStart(2, '0');

  return { minutes, seconds };
}
