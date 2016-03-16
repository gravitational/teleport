import { Reactor } from 'nuclear-js'

let enabled = true;

// temporary workaround to disable debug info during unit-tests
let karma = window.__karma__;
if(karma && karma.config.args.length === 1){
  enabled = false;
}

const reactor = new Reactor({
  debug: enabled
})

window.reactor = reactor;

export default reactor
