import { ReactNode, useCallback, useEffect, useMemo, useState } from 'react';
import { useHistory } from 'react-router';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  H2,
  Indicator,
  Text,
} from 'design';
import { FeatureName } from 'design/constants';
import * as Icons from 'design/Icon';
import { useAsync } from 'shared/hooks/useAsync';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { PluginIcon } from 'e-teleport/Integrations/IntegrationEnroll/IntegrationPick/PluginIcon';
import { CleanupDialogue } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/CleanupDialogue';
import { OktaIntegrationSetUpContextProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  getCompletedOktaIntegrationLevel,
  getNextOktaIntegrationLevel,
  OktaIntegrationLabelValues,
  OktaIntegrationLevel,
  oktaIntegrationLevels,
  StyledBox,
  UpsellBulletList,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { SetUpAppGroupSync } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpAppGroupSync';
import { SetUpScim } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpScim';
import { SetUpSSO } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpSSO';
import { SetUpUserSync } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpUserSync';
import { pluginsService } from 'e-teleport/services/plugins';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { Route, Switch, useParams } from 'teleport/components/Router';
import { addIndexToViews } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';
import { ApiError } from 'teleport/services/api/parseError';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import { CtaEvent } from 'teleport/services/userEvent';

export const OktaIntegrationSetUp = () => {
  const history = useHistory();
  const { subPage } = useParams<{ subPage?: string }>();
  const [needsCleanupAttempt, fetchNeedsCleanup] = useAsync(
    useCallback(() => pluginsService.checkPluginRequiresCleanup('okta'), [])
  );
  const [existingPluginAttempt, fetchExistingPlugin, setExistingPluginAttempt] =
    useAsync(
      useCallback(
        () =>
          pluginsService
            .fetchPlugin('okta')
            .then(pluginRes => {
              setCompletedSteps(getCompletedOktaIntegrationLevel(pluginRes));
              return pluginRes;
            })
            .catch(e => {
              // If 404, plugin doesn't exist yet & we can ignore, as this fetch is just
              // to check if the plugin already exists & what steps have been set up.
              if (e instanceof ApiError && e.response.status === 404) {
                return undefined;
              }
              throw e;
            }),
        []
      )
    );
  const setExistingPlugin = (
    plugin: Plugin<PluginOktaSpec, PluginStatusOkta>
  ) =>
    setExistingPluginAttempt({
      status: 'success',
      data: plugin,
      statusText: '',
    });
  const [showCleanUpModal, setShowCleanUpModal] = useState(false);
  const [completedSteps, setCompletedSteps] = useState<
    Record<OktaIntegrationLevel, boolean>
  >({} as Record<OktaIntegrationLevel, boolean>);
  const highestCompletedStep = useMemo<OktaIntegrationLevel | undefined>(() => {
    for (const step of [
      OktaIntegrationLevel.APP_GROUP_SYNC,
      OktaIntegrationLevel.USER_SYNC,
      OktaIntegrationLevel.SCIM,
      OktaIntegrationLevel.SSO,
    ]) {
      if (completedSteps[step]) {
        return step;
      }
    }
    return undefined;
  }, [completedSteps]);

  const navigationViews = useMemo(
    () =>
      addIndexToViews(
        Object.values(oktaIntegrationLevels).map(l => ({
          title: l.shortName,
          component: null,
        }))
      ),
    []
  );

  const onContinue = async (level: OktaIntegrationLevel) => {
    // If SSO is already set up, plugin exists & doesn't require cleanup.
    if (level !== OktaIntegrationLevel.SSO) {
      history.push(cfg.oss.getIntegrationEnrollRoute('okta', level));
      return;
    }

    const [needsCleanup] = await fetchNeedsCleanup();
    if (!needsCleanup) {
      history.push(cfg.oss.getIntegrationEnrollRoute('okta', level));
    } else {
      setShowCleanUpModal(true);
    }
  };

  // On mount, fetch any existing Okta plugin
  useEffect(() => {
    if (existingPluginAttempt.status === '') {
      void fetchExistingPlugin();
    }
  }, [existingPluginAttempt.status, fetchExistingPlugin]);

  if (['', 'processing'].includes(existingPluginAttempt.status)) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  return (
    <>
      {showCleanUpModal && (
        <CleanupDialogue
          onClose={() => setShowCleanUpModal(false)}
          pluginKind="okta"
        />
      )}
      <Box my={4}>
        <Navigation
          currentStep={(oktaIntegrationLevels[subPage]?.level ?? 0) - 1}
          views={navigationViews}
          startWithIcon={{
            title: 'Okta Integration',
            component: <PluginIcon type="okta" size={16} />,
          }}
        />
      </Box>
      <Flex flexDirection="column" gap={4}>
        {needsCleanupAttempt.status === 'error' && (
          <Alert kind="danger" mb={0}>
            {getErrMessage(needsCleanupAttempt.error)}
          </Alert>
        )}
        {existingPluginAttempt.status === 'error' && (
          <Alert kind="danger" mb={0}>
            {getErrMessage(existingPluginAttempt.error)}
          </Alert>
        )}
        <OktaIntegrationSetUpContextProvider
          plugin={existingPluginAttempt.data}
          setPlugin={setExistingPlugin}
          startFrom={highestCompletedStep}
        >
          <Switch>
            <Route
              path={cfg.oss.getIntegrationEnrollRoute(
                'okta',
                OktaIntegrationLevel.SSO
              )}
              component={SetUpSSO}
              exact
            />
            {cfg.oss.entitlements.Identity.enabled && [
              <Route
                key={OktaIntegrationLevel.SCIM}
                path={cfg.oss.getIntegrationEnrollRoute(
                  'okta',
                  OktaIntegrationLevel.SCIM
                )}
                component={SetUpScim}
                exact
              />,
              <Route
                key={OktaIntegrationLevel.USER_SYNC}
                path={cfg.oss.getIntegrationEnrollRoute(
                  'okta',
                  OktaIntegrationLevel.USER_SYNC
                )}
                component={SetUpUserSync}
                exact
              />,
              <Route
                key={OktaIntegrationLevel.APP_GROUP_SYNC}
                path={cfg.oss.getIntegrationEnrollRoute(
                  'okta',
                  OktaIntegrationLevel.APP_GROUP_SYNC
                )}
                component={SetUpAppGroupSync}
                exact
              />,
            ]}
            <Route>
              <Overview
                completedSteps={completedSteps}
                highestCompletedStep={highestCompletedStep}
                onContinue={onContinue}
              />
            </Route>
          </Switch>
        </OktaIntegrationSetUpContextProvider>
      </Flex>
    </>
  );
};

const Overview = ({
  completedSteps,
  highestCompletedStep,
  onContinue,
}: {
  completedSteps: Record<OktaIntegrationLevel, boolean>;
  highestCompletedStep: OktaIntegrationLevel;
  onContinue: (level: OktaIntegrationLevel) => void;
}) => {
  const hasIdentity = cfg.oss.entitlements.Identity.enabled;
  const nextLevel = getNextOktaIntegrationLevel(highestCompletedStep);

  return (
    <>
      <Box maxWidth="800px">
        <H1 mb={2}>Okta Integration Overview</H1>
        <Text typography="subtitle1">
          The Okta integration has 4 steps. We recommend the full integration,
          which will enable you to manage app access within Teleport, but you
          can set up each step at a time, as desired.
        </Text>
      </Box>
      <Flex flexDirection="column" gap={3} width="100%">
        {/* Show steps separately if user has Identity, otherwise show upsell */}
        {OktaIntegrationLabelValues.map(level =>
          hasIdentity || level === OktaIntegrationLevel.SSO ? (
            <IntegrationLevelTile
              key={level}
              level={level}
              completed={completedSteps[level]}
            />
          ) : null
        )}
        {!hasIdentity && <IntegrationLevelTile cta />}
      </Flex>
      <Flex flexDirection="row" alignItems="center" gap={3}>
        {nextLevel ? (
          <ButtonPrimary onClick={() => onContinue(nextLevel)}>
            Set up {oktaIntegrationLevels[nextLevel].shortName}
          </ButtonPrimary>
        ) : (
          <ButtonPrimary
            as={Link}
            to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')}
          >
            View Integration Status Page
          </ButtonPrimary>
        )}
        <ButtonSecondary as={Link} to={cfg.oss.getIntegrationEnrollRoute()}>
          Back
        </ButtonSecondary>
      </Flex>
    </>
  );
};

const getIntegrationLevelTileDetails = (
  cta: boolean,
  level?: OktaIntegrationLevel
) => {
  if (cta) {
    return {
      bullets: Object.values(oktaIntegrationLevels).reduce(
        (acc, level) => (level.level === 1 ? acc : [...acc, ...level.bullets]),
        []
      ),
      title: 'SCIM, User Sync, and Apps and Group Assignments',
    };
  }
  return {
    bullets: oktaIntegrationLevels[level].bullets,
    title: oktaIntegrationLevels[level].name,
  };
};

const IntegrationLevelTile = ({
  level,
  completed,
  cta,
}:
  | {
      level: OktaIntegrationLevel;
      completed?: boolean;
      cta?: undefined;
    }
  | {
      level?: undefined;
      completed?: undefined;
      cta: true;
    }) => {
  const { title, bullets } = getIntegrationLevelTileDetails(!!cta, level);
  let chip: ReactNode = null;
  let bulletColor = 'text.muted';

  if (!cta && completed) {
    bulletColor = 'interactive.solid.success.default';
    chip = (
      <Chip backgroundColor="interactive.tonal.success.0">
        <Icons.CircleCheck
          size="small"
          color="interactive.solid.success.default"
        />
        <Text color="interactive.solid.success.default" typography="body3">
          Enabled
        </Text>
      </Chip>
    );
  } else if (cta || level === OktaIntegrationLevel.APP_GROUP_SYNC) {
    chip = (
      <Chip backgroundColor="interactive.solid.primary.default">
        <Icons.ChatCircleSparkle size="small" color="text.primaryInverse" />
        <Text color="text.primaryInverse" typography="body3">
          Recommended
        </Text>
      </Chip>
    );
  }

  return (
    <StyledBox
      gap={2}
      header={
        <Flex flexDirection="row" gap={2} alignItems="center">
          <H2>{title}</H2>
          {chip}
        </Flex>
      }
    >
      <UpsellBulletList bullets={bullets} color={bulletColor} />
      {cta && (
        <ButtonLockedFeature
          event={CtaEvent.CTA_OKTA_SCIM}
          mt={3}
          width="fit-content"
        >
          Unlock the Full Integration with {FeatureName.IdentityGovernance}
        </ButtonLockedFeature>
      )}
    </StyledBox>
  );
};

const Chip = styled(Flex).attrs({
  flexDirection: 'row',
  alignItems: 'center',
  gap: 1,
})`
  padding: ${({ theme }) =>
    `${theme.space[1]}px ${theme.space[3]}px ${theme.space[1]}px ${theme.space[2]}px`};
  border-radius: 999px;
`;
