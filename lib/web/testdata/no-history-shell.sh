#!/bin/bash
# Shell wrapper for lib/web tests. Disables history so that tests don't pollute the user's
# shell history, and skips profile so they don't depend on the user's shell config.
exec bash --noprofile --rcfile <(echo 'unset HISTFILE')
