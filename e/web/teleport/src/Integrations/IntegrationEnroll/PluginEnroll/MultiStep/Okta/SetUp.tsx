import { useQueryClient } from '@tanstack/react-query';
import { ReactNode, useCallback, useMemo, useState } from 'react';
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
import * as Icons from 'design/Icon';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { PluginIcon } from 'e-teleport/Integrations/IntegrationEnroll/IntegrationPick/PluginIcon';
import { CleanupDialogue } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/CleanupDialogue';
import {
  OktaIntegrationSetUpContextProvider,
  useOktaIntegrationSetUpContext,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  getCompletedOktaIntegrationStepTypes,
  getOktaIntegrationSteps,
  OktaIntegrationStepType,
  OktaLevelProductRequirement,
  UpsellBulletList,
  type OktaIntegrationLevelStep,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { SetUpAppGroupSync } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpAppGroupSync';
import { SetupIdentitySecuritySync } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetupIdentitySecuritySync';
import { SetUpScim } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpScim';
import { SetUpSSO } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpSSO';
import { SetUpUserSync } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpUserSync';
import { StyledBox } from 'e-teleport/Integrations/Shared';
import {
  createCheckPluginRequiresCleanupQuery,
  useCheckPluginRequiresCleanup,
  useFetchPlugin,
} from 'e-teleport/services/plugins/hooks';
import { useTeleport } from 'teleport';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { Route, Switch, useParams } from 'teleport/components/Router';
import { addIndexToViews } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';
import { ApiError } from 'teleport/services/api/parseError';
import { storageService } from 'teleport/services/storageService';
import { CtaEvent } from 'teleport/services/userEvent';

function is404Error(error: unknown): boolean {
  return error instanceof ApiError && error.response.status === 404;
}

export const OktaIntegrationSetUp = () => {
  const ctx = useTeleport();
  const history = useHistory();
  const { subPage } = useParams<{ subPage?: string }>();

  const accessGraphEnabled =
    storageService.getAccessGraphEnabled() && ctx.getFeatureFlags().accessGraph;

  const queryClient = useQueryClient();

  const needsCleanup = useCheckPluginRequiresCleanup('okta', {
    enabled: false,
  });

  const existingPlugin = useFetchPlugin<'okta'>('okta');

  const completedStepTypes = useMemo(
    () =>
      existingPlugin.isSuccess
        ? getCompletedOktaIntegrationStepTypes(existingPlugin.data)
        : [],
    [existingPlugin.data, existingPlugin.isSuccess]
  );

  const [showCleanUpModal, setShowCleanUpModal] = useState(false);

  const oktaIntegrationSteps = useMemo(
    () => getOktaIntegrationSteps(accessGraphEnabled, cfg.oss.isCloud),
    [accessGraphEnabled]
  );

  const highestCompletedStepType = useMemo<
    OktaIntegrationStepType | undefined
  >(() => {
    for (const step of oktaIntegrationSteps) {
      if (completedStepTypes.includes(step.type)) {
        return step.type;
      }
    }

    return undefined;
  }, [completedStepTypes, oktaIntegrationSteps]);

  const navigationViews = useMemo(
    () =>
      addIndexToViews(
        oktaIntegrationSteps
          .filter(step => step.enabled)
          .map(l => ({
            component: null,
            level: l.type,
            title: l.shortName,
          }))
      ),
    [oktaIntegrationSteps]
  );

  const onContinue = useCallback(
    async (level: OktaIntegrationStepType) => {
      // If SSO is already set up, plugin exists & doesn't require cleanup.
      if (level !== OktaIntegrationStepType.Sso) {
        history.push(cfg.oss.getIntegrationEnrollRoute('okta', level));
        return;
      }

      const needsCleanup = await queryClient.fetchQuery(
        createCheckPluginRequiresCleanupQuery('okta')
      );

      if (!needsCleanup) {
        history.push(cfg.oss.getIntegrationEnrollRoute('okta', level));
      } else {
        setShowCleanUpModal(true);
      }
    },
    [history, queryClient]
  );

  if (existingPlugin.isFetching) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  const currentStep = navigationViews.find(step => step.level === subPage);

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
          currentStep={currentStep ? currentStep.index : -1}
          views={navigationViews}
          startWithIcon={{
            title: 'Okta Integration',
            component: <PluginIcon type="okta" size={16} />,
          }}
        />
      </Box>
      <Flex flexDirection="column" gap={4}>
        {needsCleanup.isError && (
          <Alert kind="danger" mb={0}>
            {getErrMessage(needsCleanup.error)}
          </Alert>
        )}
        {existingPlugin.isError && !is404Error(existingPlugin.error) && (
          <Alert kind="danger" mb={0}>
            {getErrMessage(existingPlugin.error)}
          </Alert>
        )}
        <OktaIntegrationSetUpContextProvider
          completedStepTypes={completedStepTypes}
          plugin={existingPlugin.data}
          steps={oktaIntegrationSteps}
          startFrom={highestCompletedStepType}
        >
          <Switch>
            <Route
              path={cfg.oss.getIntegrationEnrollRoute(
                'okta',
                OktaIntegrationStepType.Sso
              )}
              component={SetUpSSO}
              exact
            />
            {accessGraphEnabled && (
              <Route
                key={OktaIntegrationStepType.IdentitySecuritySync}
                path={cfg.oss.getIntegrationEnrollRoute(
                  'okta',
                  OktaIntegrationStepType.IdentitySecuritySync
                )}
                component={SetupIdentitySecuritySync}
                exact
              />
            )}
            {cfg.oss.entitlements.Identity.enabled && [
              <Route
                key={OktaIntegrationStepType.Scim}
                path={cfg.oss.getIntegrationEnrollRoute(
                  'okta',
                  OktaIntegrationStepType.Scim
                )}
                component={SetUpScim}
                exact
              />,
              <Route
                key={OktaIntegrationStepType.UserSync}
                path={cfg.oss.getIntegrationEnrollRoute(
                  'okta',
                  OktaIntegrationStepType.UserSync
                )}
                component={SetUpUserSync}
                exact
              />,
              <Route
                key={OktaIntegrationStepType.AppGroupSync}
                path={cfg.oss.getIntegrationEnrollRoute(
                  'okta',
                  OktaIntegrationStepType.AppGroupSync
                )}
                component={SetUpAppGroupSync}
                exact
              />,
            ]}
            <Route>
              <Overview
                completedStepTypes={completedStepTypes}
                highestCompletedStepType={highestCompletedStepType}
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
  completedStepTypes,
  highestCompletedStepType,
  onContinue,
}: {
  completedStepTypes: OktaIntegrationStepType[];
  highestCompletedStepType: OktaIntegrationStepType;
  onContinue: (type: OktaIntegrationStepType) => void;
}) => {
  const { steps, getNextStep } = useOktaIntegrationSetUpContext();

  const enabledSteps = useMemo(
    () => steps.filter(step => step.enabled),
    [steps]
  );

  const nextStep = getNextStep(highestCompletedStepType);

  return (
    <>
      <Box maxWidth="800px">
        <H1 mb={2}>Okta Integration Overview</H1>
        <Text typography="subtitle1">
          The Okta integration has {steps.length} steps. We recommend the full
          integration, which will enable you to manage app access within
          Teleport, but you can set up each step at a time, as desired.
        </Text>
      </Box>
      <Flex flexDirection="column" gap={3} width="100%">
        {enabledSteps.map(config => (
          <IntegrationLevelTile
            key={config.type}
            config={config}
            completed={completedStepTypes.includes(config.type)}
            completedStepTypes={completedStepTypes}
          />
        ))}

        <IntegrationLevelCTA />
      </Flex>
      <Flex flexDirection="row" alignItems="center" gap={3}>
        {nextStep ? (
          <ButtonPrimary onClick={() => onContinue(nextStep.type)}>
            Set up {nextStep.shortName}
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

function productNameToTitle(productName: OktaLevelProductRequirement) {
  switch (productName) {
    case OktaLevelProductRequirement.IdentitySecurity:
      return 'Audit Log Sync with Identity Security';

    case OktaLevelProductRequirement.IdentityGovernance:
      return 'SCIM, User Sync, and Apps and Group Assignments';
  }
}

function productNameToCtaEvent(
  productName: OktaLevelProductRequirement
): CtaEvent {
  switch (productName) {
    case OktaLevelProductRequirement.IdentitySecurity:
      return CtaEvent.CTA_IDENTITY_SECURITY;

    case OktaLevelProductRequirement.IdentityGovernance:
      return CtaEvent.CTA_OKTA_SCIM;
  }
}

function productNameToLockedButtonText(
  productName: OktaLevelProductRequirement
) {
  switch (productName) {
    case OktaLevelProductRequirement.IdentitySecurity:
      return `Unlock Audit Log syncing with ${productName}`;

    case OktaLevelProductRequirement.IdentityGovernance:
      return `Unlock the Full Integration with ${productName}`;
  }
}

function IntegrationLevelCTA() {
  const { completedStepTypes, steps } = useOktaIntegrationSetUpContext();

  const items = useMemo(() => {
    const productRequirementToBullets = new Map<
      OktaLevelProductRequirement,
      ReactNode[]
    >();

    for (const level of steps) {
      if (level.enabled) {
        continue;
      }

      const bullets =
        productRequirementToBullets.get(level.productRequirement) ?? [];

      bullets.push(...level.bullets);

      productRequirementToBullets.set(level.productRequirement, bullets);
    }

    const content: ReactNode[] = [];

    const hasCompletedIdentityGovernance = steps
      .filter(
        config =>
          config.productRequirement ===
          OktaLevelProductRequirement.IdentityGovernance
      )
      .every(config => completedStepTypes.some(step => step === config.type));

    for (const [
      productName,
      bullets,
    ] of productRequirementToBullets.entries()) {
      let chip: ReactNode | null = null;

      if (
        (productName === OktaLevelProductRequirement.IdentityGovernance &&
          !cfg.oss.entitlements.Identity.enabled) ||
        (productName === OktaLevelProductRequirement.IdentitySecurity &&
          hasCompletedIdentityGovernance)
      ) {
        chip = (
          <Chip backgroundColor="interactive.solid.primary.default">
            <Icons.ChatCircleSparkle size="small" color="text.primaryInverse" />
            <Text color="text.primaryInverse" typography="body3">
              Recommended
            </Text>
          </Chip>
        );
      }

      content.push(
        <StyledBox
          key={productName}
          gap={2}
          header={
            <Flex flexDirection="row" gap={2} alignItems="center">
              <H2>{productNameToTitle(productName)}</H2>
              {chip}
            </Flex>
          }
        >
          <UpsellBulletList bullets={bullets} color="text.muted" />

          <ButtonLockedFeature
            event={productNameToCtaEvent(productName)}
            mt={3}
            width="fit-content"
          >
            {productNameToLockedButtonText(productName)}
          </ButtonLockedFeature>
        </StyledBox>
      );
    }

    return content;
  }, [completedStepTypes, steps]);

  return <>{items}</>;
}

const IntegrationLevelTile = ({
  config,
  completed,
  completedStepTypes,
}: {
  config: OktaIntegrationLevelStep;
  completed: boolean;
  completedStepTypes: OktaIntegrationStepType[];
}) => {
  let chip: ReactNode = null;
  let bulletColor = 'text.muted';

  if (completed) {
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
  } else if (
    config.type === OktaIntegrationStepType.AppGroupSync ||
    (completedStepTypes.includes(OktaIntegrationStepType.AppGroupSync) &&
      config.type === OktaIntegrationStepType.IdentitySecuritySync)
  ) {
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
          <H2>{config.name}</H2>
          {chip}
        </Flex>
      }
    >
      <UpsellBulletList bullets={config.bullets} color={bulletColor} />
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
