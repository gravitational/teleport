/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import json from './app.json';
import clusterJson from './cluster.json';
import flavors from './flavors.json';
import operations from './operations.json';
import { eula } from './eula';
import { merge } from 'lodash';

const appWithLicense = {};
const appWithEula = {};
const app = {};
const cluster = {};

merge(app, json, {
  manifest: {
    webConfig: JSON.stringify({
      agentReport: {
        provision: {
          interfaces: {
            ipv4: {
              labelText: '!IP Address',
              toolipText: '!IP address used to communicate within the cluster'
            }
          },

          mounts: {
            tmp: {
              labelText: '!input field label',
              toolipText: '!input field tooltip'
            }
          }
        }
      },
      modules: {
        installer: {
          eulaAgreeText: "!!I Agree To The Terms",
          eulaHeaderText: "!!Welcome to the {0} Installer",
          eulaContentLabelText: "!!License Agreement",
          licenseHeaderText: "!!Enter your license",
          licenseOptionTrialText: "!!Trial without license",
          licenseOptionText: "!!With a license",
          licenseUserHintText: "!!If you have a license, please insert it here. In the next steps you will select the location of your application and the capacity you need",
          progressUserHintText: "!!Your infrastructure is being provisioned and your application is being installed.\n\n Once the installation is complete you will be taken to your infrastructure where you can access your application.",
          prereqUserHintText: `!!The cluster name will be used for issuing SSH and HTTP/TLS certificates to securely access the cluster.\n\n For this reason it is recommended to use a fully qualified domain name (FQDN) for the cluster name, e.g. prod.example.com`,
          provisionUserHintText: "!!Drag the slider to estimate the number of resources needed for that performance level. You can also add / remove resources after the installation. \n\n Once you click Start Installation the resources will be provisioned on your infrastructure.",
          iamPermissionsHelpLink: "!!https://gravitational.com/gravity/docs/overview/"
        }
      }
    }),
  }
});

merge(cluster, clusterJson);

merge(appWithLicense, json, {
  manifest: {
    license: {
      enabled: true
    },
  }
})

merge(appWithEula, json, {
  manifest: {
    installer: {
      eula: {
        source: eula
      }
    },
  }
})

export {
  app,
  appWithLicense,
  appWithEula,
  cluster,
  flavors,
  operations
}
