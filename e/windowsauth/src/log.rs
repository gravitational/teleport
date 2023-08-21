use std::fs::{File, OpenOptions};
use std::io;
use std::io::Write;
use std::path::Path;
use std::sync::{Mutex, OnceLock};
use std::time::SystemTime;

use log::{Level, LevelFilter, Log, Metadata, Record};

struct Logger(OnceLock<Mutex<File>>);

static LOGGER: Logger = Logger(OnceLock::new());

pub fn initialize(file: &str) -> io::Result<()> {
    let file = OpenOptions::new()
        .append(true)
        .create(true)
        .open(Path::new(file))?;
    LOGGER.0.get_or_init(|| Mutex::new(file));

    // set_logger will fail only if the logger was set before, which is fine
    let _ = log::set_logger(&LOGGER);
    log::set_max_level(LevelFilter::Trace);
    Ok(())
}

impl Logger {
    /// This method will check existence of marker file
    fn debug_enabled(&self) -> bool {
        Path::new("C:\\Windows\\Logs\\teleport.debug").exists()
    }
}

impl Log for Logger {
    fn enabled(&self, metadata: &Metadata) -> bool {
        metadata.level() <= Level::Info || self.debug_enabled()
    }

    fn log(&self, record: &Record) {
        if !self.enabled(record.metadata()) {
            return;
        }
        if let Some(file) = self.0.get() {
            let now = humantime::format_rfc3339_seconds(SystemTime::now());
            let level = record.level();
            let args = record.args();
            let mut file = file.lock().unwrap();
            if let Some((file_name, line)) = self
                .debug_enabled()
                .then_some(record.file())
                .flatten()
                .zip(record.line())
            {
                let _ = writeln!(
                    file,
                    "[{}] {:5} {} ({}:{})",
                    now, level, args, file_name, line
                );
            } else {
                let _ = writeln!(file, "[{}] {:5} {}", now, level, args);
            }
            //try to flush file, we can't do much more with the error here
            _ = file.flush();
        }
    }

    fn flush(&self) {}
}
