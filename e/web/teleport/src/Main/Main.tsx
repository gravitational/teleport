import React, { useMemo } from 'react';
import useMain from 'teleport/Main/useMain';
import { Main } from 'teleport/Main/Main';
import getFeatures from '../features';

export default function Container() {
  const features = useMemo(() => getFeatures(), []);
  const state = useMain(features);

  return <Main {...state} />;
}
