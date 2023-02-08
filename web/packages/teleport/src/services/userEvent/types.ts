/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export enum CaptureEvent {
  // UserEvent types
  BannerClickEvent = 'tp.ui.banner.click',
  OnboardAddFirstResourceClickEvent = 'tp.ui.onboard.addFirstResource.click',
  OnboardAddFirstResourceLaterClickEvent = 'tp.ui.onboard.addFirstResourceLater.click',

  // PreUserEvent types
  //   these events are unauthenticated,
  //   and require username in the request

  PreUserOnboardSetCredentialSubmitEvent = 'tp.ui.onboard.setCredential.submit',
  PreUserOnboardRegisterChallengeSubmitEvent = 'tp.ui.onboard.registerChallenge.submit',
  PreUserCompleteGoToDashboardClickEvent = 'tp.ui.onboard.completeGoToDashboard.click',

  PreUserRecoveryCodesContinueClickEvent = 'tp.ui.recoveryCodesContinue.click',
  PreUserRecoveryCodesCopyClickEvent = 'tp.ui.recoveryCodesCopy.click',
  PreUserRecoveryCodesPrintClickEvent = 'tp.ui.recoveryCodesPrint.click',
}
