/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import '@xterm/xterm/css/xterm.css';

import { FitAddon } from '@xterm/addon-fit';
import { IDisposable, ITheme, Terminal } from '@xterm/xterm';

import {
  SearchAddon,
  TerminalSearcher,
} from 'shared/components/TerminalSearch';
import { debounce } from 'shared/utils/highbar';

import Logger from 'teleterm/logger';
import { AppConfig, ConfigService } from 'teleterm/services/config';
import { WindowsPty } from 'teleterm/services/pty';
import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';

const WINDOW_RESIZE_DEBOUNCE_DELAY = 200;

type Options = {
  el: HTMLElement;
  fontSize: number;
  theme: ITheme;
  windowsPty: WindowsPty;
  openContextMenu(e: MouseEvent): void;
};

export default class TtyTerminal implements TerminalSearcher {
  public term: Terminal;
  private el: HTMLElement;
  private fitAddon = new FitAddon();
  private searchAddon = new SearchAddon();
  private resizeHandler: IDisposable;
  private debouncedResize: () => void;
  private logger = new Logger('lib/term/terminal');
  private removePtyProcessOnDataListener: () => void;
  private config: Pick<
    AppConfig,
    'terminal.rightClick' | 'terminal.copyOnSelect'
  >;
  private customKeyEventHandlers = new Set<(event: KeyboardEvent) => boolean>();

  constructor(
    private ptyProcess: IPtyProcess,
    private options: Options,
    configService: ConfigService,
    private keyboardShortcutsService: KeyboardShortcutsService
  ) {
    this.el = options.el;
    this.term = null;
    this.config = {
      'terminal.rightClick': configService.get('terminal.rightClick').value,
      'terminal.copyOnSelect': configService.get('terminal.copyOnSelect').value,
    };

    this.debouncedResize = debounce(
      this.requestResize.bind(this),
      WINDOW_RESIZE_DEBOUNCE_DELAY
    );
  }

  registerCustomKeyEventHandler(customHandler: (e: KeyboardEvent) => boolean) {
    this.customKeyEventHandlers.add(customHandler);
    return {
      unregister: () => this.customKeyEventHandlers.delete(customHandler),
    };
  }

  open(): void {
    this.term = new Terminal({
      cursorBlink: false,
      /**
       * `fontFamily` can be provided by the user and is unsanitized. This means that it cannot be directly used in CSS,
       * as it may inject malicious CSS code.
       * To sanitize the value, we set it as a style on the HTML element and then read it from it.
       * Read more https://frontarm.com/james-k-nelson/how-can-i-use-css-in-js-securely/.
       */
      fontFamily: this.el.style.fontFamily,
      fontSize: this.options.fontSize,
      scrollback: 5000,
      minimumContrastRatio: 4.5, // minimum for WCAG AA compliance
      rightClickSelectsWord: this.config['terminal.rightClick'] === 'menu',
      theme: this.options.theme,
      windowsPty: this.options.windowsPty && {
        backend: this.options.windowsPty.useConpty ? 'conpty' : 'winpty',
        buildNumber: this.options.windowsPty.buildNumber,
      },
      windowOptions: {
        setWinSizeChars: true,
      },
      allowProposedApi: true, // required for customizing SearchAddon properties
    });

    this.term.onSelectionChange(() => {
      if (this.config['terminal.copyOnSelect'] && this.term.hasSelection()) {
        void this.copySelection();
      }
    });

    this.term.loadAddon(this.fitAddon);
    this.term.loadAddon(this.searchAddon);

    this.registerResizeHandler();

    this.term.open(this.el);

    this.registerCustomKeyEventHandler(e => {
      const action = this.keyboardShortcutsService.getShortcutAction(e);
      const isKeyDown = e.type === 'keydown';
      if (action === 'terminalCopy' && isKeyDown && this.term.hasSelection()) {
        void this.copySelection();
        // Do not invoke a copy action from the menu.
        e.preventDefault();
        // Event handled, do not process it in xterm.
        return false;
      }
      if (action === 'terminalPaste' && isKeyDown) {
        void this.paste();
        // Do not invoke a copy action from the menu.
        e.preventDefault();
        // Event handled, do not process it in xterm.
        return false;
      }

      return true;
    });

    this.term.attachCustomKeyEventHandler(e => {
      for (const eventHandler of this.customKeyEventHandlers) {
        if (!eventHandler(e)) {
          // The event was handled, we can return early.
          return false;
        }
      }
      // The event wasn't handled, pass it to xterm.
      return true;
    });

    this.term.element.addEventListener('contextmenu', e => {
      // We always call preventDefault because:
      // 1. When `terminalRightClick` is not `menu`, we don't want to show it.
      // 2. When `terminalRightClick` is `menu`, opening two menus at
      //  the same time on Linux causes flickering.
      e.preventDefault();

      if (this.config['terminal.rightClick'] === 'menu') {
        this.options.openContextMenu(e);
      }
    });

    this.term.element.addEventListener('mousedown', e => {
      // Secondary button, usually the right button.
      if (e.button !== 2) {
        return;
      }

      e.stopImmediatePropagation();
      e.stopPropagation();
      e.preventDefault();

      const terminalRightClick = this.config['terminal.rightClick'];

      switch (terminalRightClick) {
        case 'paste': {
          void this.paste();
          break;
        }
        case 'copyPaste': {
          if (this.term.hasSelection()) {
            void this.copySelection();
            this.term.clearSelection();
          } else {
            void this.paste();
          }
          break;
        }
      }
    });

    this.fitAddon.fit();

    this.term.onData(data => {
      this.ptyProcess.write(data);
    });

    this.term.onResize(size => {
      this.ptyProcess.resize(size.cols, size.rows);
    });

    this.removePtyProcessOnDataListener = this.ptyProcess.onData(data =>
      this.handleData(data)
    );

    // TODO(ravicious): Don't call start if the process was already started.
    // This is what is causing the terminal to visually repeat the input on hot reload.
    // The shared process version of PtyProcess knows whether it was started or not (the status
    // field), so it's a matter of exposing this field through gRPC and reading it here.
    this.ptyProcess.start(this.term.cols, this.term.rows);

    window.addEventListener('resize', this.debouncedResize);
  }

  focus() {
    this.term.focus();
  }

  getSearchAddon() {
    return this.searchAddon;
  }

  requestResize(): void {
    const visible = !!this.el.clientWidth && !!this.el.clientHeight;
    if (!visible) {
      this.logger.info(`unable to resize terminal (container might be hidden)`);
      return;
    }
    this.fitAddon.fit();
  }

  destroy(): void {
    this.removePtyProcessOnDataListener?.();
    this.term?.dispose();
    this.fitAddon.dispose();
    this.resizeHandler?.dispose();
    this.el.innerHTML = null;

    window.removeEventListener('resize', this.debouncedResize);
  }

  private async copySelection(): Promise<void> {
    const selection = this.term.getSelection();
    await navigator.clipboard.writeText(selection);
  }

  private async paste(): Promise<void> {
    const text = await navigator.clipboard.readText();
    this.term.paste(text);
  }

  private registerResizeHandler(): void {
    let prevCols: number, prevRows: number;
    this.resizeHandler = this.term.parser.registerCsiHandler(
      { final: 't' },
      params => {
        const [ps] = params;
        if (ps === 8) {
          // Ps = 8 - resizes the text area to given height and width in characters.
          const rows = params[1] as number;
          const cols = params[2] as number;
          if (prevRows !== rows || prevCols !== cols) {
            prevRows = rows;
            prevCols = cols;
            this.term.resize(cols, rows);
          }
          return true; // sequence has been handled
        }
        return false;
      }
    );
  }

  private handleData(data: string): void {
    try {
      this.term.write(data);
    } catch (err) {
      this.logger.error('xterm.write', data, err);
      // recover xtermjs by resetting it
      this.term.reset();
    }
  }
}
