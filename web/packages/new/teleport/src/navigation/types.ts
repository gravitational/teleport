import type { SidenavCategory } from './categories';

export interface RecentHistoryItem {
  category?: SidenavCategory;
  title: string;
  route: string;
  exact?: boolean;
}
