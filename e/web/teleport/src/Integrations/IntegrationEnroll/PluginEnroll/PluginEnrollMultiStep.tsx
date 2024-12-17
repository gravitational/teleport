import { Box } from 'design';

import { Navigation } from 'teleport/components/Wizard/Navigation';
import { findViewAtIndex } from 'teleport/components/Wizard/flow';

import { PluginIcon } from '../IntegrationPick/PluginIcon';

import { usePlugin } from './MultiStep/usePlugin';

export function PluginEnrollMultiStep() {
  const { currentStep, indexedViews, selectedPlugin } = usePlugin();

  const CurrentViewComponent = findViewAtIndex(
    indexedViews,
    currentStep
  ).component;

  return (
    <>
      <Box mt={4} mb={4}>
        <Navigation
          currentStep={currentStep}
          views={indexedViews}
          startWithIcon={{
            title: selectedPlugin.fullName,
            component: <PluginIcon type={selectedPlugin.type} size={16} />,
          }}
        />
      </Box>
      <CurrentViewComponent />
      {/* TODO(lisa): add a alert (react prompt like in discover) before
          letting user change routes (that they'll lose all prior work)
       */}
    </>
  );
}
