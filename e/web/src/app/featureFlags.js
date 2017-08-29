import reactor from 'app/reactor';
import cfg from 'app/config';
import userAclGetters from 'app/flux/userAcl/getters';
import userGetters from 'app/flux/user/getters';

const getAcl = () => reactor.evaluate(userAclGetters.userAcl);

export function siteMonitoring() {  
  return getAcl().isK8sEnabled() && cfg.isSiteMonitoringEnabled();
}

export function siteK8s() {  
  return getAcl().isK8sEnabled() && cfg.isSiteK8sEnabled();
}

export function siteConfigMaps() {  
  return getAcl().isK8sEnabled() && cfg.isSiteConfigMapsEnabled();
}

export function siteLogs() {  
  return cfg.isSiteLogsEnabled();
}

export function settingsAccount(){
  const userStore = reactor.evaluate(userGetters.user)
  return !userStore.isSso();
}

export function settingsCertificate() {  
  return getAcl().isAdminEnabled() && !cfg.isDevCluster();
}

export function settingsLicense(isOpsCenter) {  
  const allowed = getAcl().isLicenseGenEnabled();
  return isOpsCenter && cfg.isSettingsLicenseGenEnabled() === true && allowed;
}

export function settingsLogForwarder(isOpsCenter) {
  return !isOpsCenter && cfg.isSettingsLogsEnabled()
}

export function settingsMonitoring(isOpsCenter) {
  if (cfg.isDevCluster()) {
    return false;
  }
  
  return !isOpsCenter && cfg.isSettingsMonitoringEnabled() && getAcl().isK8sEnabled();
}

export function settingsAuth() {  
  return getAcl().isAdminEnabled();
}

export function settingsRole() {  
  return getAcl().isAdminEnabled();
}

export function settingsUsers() {  
  return getAcl().isAdminEnabled();
}