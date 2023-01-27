/**
 * `StaticConfig` allows providing different values between the dev build and
 * the packaged app.
 * The proper config is resolved by webpack at compile time.
 * This differs from `RuntimeSettings`, where properties are resolved during
 * the app's runtime.
 */

interface IStaticConfig {
  prehogAddress: string;
  feedbackAddress: string;
}

let staticConfig: IStaticConfig;

if (process.env.NODE_ENV === 'development') {
  staticConfig = {
    prehogAddress: 'https://reporting-staging.teleportinfra.dev',
    feedbackAddress:
      'https://kcwm2is93l.execute-api.us-west-2.amazonaws.com/prod',
  };
}

if (process.env.NODE_ENV === 'production') {
  staticConfig = {
    prehogAddress: '',
    feedbackAddress: 'https://usage.teleport.dev',
  };
}

export { staticConfig };
