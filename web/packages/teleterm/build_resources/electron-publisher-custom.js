/**
 * A publisher class is required by `electron-updater` even when using a custom provider.
 * Although we don't use `electron-updater` to publish updates,
 * this config allows generating `app-update.yml` file at build time (kept in
 * app resources), which is required at runtime by `electron-updater` to work.
 */
export default class Noop {}
