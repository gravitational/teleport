interface Env {
  prehogAddress: string;
  feedbackAddress: string;
}

let ENV: Env;

// process.env.NODE_ENV is resolved by webpack at compile time
if (process.env.NODE_ENV === 'development') {
  ENV = {
    prehogAddress: 'https://reporting-staging.teleportinfra.dev',
    feedbackAddress:
      'https://kcwm2is93l.execute-api.us-west-2.amazonaws.com/prod',
  };
}

if (process.env.NODE_ENV === 'production') {
  ENV = {
    prehogAddress: '',
    feedbackAddress: 'https://usage.teleport.dev',
  };
}

export { ENV };
