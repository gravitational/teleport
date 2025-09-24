/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';
import { CollapsibleInfoSection as CollapsibleInfoSectionComponent } from 'design/CollapsibleInfoSection';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { IamIntegration } from 'teleport/Integrations/Enroll/AwsConsole/IamIntegration/IamIntegration';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

export default {
  title: 'Teleport/Integrations/Enroll/AwsConsole/IamIntegration',
};

const ctx = createTeleportContext();

export const Loaded = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter>
        <CollapsibleInfoSectionComponent openLabel="Devs Instructions">
          <Info
            kind="success"
            details="(Devs) step 1: use test as the integration name"
          >
            Case: Profiles
          </Info>
          <Info
            kind="warning"
            details="(Devs) step 1: use zero as the integration name"
          >
            Case: No Profiles
          </Info>
          <Info
            kind="danger"
            details="(Devs) step 1: use error as the integration name"
          >
            Case: Test error
          </Info>
          <Info
            kind="info"
            details={`(Devs) use any valid CloudShell output and click submit to see a success message, for instance:
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/bar
arn:aws:iam::123456789012:role/baz`}
          >
            Step 3
          </Info>
        </CollapsibleInfoSectionComponent>
        <IamIntegration />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </ContextProvider>
);
Loaded.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getValidateAWSRolesAnywhereIntegrationUrl('test'), () => {
        return HttpResponse.json({});
      }),
      http.post(cfg.getValidateAWSRolesAnywhereIntegrationUrl('zero'), () => {
        return HttpResponse.json({});
      }),
      http.post(cfg.getValidateAWSRolesAnywhereIntegrationUrl('error'), () => {
        return HttpResponse.json({});
      }),
      http.post(
        cfg.getValidateAWSRolesAnywhereIntegrationUrl('duplicate'),
        () => {
          return HttpResponse.json(
            {
              message: 'Integration name already exists',
            },
            { status: 400 }
          );
        }
      ),
      http.post(cfg.getAwsRolesAnywherePingUrl('test'), () => {
        return HttpResponse.json({
          profileCount: 3,
          accountID: 'fc2ef183-2ac0-4836-9d7d-ff873c99e733',
          arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
          userId: 'edd13a04-9956-4ef2-9ef5-7b0169e1cd5b',
        });
      }),
      http.post(cfg.getAwsRolesAnywherePingUrl('zero'), () => {
        return HttpResponse.json({
          profileCount: 0,
          accountID: 'fc2ef183-2ac0-4836-9d7d-ff873c99e733',
          arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
          userId: 'edd13a04-9956-4ef2-9ef5-7b0169e1cd5b',
        });
      }),
      http.post(cfg.getAwsRolesAnywherePingUrl('error'), () => {
        return HttpResponse.json(
          {
            message: 'some error message',
          },
          { status: 500 }
        );
      }),
      http.post(cfg.getIntegrationsUrl(), () => {
        return HttpResponse.json({ status: 200 });
      }),
    ],
  },
};

export const WithoutAccess = () => {
  const noCtx = createTeleportContext({
    customAcl: makeAcl({
      integrations: {
        ...defaultAccess,
      },
    }),
  });

  return (
    <ContextProvider ctx={noCtx}>
      <InfoGuidePanelProvider>
        <MemoryRouter>
          <IamIntegration />
        </MemoryRouter>
      </InfoGuidePanelProvider>
    </ContextProvider>
  );
};
