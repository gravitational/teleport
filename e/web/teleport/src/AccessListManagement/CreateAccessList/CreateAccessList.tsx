import styled from 'styled-components';

import { Box, Flex, Indicator } from 'design';
import { UserList } from 'design/Icon';

import {
  AccessListManagementContextProvider,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { FeatureBox } from 'teleport/components/Layout';
import { Prompt } from 'teleport/components/Router';
import { Navigation } from 'teleport/components/Wizard/Navigation';

import { defaultSidePanelWidth, GuideContainer } from '../GuideEditor/Shared';
import { TerraformPanel } from '../GuideEditor/Terraform/TerraformPanel';
import { TerraformSideTab } from '../GuideEditor/Terraform/TerraformSideTab';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { SelectGuide } from './SelectGuide/SelectGuide';
import { cancelPrompt } from './types';

export const CreateAccessListWithProvider = () => (
  <AccessListManagementContextProvider>
    <CreateAccessListContextProvider>
      <CreateAccessList />
    </CreateAccessListContextProvider>
  </AccessListManagementContextProvider>
);

export function CreateAccessList() {
  const { guideEditor, oktaPluginAttempt } = useAccessListManagementContext();
  const { preset, currentStep, views, terraform } = guideEditor;

  if (preset == null) {
    return (
      <FeatureBox>
        <SelectGuide />
      </FeatureBox>
    );
  }

  if (
    // Doesn't hurt to wait it out here.
    // Error handling is omitted since this attempt
    // is to check if a okta plugin exists.
    oktaPluginAttempt.status === 'processing'
  ) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  let navTitle = '';
  switch (preset) {
    case 'long-term':
    case 'short-term':
      navTitle = `Access List (${preset === 'long-term' ? 'standing access' : 'JIT access'})`;
      break;
    case '':
      navTitle = 'Access List (custom)';
      break;
    default:
      preset satisfies never;
  }

  function toggleSidePanel() {
    terraform.updateSidePanel(
      terraform.sidePanel === 0 ? defaultSidePanelWidth : 0
    );
  }

  const terraformPanelSupported =
    preset === 'long-term' || preset === 'short-term';

  return (
    <Container>
      <MainViewContainer height="100%">
        <Flex
          minWidth={0}
          mt={3}
          px={6}
          css={`
            position: relative;
          `}
        >
          <Navigation
            currentStep={currentStep}
            views={views}
            startWithIcon={{
              title: navTitle,
              component: <UserList size={20} />,
            }}
          />
          {terraformPanelSupported && (
            <TerraformSideTab
              onClick={toggleSidePanel}
              panelWidth={terraform.sidePanel}
              top="-10px"
            />
          )}
        </Flex>
        <GuideContainer>{views[currentStep].Component}</GuideContainer>
      </MainViewContainer>
      {terraformPanelSupported && terraform.sidePanel > 0 && (
        <TerraformPanel terraform={terraform} disableResizer={false} />
      )}
      {preset && currentStep < views.length - 1 && (
        <Prompt
          when
          message={nextLocation => {
            if (views[currentStep].isFinishedStep) {
              // Allow navigating away. The finished step doesn't
              // require any more input from user and is a informational view.
              return true;
            }
            if (
              nextLocation.pathname === cfg.routes.accessListNew ||
              // Don't show prompt since going to okta will allow user to
              // resume this flow at the same spot and state they left it at.
              nextLocation.pathname.startsWith(
                cfg.oss.getIntegrationEnrollRoute('okta')
              )
            ) {
              return true;
            }
            return cancelPrompt;
          }}
        />
      )}
    </Container>
  );
}

const Container = styled(Flex)`
  flex: 1;
  width: 100%;
  overflow: auto;
`;

const MainViewContainer = styled(Flex)`
  flex: 1;
  min-width: 600px;
  flex-direction: column;
  overflow: auto;
`;
