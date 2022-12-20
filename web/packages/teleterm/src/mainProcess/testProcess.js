const process = require('process');

const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

(async () => {
  const waitTime = parseInt(process.argv[2]);
  if (waitTime) {
    await sleep(waitTime);
  }

  const shouldExit = process.argv[3];
  if (shouldExit) {
    process.exit(1);
  }

  console.log('Lorem ipsum dolor sit amet');
  console.log('{CONNECT_GRPC_PORT: 1337}');
  console.log('Lorem ipsum dolor sit amet');
})();
