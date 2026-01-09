import { Box, Indicator } from 'design';
import { UserList } from 'design/Icon';

import {
  AccessListManagementContextProvider,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { FeatureBox } from 'teleport/components/Layout';
import { BaseView } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';

import { GuideContainer } from '../GuideEditor/Shared';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { customViews } from './Guides/Custom/Custom';
import { presetGuideViews } from './Guides/Preset/Presets';
import { SelectGuide } from './SelectGuide/SelectGuide';
import { NavView } from './types';

export const CreateAccessListWithProvider = () => (
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
      {views[currentStep].Component}
    </GuideContainer>
  );
}
