import { Platform } from 'teleterm/mainProcess/types';
import {
  KeyboardShortcutsConfig,
  KeyboardShortcutType,
  ConfigService,
} from 'teleterm/services/config';

import {
  KeyboardShortcutEvent,
  KeyboardShortcutEventSubscriber,
} from './types';

export class KeyboardShortcutsService {
  private eventsSubscribers = new Set<KeyboardShortcutEventSubscriber>();
  private keysToShortcuts = new Map<string, KeyboardShortcutType>();
  private readonly shortcutsConfig: Record<KeyboardShortcutType, string>;

  constructor(
    private platform: Platform,
    private configService: ConfigService
  ) {
    this.shortcutsConfig = {
      'tab-1': this.configService.get('keymap.tab1').value,
      'tab-2': this.configService.get('keymap.tab2').value,
      'tab-3': this.configService.get('keymap.tab3').value,
      'tab-4': this.configService.get('keymap.tab4').value,
      'tab-5': this.configService.get('keymap.tab5').value,
      'tab-6': this.configService.get('keymap.tab6').value,
      'tab-7': this.configService.get('keymap.tab7').value,
      'tab-8': this.configService.get('keymap.tab8').value,
      'tab-9': this.configService.get('keymap.tab9').value,
      'tab-close': this.configService.get('keymap.tabClose').value,
      'tab-previous': this.configService.get('keymap.tabPrevious').value,
      'tab-next': this.configService.get('keymap.tabNext').value,
      'tab-new': this.configService.get('keymap.tabNew').value,
      'open-quick-input': this.configService.get('keymap.openQuickInput').value,
      'toggle-connections': this.configService.get('keymap.toggleConnections')
        .value,
      'toggle-clusters': this.configService.get('keymap.toggleClusters').value,
      'toggle-identity': this.configService.get('keymap.toggleIdentity').value,
    };
    this.recalculateKeysToShortcuts(this.shortcutsConfig);
    this.attachKeydownHandler();
  }

  subscribeToEvents(subscriber: KeyboardShortcutEventSubscriber): void {
    this.eventsSubscribers.add(subscriber);
  }

  unsubscribeFromEvents(subscriber: KeyboardShortcutEventSubscriber): void {
    this.eventsSubscribers.delete(subscriber);
  }

  getShortcutsConfig() {
    return this.shortcutsConfig;
  }

  private attachKeydownHandler(): void {
    const handleKeydown = (event: KeyboardEvent): void => {
      const shortcutType = this.getShortcut(event);
      if (!shortcutType) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();
      this.notifyEventsSubscribers({ type: shortcutType });
    };

    window.addEventListener('keydown', handleKeydown, {
      capture: true,
    });
  }

  private getShortcut(event: KeyboardEvent): KeyboardShortcutType | undefined {
    const getEventKey = () =>
      event.key.length === 1 ? event.key.toUpperCase() : event.key;

    const key = [...this.getPlatformModifierKeys(event), getEventKey()]
      .filter(Boolean)
      .join('-');

    return this.keysToShortcuts.get(key);
  }

  private getPlatformModifierKeys(event: KeyboardEvent): string[] {
    switch (this.platform) {
      case 'darwin':
        return [
          event.metaKey && 'Command',
          event.ctrlKey && 'Control',
          event.altKey && 'Option',
          event.shiftKey && 'Shift',
        ];
      default:
        return [
          event.ctrlKey && 'Ctrl',
          event.altKey && 'Alt',
          event.shiftKey && 'Shift',
        ];
    }
  }

  /**
   * Inverts shortcuts-keys pairs to allow accessing shortcut by a key
   */
  private recalculateKeysToShortcuts(
    toInvert: Partial<KeyboardShortcutsConfig>
  ): void {
    this.keysToShortcuts.clear();
    Object.entries(toInvert).forEach(([shortcutType, key]) => {
      this.keysToShortcuts.set(key, shortcutType as KeyboardShortcutType);
    });
  }

  private notifyEventsSubscribers(event: KeyboardShortcutEvent): void {
    this.eventsSubscribers.forEach(subscriber => subscriber(event));
  }
}
