/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package auditqueue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	incrementalVacuumInterval = 10 * time.Minute
	defaultOrphanScanInterval = 10 * time.Minute
	queueLockFile             = "queue.lock"
	queueDBFile               = "queue.db"
	tmpDirSuffix              = ".tmp"
	defaultSoftLimit          = 100 * 1024 * 1024 // 100 MiB
	softLimitCheckInterval    = time.Minute

	initQueueDirMaxAttempts = 10
	initQueueDirRetryDelay  = 50 * time.Millisecond

	// walJournalSizeLimit is the number of bytes the `-wal` file gets
	// truncated to in between checkpoints.
	walJournalSizeLimit = 64 * 1024 * 1024 // 64 MiB

	// defaultMaxBytes limits the size of the SQLite database file.
	defaultMaxBytes = 5 * 1024 * 1024 * 1024 // 5 GiB

	// staleTmpThreshold is how old an in-progress <uuid>.tmp/ directory
	// must be before the orphan scanner removes it. Anything younger may
	// still belong to a peer that's mid-creation.
	staleTmpThreshold = time.Hour
)

func getDSN(dbPath string, maxBytes int64, synchronous SynchronousMode) string {
	if synchronous == "" {
		synchronous = SynchronousNormal
	}
	params := url.Values{}
	params.Add("_pragma", "auto_vacuum(INCREMENTAL)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", fmt.Sprintf("synchronous(%s)", synchronous))
	params.Add("_pragma", fmt.Sprintf("journal_size_limit(%d)", walJournalSizeLimit))
	addSharedParams(params, maxBytes)
	u := url.URL{
		Scheme:   "file",
		OmitHost: true,
		Path:     dbPath,
		RawQuery: params.Encode(),
	}
	return u.String()
}

func newSQLiteQueue(cfg Config) (*sqliteQueue, error) {
	if cfg.Path == "" {
		return nil, trace.BadParameter("Path is required to create an sqlite queue")
	}

	unlock, err := initQueueDir(cfg.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	db, err := initializeDb(cfg.Path, cfg.MaxBytes, cfg.Synchronous)
	if err != nil {
		_ = unlock()
		_ = os.RemoveAll(cfg.Path)
		return nil, trace.Wrap(err)
	}

	selfStat, err := os.Stat(cfg.Path)
	if err != nil {
		db.Close()
		_ = unlock()
		_ = os.RemoveAll(cfg.Path)
		return nil, trace.ConvertSystemError(err)
	}

	scanInterval := cfg.OrphanScanInterval
	if scanInterval <= 0 {
		scanInterval = defaultOrphanScanInterval
	}

	softLimit := cfg.SoftLimit
	if softLimit <= 0 {
		softLimit = defaultSoftLimit
	}

	q, err := newBaseQueue(db, cfg)
	if err != nil {
		db.Close()
		_ = unlock()
		_ = os.RemoveAll(cfg.Path)
		return nil, trace.Wrap(err)
	}

	q.path = cfg.Path
	q.parentDir = filepath.Dir(cfg.Path)
	q.selfStat = selfStat
	q.unlock = unlock
	q.orphanScanInterval = scanInterval
	q.softLimit = softLimit

	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	slog.InfoContext(q.ctx, "Audit queue initialized.",
		"path", cfg.Path,
		"max_bytes", maxBytes,
		"soft_limit", softLimit,
		"max_attempts", q.maxAttempts,
		"dead_letter_ttl", q.deadLetterTTL,
	)

	q.wg.Go(q.softLimitLoop)
	q.wg.Go(q.vacuumLoop)

	return q, nil
}

func initializeDb(path string, maxBytes int64, synchronous SynchronousMode) (*sql.DB, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	dbPath := filepath.Join(path, queueDBFile)
	db, err := sql.Open("sqlite", getDSN(dbPath, maxBytes, synchronous))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}
	if err := recordTeleportVersion(db, teleport.Version); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}

	return db, nil
}

// initQueueDir creates and initializes the directory where the audit log queue
// will reside. It does this in a way that is safe from race conditions. If the
// directory is transiently claimed by another instance (e.g. due to clock
// skew), it retries a bounded number of times.
func initQueueDir(path string) (func() error, error) {
	tmpPath := path + tmpDirSuffix
	var err error
	for attempt := range initQueueDirMaxAttempts {
		var unlock func() error
		unlock, err = tryInitQueueDir(path, tmpPath)
		if err == nil {
			return unlock, nil
		}

		isRetryableError := errors.Is(err, utils.ErrUnsuccessfulLockTry) || trace.IsNotFound(err)
		if !isRetryableError {
			return nil, trace.Wrap(err)
		}

		if attempt < initQueueDirMaxAttempts-1 {
			time.Sleep(initQueueDirRetryDelay)
		}
	}
	return nil, trace.Wrap(err, "failed to initialize audit-queue directory %q after %d attempts (is the system time set correctly?)", path, initQueueDirMaxAttempts)
}

func tryInitQueueDir(path, tmpPath string) (func() error, error) {
	if err := os.MkdirAll(tmpPath, 0o700); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Take the lock on the `queue.lock` file. This ensures that no other
	// instances try to adopt this queue.
	unlock, err := utils.FSTryWriteLock(filepath.Join(tmpPath, queueLockFile))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove the `.tmp` suffix, marking this as a live queue.
	if err := os.Rename(tmpPath, path); err != nil {
		// Only remove the directory if we own the flock.
		_ = os.RemoveAll(tmpPath)
		_ = unlock()
		return nil, trace.ConvertSystemError(err)
	}
	return unlock, nil
}

// vacuumLoop periodically cleans up deleted records from the SQLite database
// file.
func (q *sqliteQueue) vacuumLoop() {
	timer := time.NewTimer(incrementalVacuumInterval)
	defer timer.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-timer.C:
			if _, err := q.db.ExecContext(q.ctx, "PRAGMA incremental_vacuum"); err != nil {
				slog.ErrorContext(q.ctx, "Failed to run incremental_vacuum.", "error", err)
			}
			timer.Reset(incrementalVacuumInterval)
		}
	}
}

// softLimitLoop periodically stats queue.db and emits a warning when its
// size exceeds the configured soft limit.
func (q *sqliteQueue) softLimitLoop() {
	ticker := time.NewTicker(softLimitCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
		}
		above, size, err := q.aboveSoftLimit()
		if err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to stat audit queue file.",
				"path", q.path,
				"error", err,
			)
			continue
		}
		if above {
			slog.WarnContext(q.ctx,
				"audit event queue above soft limit",
				"path", q.path,
				"size_bytes", size,
				"soft_limit_bytes", q.softLimit,
			)
			softLimitWarnings.Inc()
		}
	}
}

func (q *sqliteQueue) aboveSoftLimit() (bool, int64, error) {
	info, err := os.Stat(filepath.Join(q.path, queueDBFile))
	if err != nil {
		return false, 0, trace.ConvertSystemError(err)
	}
	size := info.Size()
	return size > q.softLimit, size, nil
}

// Run drains the queue. `handler` is the function called for each audit log
// event that is held within the queue. The audit log queue follows a single
// consumer model in order to batch events together and commit them as groups.
func (q *sqliteQueue) Run(ctx context.Context, handler Handler) error {
	// Ensure we only have a single consumer running.
	if !q.runMu.TryLock() {
		return trace.Wrap(ErrAlreadyRunning)
	}
	defer q.runMu.Unlock()

	// Startup the orphan scanner and dead-letter sweeper.
	var wg sync.WaitGroup
	wg.Go(func() {
		q.orphanScanLoop(ctx)
	})
	wg.Go(func() {
		q.deadLetterSweepLoop(ctx, handler)
	})
	defer wg.Wait()

	return q.runPollLoop(ctx, handler)
}

func (q *sqliteQueue) orphanScanLoop(ctx context.Context) {
	ticker := time.NewTicker(q.orphanScanInterval)
	defer ticker.Stop()
	for !q.isDraining() {
		q.sweepStaleTmp()
		q.adoptOrphans(ctx)

		select {
		case <-ctx.Done():
			return
		case <-q.ctx.Done():
			return
		case <-q.drainCh:
			return
		case <-ticker.C:
		}
	}
}

func (q *sqliteQueue) isDraining() bool {
	select {
	case <-q.drainCh:
		return true
	default:
		return false
	}
}

// sweepStaleTmp cleans up any audit queue directories that have been orphaned
// during their initialization phase. This should be rare, but we still want to
// account for it.
func (q *sqliteQueue) sweepStaleTmp() {
	entries, err := os.ReadDir(q.parentDir)
	if err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to list audit-queue parent directory.",
			"parent_dir", q.parentDir,
			"error", err,
		)
		return
	}
	cutoff := time.Now().Add(-staleTmpThreshold)
	for _, dirEntry := range entries {
		if !dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), tmpDirSuffix) {
			continue
		}
		info, err := dirEntry.Info()
		if err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to stat audit-queue tmp directory.",
				"name", dirEntry.Name(),
				"error", err,
			)
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		// If we get to this point, then the directory is older than the cutoff,
		// which should not be the case for a tmp directory. These directories
		// should have their `*.tmp` suffix removed as soon as they are done
		// initializing, which should be a very quick process.
		stalePath := filepath.Join(q.parentDir, dirEntry.Name())
		tryRemoveStaleTmp(q.ctx, stalePath)
	}
}

func tryRemoveStaleTmp(ctx context.Context, stalePath string) {
	unlock, err := utils.FSTryWriteLock(filepath.Join(stalePath, queueLockFile))
	if err != nil {
		// The lock is held, so a creator is still active. Leave the directory
		// for its owner and try again on the next sweep.
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			slog.WarnContext(ctx,
				"Audit-queue tmp directory is still locked past the stale threshold. Leaving it for its owner.",
				"path", stalePath,
				"stale_threshold", staleTmpThreshold,
			)
			return
		}
		orphanScanErrors.Inc()
		slog.ErrorContext(ctx,
			"Failed to lock stale audit-queue tmp directory.",
			"path", stalePath,
			"error", err,
		)
		return
	}
	defer func() {
		if err := unlock(); err != nil {
			slog.ErrorContext(ctx,
				"Failed to release stale audit-queue tmp flock.",
				"path", stalePath,
				"error", err,
			)
		}
	}()

	// We hold the lock, so the directory is unowned. Tmp directories were
	// orphaned before they finished initializing, therefore they will have no
	// audit log events, hence they are safe to remove.
	if err := os.RemoveAll(stalePath); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(ctx,
			"Failed to remove stale audit-queue tmp directory.",
			"path", stalePath,
			"error", err,
		)
	}
}

func (q *sqliteQueue) adoptOrphans(ctx context.Context) {
	entries, err := os.ReadDir(q.parentDir)
	if err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to list audit-queue parent directory.",
			"parent_dir", q.parentDir,
			"error", err,
		)
		return
	}
	for _, dirEntry := range entries {
		if ctx.Err() != nil {
			return
		}
		if !dirEntry.IsDir() || strings.HasSuffix(dirEntry.Name(), tmpDirSuffix) {
			continue
		}

		entryPath := filepath.Join(q.parentDir, dirEntry.Name())
		entryStat, err := os.Stat(entryPath)
		if err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to stat audit-queue candidate directory.",
				"path", entryPath,
				"error", err,
			)
			continue
		}
		if os.SameFile(entryStat, q.selfStat) {
			continue
		}

		// If we got to this point, then we have found a directory that is not a
		// tmp directory and is not the same directory as the one this process
		// is already using. We are safe to attempt to adopt it.
		q.tryAdoptOrphan(ctx, entryPath)
	}
}

func (q *sqliteQueue) tryAdoptOrphan(ctx context.Context, path string) {
	unlock, err := utils.FSTryWriteLock(filepath.Join(path, queueLockFile))
	if err != nil {
		// This error indicates that the lock has already been taken, hence this
		// queue is not an orphan. We can skip it.
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			return
		}

		// Otherwise, an error occurred while attempting to take the file lock.
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to attempt orphan flock.",
			"path", path,
			"error", err,
		)
		return
	}
	defer func() {
		if err := unlock(); err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to release orphan flock.",
				"path", path,
				"error", err,
			)
		}
	}()

	dbFilePath := filepath.Join(path, queueDBFile)
	db, err := sql.Open("sqlite", getDSN(dbFilePath, defaultMaxBytes, q.synchronous))
	if err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx, "Failed to open orphan SQLite database.", "path", path, "error", err)
		return
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to connect to orphan SQLite database.",
			"path", path,
			"error", err,
		)
		_ = db.Close()
		return
	}

	migrateErr := q.migrateOrphanDB(ctx, db, filepath.Base(path))
	if err := db.Close(); err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to close orphan SQLite database.",
			"path", path,
			"error", err,
		)
		return
	}

	if migrateErr != nil {
		// If we fail to migrate, we will try again on the next orphan adoption
		// cycle. Cancellation is a normal shutdown, not a migration failure.
		if ctx.Err() == nil && q.ctx.Err() == nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to migrate orphaned audit-queue database.",
				"path", path,
				"error", migrateErr,
			)
		}
		return
	}

	// If we got here, then we have successfully migrated the orphan. We can now
	// safely remove it.
	if err := os.RemoveAll(path); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to remove migrated orphan directory.",
			"path", path,
			"error", err,
		)
		return
	}
	q.clearOrphanWatermarks(ctx, filepath.Base(path))
	orphansAdopted.Inc()
	slog.InfoContext(q.ctx, "Adopted orphaned audit-queue directory.", "path", path)
}

func (q *sqliteQueue) migrateOrphanDB(ctx context.Context, db *sql.DB, name string) error {
	if err := q.migrateOrphanQueue(ctx, db, name); err != nil {
		return trace.Wrap(err)
	}
	if err := q.migrateOrphanDeadLetter(ctx, db, name); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(q.migrateOrphanCorruptEvents(ctx, db, name))
}

func (q *sqliteQueue) migrateOrphanQueue(ctx context.Context, orphan *sql.DB, name string) error {
	return q.migrateOrphanTable(ctx, orphan, name, auditQueueTable,
		"SELECT id, payload, attempts FROM audit_queue WHERE id > ? ORDER BY id ASC LIMIT ?",
		"INSERT INTO audit_queue (payload, attempts) VALUES (?, ?)",
	)
}

func (q *sqliteQueue) migrateOrphanDeadLetter(ctx context.Context, orphan *sql.DB, name string) error {
	return q.migrateOrphanTable(ctx, orphan, name, auditDeadLetterTable,
		"SELECT id, payload, failed_at FROM audit_dead_letter WHERE id > ? ORDER BY id ASC LIMIT ?",
		"INSERT INTO audit_dead_letter (payload, failed_at) VALUES (?, ?)",
	)
}

func orphanWatermarkKey(name, table string) string {
	return "orphan_migration:" + name + ":" + table
}

func (q *sqliteQueue) migrateOrphanTable(ctx context.Context, orphan *sql.DB, name, table, selectSQL, insertSQL string) error {
	watermarkKey := orphanWatermarkKey(name, table)
	watermark, err := q.readOrphanWatermark(ctx, watermarkKey)
	if err != nil {
		return trace.Wrap(err, "reading orphan %s watermark", table)
	}
	for {
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		if err := q.ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		batch, err := fetchOrphanRows(ctx, orphan, selectSQL, watermark, dequeueBatchSize)
		if err != nil {
			return trace.Wrap(err, "fetching orphan %s rows", table)
		}
		if len(batch) == 0 {
			return nil
		}

		maxID := batch[len(batch)-1].id
		if err := q.insertMigratedBatch(ctx, insertSQL, batch, watermarkKey, maxID); err != nil {
			return trace.Wrap(err, "migrating orphan %s rows", table)
		}
		watermark = maxID

		ids := make([]int64, len(batch))
		for i, r := range batch {
			ids[i] = r.id
		}
		if err := deleteIDsFromTable(ctx, orphan, table, ids); err != nil {
			return trace.Wrap(err, "deleting migrated orphan %s rows", table)
		}
	}
}

func (q *sqliteQueue) readOrphanWatermark(ctx context.Context, key string) (int64, error) {
	var value string
	err := q.db.QueryRowContext(ctx,
		"SELECT value FROM teleport_info WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, trace.Wrap(err)
	}
	watermark, err := strconv.ParseInt(value, 10, 64)
	return watermark, trace.Wrap(err)
}

func (q *sqliteQueue) clearOrphanWatermarks(ctx context.Context, name string) {
	if _, err := q.db.ExecContext(ctx,
		"DELETE FROM teleport_info WHERE key IN (?, ?, ?)",
		orphanWatermarkKey(name, auditQueueTable),
		orphanWatermarkKey(name, auditDeadLetterTable),
		orphanWatermarkKey(name, corruptEventsTable),
	); err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to clear orphan migration watermarks.",
			"orphan", name,
			"error", err,
		)
	}
}

type migratedRow struct {
	id     int64
	values []any
}

func fetchOrphanRows(ctx context.Context, db *sql.DB, selectSQL string, afterID int64, limit int) ([]migratedRow, error) {
	rows, err := db.QueryContext(ctx, selectSQL, afterID, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []migratedRow
	for rows.Next() {
		r := migratedRow{values: make([]any, len(cols)-1)}
		targets := make([]any, 0, len(cols))
		targets = append(targets, &r.id)
		for i := range r.values {
			targets = append(targets, &r.values[i])
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, r)
	}
	return out, trace.Wrap(rows.Err())
}

func (q *sqliteQueue) insertMigratedBatch(ctx context.Context, insertSQL string, batch []migratedRow, watermarkKey string, maxID int64) error {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stmt.Close()
	for _, r := range batch {
		if _, err := stmt.ExecContext(ctx, r.values...); err != nil {
			return trace.Wrap(err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO teleport_info (key, value) VALUES (?, ?)",
		watermarkKey, strconv.FormatInt(maxID, 10),
	); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(tx.Commit())
}

func (q *sqliteQueue) migrateOrphanCorruptEvents(ctx context.Context, orphan *sql.DB, name string) error {
	return q.migrateOrphanTable(ctx, orphan, name, corruptEventsTable,
		"SELECT id, payload, error, source, failed_at FROM corrupt_events WHERE id > ? ORDER BY id ASC LIMIT ?",
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, ?, ?, ?)",
	)
}

func (q *sqliteQueue) Close() error {
	var errs []error
	q.closeOnce.Do(func() {
		q.cancel()
		q.wg.Wait()

		// Flush the WAL file.
		if _, err := q.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to checkpoint WAL on close.",
				"path", q.path,
				"error", err,
			)
		}

		empty, err := isQueueEmpty(q.db)
		if err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to check whether audit queue is empty on close.",
				"path", q.path,
				"error", err,
			)
		}

		if err := q.db.Close(); err != nil {
			errs = append(errs, err)
		}

		// Remove the directory before releasing the lock. This ensures no other
		// process can adopt and start draining a queue we are about to delete.
		if empty {
			if err := os.RemoveAll(q.path); err != nil {
				slog.ErrorContext(q.ctx, "Failed to remove empty audit-queue directory on close.", "path", q.path, "error", err)
			}
		}

		if q.unlock != nil {
			if err := q.unlock(); err != nil {
				errs = append(errs, err)
			}
		}
	})
	return trace.NewAggregate(errs...)
}

const isEmptyQuery = `SELECT EXISTS(SELECT 1 FROM audit_queue)
	OR EXISTS(SELECT 1 FROM audit_dead_letter)
	OR EXISTS(SELECT 1 FROM corrupt_events)`

func isQueueEmpty(db *sql.DB) (bool, error) {
	var hasRows int
	err := db.QueryRow(isEmptyQuery).Scan(&hasRows)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return hasRows == 0, nil
}
