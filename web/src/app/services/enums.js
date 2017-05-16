export default {
  AuthTypeEnum: {
    LOCAL: 'local',
    OIDC: 'oidc',
    SAML: 'saml',
    LDAP: 'ldap'
  },

  Auth2faTypeEnum: {
    UTF: 'u2f',
    OTP: 'otp',
    DISABLED: 'off'
  },
  
  AuthProviderEnum: {
    GOOGLE: 'google',
    MS: 'microsoft',
    GITHUB: 'github',
    BITBUCKET: 'bitbucket'
  }
}
