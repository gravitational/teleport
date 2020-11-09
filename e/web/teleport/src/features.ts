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

import * as Features from 'teleport/features';

export default function getFeatures() {
  return [
    new Features.FeatureNodes(),
    new Features.FeatureApps(),
    new Features.FeatureSessions(),
    new Features.FeatureRecordings(),
    new Features.FeatureAudit(),
    new Features.FeatureUsers(),
    new Features.FeatureRoles(),
    new Features.FeatureAuthConnectors(),
    new Features.FeatureClusters(),
    new Features.FeatureTrust(),
    new Features.FeatureHelpAndSupport(),
    new Features.FeatureAccount(),
  ];
}
