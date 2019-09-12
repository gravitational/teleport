import React from 'react';
import { FluxContext } from 'gravity/components/nuclear';
import { withState } from 'shared/hooks';
import { getters as navGetters } from 'e-gravity/hub/flux/nav';
import SideNavLayout from '../components/SideNavLayout';

function mapState({ feature }) {
  const reactor = React.useContext(FluxContext);
  const navStore = reactor.evaluate(navGetters.navStore);
  return {
    navItems: navStore.userRole,
    feature
  }
}

export default withState(mapState)(SideNavLayout);