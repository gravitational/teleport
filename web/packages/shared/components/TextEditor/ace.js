import ace from 'ace-builds/src-min-noconflict/ace';

// Ace extension and mode files (e.g. mode-json.js, ext-searchbox.js) reference
// `ace` as a bare global identifier (`ace.define(...)`). The ace.js IIFE tries
// to set `window.ace` via `(function(){ return this })()`, but Vite 8's bundler
// (Rolldown) can drop or scope that assignment. Explicitly assign it here so
// that subsequently-evaluated extension modules can resolve the bare `ace` ref.
// eslint-disable-next-line no-undef
globalThis.ace = ace;

export default ace;
