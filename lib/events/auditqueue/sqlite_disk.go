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

func getDSN(dbPath string, maxBytes int64) string {
	params := url.Values{}
	params.Add("_pragma", "auto_vacuum(INCREMENTAL)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
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

	db, err := initializeDb(cfg.Path, cfg.MaxBytes)
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

func initializeDb(path string, maxBytes int64) (*sql.DB, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	dbPath := filepath.Join(path, queueDBFile)
	db, err := sql.Open("sqlite", getDSN(dbPath, maxBytes))
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
// will reside. It does this in a way that is safe from race conditions.
func initQueueDir(path string) (func() error, error) {
	// Create the tmp directory with the suffix `.tmp`
	// This avoids a race conditions from other audit log instances attempting
	// to claim this as an orphaned directory before it is fully initialized.
	tmpPath := path + tmpDirSuffix
	if err := os.MkdirAll(tmpPath, 0o700); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Take the lock on the `queue.lock` file. This ensures that no other
	// instances try to adopt this queue.
	unlock, err := utils.FSTryWriteLock(filepath.Join(tmpPath, queueLockFile))
	if err != nil {
		_ = os.RemoveAll(tmpPath)
		return nil, trace.Wrap(err)
	}

	// Remove the `.tmp` suffix, marking this as a live queue.
	if err := os.Rename(tmpPath, path); err != nil {
		_ = unlock()
		_ = os.RemoveAll(tmpPath)
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
		q.orphanScanLoop(ctx, handler)
	})
	wg.Go(func() {
		q.deadLetterSweepLoop(ctx, handler)
	})
	defer wg.Wait()

	q.runPollLoop(ctx, handler)
	return nil
}

func (q *sqliteQueue) orphanScanLoop(ctx context.Context, handler Handler) {
	q.sweepStaleTmp()
	q.adoptOrphans(ctx, handler)

	timer := time.NewTimer(q.orphanScanInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.ctx.Done():
			return
		case <-timer.C:
			q.sweepStaleTmp()
			q.adoptOrphans(ctx, handler)
			timer.Reset(q.orphanScanInterval)
		}
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

		// Tmp directories were orphaned before they finished initializing,
		// therefore they will have no audit log events, hence they are safe to
		// remove.
		if err := os.RemoveAll(stalePath); err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to remove stale audit-queue tmp directory.",
				"path", stalePath,
				"error", err,
			)
		}
	}
}

func (q *sqliteQueue) adoptOrphans(ctx context.Context, handler Handler) {
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

		// TODO:(kkloberdanz): We need to ensure that we only adopt a queue if
		// we have the appropriate inner.EmitAuditEvent. Perhaps we can do this
		// on a component basis where we check if the adopted queue is from SSH,
		// DB, etc. Will fix in a follow up.

		// If we got to this point, then we have found a directory that is not a
		// tmp directory and is not the same directory as the one this process
		// is already using. We are safe to attempt to adopt it.
		q.tryAdoptOrphan(entryPath, handler)
	}
}

func (q *sqliteQueue) tryAdoptOrphan(path string, handler Handler) {
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
	db, err := sql.Open("sqlite", getDSN(dbFilePath, defaultMaxBytes))
	if err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx, "Failed to open orphan SQLite database.", "path", path, "error", err)
		return
	}
	db.SetMaxOpenConns(1)

	drained := q.drainOrphanDB(db, handler)
	if err := db.Close(); err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to close orphan SQLite database.",
			"path", path,
			"error", err,
		)
		return
	}

	if !drained {
		// If we fail to drain, we will try again on the next orphan adoption
		// cycle.
		return
	}

	// If we got here, then we have successfully drained the orphan. We can now
	// safely remove it.
	if err := os.RemoveAll(path); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to remove drained orphan directory.",
			"path", path,
			"error", err,
		)
		return
	}
	orphansAdopted.Inc()
	slog.InfoContext(q.ctx, "Adopted orphaned audit-queue directory.", "path", path)
}

func (q *sqliteQueue) drainOrphanDB(db *sql.DB, handler Handler) bool {
	for {
		if q.ctx.Err() != nil {
			return false
		}
		items, err := fetchDB(q.ctx, db, dequeueBatchSize)
		if err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx, "Failed to fetch orphan events.", "error", err)
			return false
		}
		if len(items) == 0 {
			break
		}
		successfullyDelivered := q.forwardBatch(q.ctx, handler, items)
		if len(successfullyDelivered) == 0 {
			return false
		}

		// Notice: We do not want to call q.ack() here. This is because this db
		// is an adopted db. Hence it is a different database than the one that
		// is currently held by `q`.
		if err := ackDB(q.ctx, db, successfullyDelivered); err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx, "Failed to ack orphan events.", "error", err)
			return false
		}

		// We didn't manage to drain the entire orphan. We will re-attempt to
		// drain it on the next orphan sweep.
		if len(successfullyDelivered) < len(items) {
			return false
		}
	}
	return q.migrateOrphanDeadLetter(db)
}

// migrateOrphanDeadLetter moves rows from the orphan's audit_dead_letter table
// into this queue's audit_dead_letter table.
func (q *sqliteQueue) migrateOrphanDeadLetter(orphan *sql.DB) bool {
	for {
		if q.ctx.Err() != nil {
			return false
		}
		batch, err := fetchOrphanDeadLetter(q.ctx, orphan, dequeueBatchSize)
		if err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to fetch orphan dead-letter events.",
				"error", err,
			)
			return false
		}
		if len(batch) == 0 {
			return true
		}
		if err := q.insertDeadLetterBatch(batch); err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to migrate orphan dead-letter events.",
				"error", err,
			)
			return false
		}
		ids := make([]int64, len(batch))
		for i, r := range batch {
			ids[i] = r.id
		}
		if err := deleteIDsFromTable(q.ctx, orphan, "audit_dead_letter", ids); err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to delete migrated orphan dead-letter rows.",
				"error", err,
			)
			return false
		}
	}
}

func fetchOrphanDeadLetter(ctx context.Context, db *sql.DB, limit int) ([]deadLetterRow, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, payload, failed_at FROM audit_dead_letter ORDER BY id ASC LIMIT ?", limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rows.Close()
	var out []deadLetterRow
	for rows.Next() {
		var r deadLetterRow
		if err := rows.Scan(&r.id, &r.payload, &r.failedAt); err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, r)
	}
	return out, trace.Wrap(rows.Err())
}

func (q *sqliteQueue) insertDeadLetterBatch(batch []deadLetterRow) error {
	if len(batch) == 0 {
		return nil
	}
	tx, err := q.db.BeginTx(q.ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tx.Rollback()
	if err := insertDeadLetterTx(q.ctx, tx, batch); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(tx.Commit())
}

func (q *sqliteQueue) Close() error {
	var firstErr error
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
			firstErr = errors.Join(firstErr, err)
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
				firstErr = errors.Join(firstErr, err)
			}
		}
	})
	return trace.Wrap(firstErr)
}

func isQueueEmpty(db *sql.DB) (bool, error) {
	var hasRows int
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM audit_queue) OR EXISTS(SELECT 1 FROM audit_dead_letter)",
	).Scan(&hasRows)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return hasRows == 0, nil
}
