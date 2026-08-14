import { ComponentType, lazy, useCallback, useState } from 'react';
import { type FallbackProps } from 'react-error-boundary';
import { useTheme } from 'styled-components';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { Theme } from 'design/theme';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';
import {
  useInfoGuide,
  type InfoGuideConfig,
} from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuideContext';
import { getErrorMessage } from 'shared/utils/error';

import cfg, { EnterpriseConfig } from 'e-teleport/config';
import CloudService from 'e-teleport/services/cloud';
import useTeleportE from 'e-teleport/useTeleportE';
import { useNoMinWidth } from 'teleport/Main';
import { userEventService } from 'teleport/services/userEvent';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { loadCloud } from './loader';

const APP_FILE = 'index.js';

// Module-level variable so the bundle is only fetched once across remounts.
// Replaced on each retry so a successful retry survives component unmount/remount
// without hitting React.lazy's cached rejection on the previous instance.
let currentCloudUI = lazy<ComponentType<CloudUIProps>>(() =>
  loadCloud(APP_FILE)
);

export type CloudUIProps = {
  clusterId: string;
  cfg: EnterpriseConfig;
  backUrl: string;
  theme: Theme;
  urlPrefix?: string;
  cloudService: CloudService;
  setInfoGuideConfig?: (cfg: InfoGuideConfig | null) => void;
  userEventService?: typeof userEventService;
  stripeManaged?: boolean;
};

function CloudLoading() {
  return (
    <Box p={4}>
      <Indicator delay="none" />
    </Box>
  );
}

export function Cloud() {
  useNoMinWidth();
  const theme = useTheme();
  const { clusterId } = useStickyClusterId();
  const ctx = useTeleportE();
  const { setInfoGuideConfig } = useInfoGuide();
  // Changing this counter forces Cloud to re-render, so the const below
  // re-reads the updated currentCloudUI after a retry.
  const [, setRetryCount] = useState(0);
  // Capture as a capitalized const for JSX — module-level lets can't be used
  // directly as JSX tags. This read happens at render time, so it always picks
  // up the latest value of currentCloudUI (including after a retry).
  const CloudUI = currentCloudUI;

  const cloudError = useCallback(
    ({ error, resetErrorBoundary }: FallbackProps) => (
      <Box p={4}>
        <Danger
          primaryAction={{
            content: 'Retry',
            onClick: () => {
              // Replace the module-level instance before triggering a re-render
              // so the fresh lazy is used on the next render and on any
              // subsequent remounts (avoiding the cached rejection).
              currentCloudUI = lazy<ComponentType<CloudUIProps>>(() =>
                loadCloud(APP_FILE)
              );
              setRetryCount(c => c + 1);
              resetErrorBoundary();
            },
          }}
        >
          {getErrorMessage(error)}
        </Danger>
      </Box>
    ),
    []
  );

  const backUrl = '/';

  return (
    <ErrorSuspenseWrapper
      errorComponent={cloudError}
      loadingComponent={CloudLoading}
    >
      <Flex
        data-scrollbar="default"
        width="100%"
        height="100%"
        overflowY="auto"
        flexDirection="column"
      >
        <CloudUI
          clusterId={clusterId}
          cfg={cfg}
          backUrl={backUrl}
          theme={theme}
          urlPrefix={cfg.routes.cloud.root}
          cloudService={ctx.cloudService}
          setInfoGuideConfig={setInfoGuideConfig}
          userEventService={userEventService}
          stripeManaged={cfg.oss.isStripeManaged}
        />
      </Flex>
    </ErrorSuspenseWrapper>
  );
}
