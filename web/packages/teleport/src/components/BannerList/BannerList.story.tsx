import React from 'react';

import { BannerList } from './BannerList';

export default {
  title: 'Teleport/BannerList',
};

export function List() {
  return (
    <BannerList
      banners={[{ id: 'ban1', severity: 'info', message: 'This is fine.' }]}
    />
  );
}
