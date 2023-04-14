import React from 'react';
import { MemoryRouter } from 'react-router';

import { IntegrationEnroll } from './IntegrationEnroll';

export default {
  title: 'TeleportE/IntegrationEnroll',
};

export const Processing = () => (
  <MemoryRouter>
    <IntegrationEnroll {...sample} attempt={{ status: 'processing' as any }} />
  </MemoryRouter>
);

export const NoPluginsEnrolled = () => (
  <MemoryRouter>
    <IntegrationEnroll {...sample} />
  </MemoryRouter>
);

export const SlackAlreadyEnrolled = () => (
  <MemoryRouter>
    <IntegrationEnroll {...sample} existingTypes={['slack']} />
  </MemoryRouter>
);

export const NoAccessToPlugins = () => (
  <MemoryRouter>
    <IntegrationEnroll
      {...sample}
      existingTypes={['slack']}
      hasPluginAccess={false}
    />
  </MemoryRouter>
);

export const NoAccessToIntegrations = () => (
  <MemoryRouter>
    <IntegrationEnroll
      {...sample}
      existingTypes={['slack']}
      hasIntegrationAccess={false}
    />
  </MemoryRouter>
);

export const SlackChosen = function () {
  return (
    <MemoryRouter>
      <IntegrationEnroll {...sample} selectedType="slack" />
    </MemoryRouter>
  );
};

export const SlackEnrollSuccess = () => {
  const enrollResponse = {
    ...sample.enrollResponse,
    success: JSON.stringify({
      slack: {
        fallback_channel: '#access-requests',
      },
    }),
  };
  return (
    <MemoryRouter>
      <IntegrationEnroll
        {...sample}
        selectedType="slack"
        enrollResponse={enrollResponse}
      />
    </MemoryRouter>
  );
};

export const SlackEnrollSuccessMalformed = () => {
  const enrollResponse = {
    ...sample.enrollResponse,
    success: 'foo', // not valid JSON
  };
  return (
    <MemoryRouter>
      <IntegrationEnroll
        {...sample}
        selectedType="slack"
        enrollResponse={enrollResponse}
      />
    </MemoryRouter>
  );
};

export const SlackEnrollSuccessInternallyMalformed = () => {
  const enrollResponse = {
    ...sample.enrollResponse,
    success: JSON.stringify({}), // missing required properties
  };
  return (
    <MemoryRouter>
      <IntegrationEnroll
        {...sample}
        selectedType="slack"
        enrollResponse={enrollResponse}
      />
    </MemoryRouter>
  );
};

export const SlackEnrollError = () => {
  const enrollResponse = {
    ...sample.enrollResponse,
    error: 'access_denied',
    errorDescription: 'This is an extended error description from the provider',
  };
  return (
    <MemoryRouter>
      <IntegrationEnroll
        {...sample}
        selectedType="slack"
        enrollResponse={enrollResponse}
      />
    </MemoryRouter>
  );
};

export const Failed = () => (
  <MemoryRouter>
    <IntegrationEnroll
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const sample = {
  attempt: {
    status: 'success' as const,
  },
  availableTypes: ['slack'],
  existingTypes: [],
  run: () => Promise.resolve(true),
  selectedType: null,
  enrollResponse: {
    success: null,
    error: null,
    errorDescription: null,
    clearError: () => {},
  },
  hasPluginAccess: true,
  hasIntegrationAccess: true,
};
