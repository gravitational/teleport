import { resolve } from 'path';

import { createViteConfig } from '../../../web/packages/build/vite/config';

const rootDirectory = resolve(__dirname, '../../..');
const outputDirectory = resolve(rootDirectory, 'webassets/e/teleport');

const config = createViteConfig(rootDirectory, outputDirectory);

export { config as default };
