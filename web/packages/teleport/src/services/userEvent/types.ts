export enum CaptureEvent {
  // UserEvent types
  BannerClickEvent = 'tp.ui.banner.click',
  OnboardAddFirstResourceClickEvent = 'tp.ui.onboard.addFirstResource.click',
  OnboardAddFirstResourceLaterClickEvent = 'tp.ui.onboard.addFirstResourceLater.click',

  // PreUserEvent types
  //   these events are unauthenticated,
  //   and require username in the request

  PreUserOnboardGetStartedClickEvent = 'tp.ui.onboard.getStarted.click', // not yet implemented
  PreUserOnboardSetCredentialSubmitEvent = 'tp.ui.onboard.setCredential.submit',
  PreUserOnboardRegisterChallengeSubmitEvent = 'tp.ui.onboard.registerChallenge.submit',

  PreUserRecoveryCodesContinueClickEvent = 'tp.ui.recoveryCodesContinue.click',
}
