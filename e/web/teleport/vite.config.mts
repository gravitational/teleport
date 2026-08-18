import { resolve } from 'path';

import { createViteConfig } from '../../../web/packages/build/vite/config.mts';

const rootDirectory = resolve(import.meta.dirname, '../../..');
const outputDirectory = resolve(rootDirectory, 'webassets/e/teleport');

const config = createViteConfig(rootDirectory, outputDirectory);

export { config as default };
