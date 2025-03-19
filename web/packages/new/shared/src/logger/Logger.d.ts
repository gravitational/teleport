export declare class Logger {
    private name;
    constructor(name?: string);
    log(level?: 'log' | 'trace' | 'warn' | 'info' | 'debug' | 'error', ...args: any[]): void;
    trace(...args: any[]): void;
    warn(...args: any[]): void;
    info(...args: any[]): void;
    debug(...args: any[]): void;
    error(...args: any[]): void;
}
//# sourceMappingURL=Logger.d.ts.map