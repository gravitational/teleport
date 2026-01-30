import { Prompt } from 'react-router-dom';

import { Box, Indicator } from 'design';
import { UserList } from 'design/Icon';

import {
  AccessListManagementContextProvider,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { FeatureBox } from 'teleport/components/Layout';
import { BaseView } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';

import { stepButtonContainerHeight } from '../GuideEditor/const';
import { GuideContainer } from '../GuideEditor/Shared';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { customViews } from './Guides/Custom/Custom';
import { presetGuideViews } from './Guides/Preset/Presets';
import { SelectGuide } from './SelectGuide/SelectGuide';
import { NavView } from './types';

export const CreateAccessListWithProviderV2 = () => (
  <AccessListManagementContextProvider>
    <CreateAccessListContextProvider>
      <CreateAccessList />
    </CreateAccessListContextProvider>
  </AccessListManagementContextProvider>
);

export function CreateAccessList() {
  const { guideEditor, oktaPluginAttempt } = useAccessListManagementContext();
  const { preset, currentStep } = guideEditor;

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

  let views: BaseView<NavView>[] = [];
  let navTitle = '';
  switch (preset) {
    case 'long-term':
    case 'short-term':
      navTitle = `Access List (${preset === 'long-term' ? 'standing access' : 'JIT access'})`;
      views = presetGuideViews;
      break;
    case '':
      navTitle = 'Access List (custom)';
      views = customViews;
      break;
    default:
      preset satisfies never;
  }

  return (
    <GuideContainer>
      <Box mb={4} mt={3}>
        <Navigation
          currentStep={currentStep}
          views={views}
          startWithIcon={{
            title: navTitle,
            component: <UserList size={20} />,
          }}
        />
      </Box>
      <Box
        css={`
          margin-bottom: ${stepButtonContainerHeight};
        `}
      >
        {views[currentStep].Component}
      </Box>
      {preset && currentStep < presetGuideViews.length - 1 && (
        <Prompt
          message={nextLocation => {
            if (nextLocation.pathname === cfg.routes.accessListNew) return true;
            return 'Are you sure you want to exit the "Create New Access List" workflow? You’ll have to start from the beginning next time.';
          }}
        />
      )}
    </GuideContainer>
  );
}
