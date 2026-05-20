import { ReactNode } from 'react';

import {
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  H2,
  ResourceIcon,
  Stack,
  Text,
} from 'design';
import { Bell, NewTab } from 'design/Icon';

import cfg from 'e-teleport/config';
import { FeatureBox } from 'teleport/components/Layout/Layout';
import { useNoMinWidth } from 'teleport/Main';

import { BeamsCard } from './components';

const PAGE_MAX_WIDTH = 800;
const CALLOUT_ICON_SIZE = 32;

type Callout = {
  icon: ReactNode;
  heading?: ReactNode;
  body: ReactNode;
  cta: {
    label: string;
    href: string;
    variant: 'primary' | 'secondary';
  };
};

const callouts: Callout[] = [
  {
    icon: <Bell size={CALLOUT_ICON_SIZE} />,
    heading: (
      <H2>You&apos;re on the 14-day trial of Beams, powered by Teleport.</H2>
    ),
    body: 'Extend your trial or discuss commercial options by setting up time with our team.',
    cta: {
      label: 'Schedule time with a Teleporter',
      href: cfg.beamsSchedulerUrl,
      variant: 'primary',
    },
  },
  {
    icon: <ResourceIcon name="slack" width={`${CALLOUT_ICON_SIZE}px`} />,
    body: (
      <>
        Share feedback at <strong>#beams</strong> in the Teleport Community
        Slack.
      </>
    ),
    cta: {
      label: 'Join Teleport on Slack',
      href: cfg.communitySlackUrl,
      variant: 'secondary',
    },
  },
];

export function BeamsFeedback() {
  useNoMinWidth();

  return (
    <FeatureBox>
      <Flex
        flexDirection="column"
        maxWidth={PAGE_MAX_WIDTH}
        mx="auto"
        width="100%"
        gap={3}
      >
        <H1 mt={5} mb={2}>
          Feedback
        </H1>
        {callouts.map(({ icon, heading, body, cta }) => {
          const Button =
            cta.variant === 'primary' ? ButtonPrimary : ButtonSecondary;
          return (
            <BeamsCard key={cta.href}>
              <Stack alignItems="center" gap={4}>
                {icon}
                {heading}
                <Text typography="body1" textAlign="center">
                  {body}
                </Text>
                <Button
                  as="a"
                  size="large"
                  href={cta.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={`${cta.label} (opens in a new tab)`}
                  gap={2}
                >
                  {cta.label}
                  <NewTab size="small" />
                </Button>
              </Stack>
            </BeamsCard>
          );
        })}
      </Flex>
    </FeatureBox>
  );
}
