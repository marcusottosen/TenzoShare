// Package cleanup implements the background file retention worker.
// It runs every hour and deletes files that have exceeded their retention period.
package cleanup

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tenzoshare/tenzoshare/services/storage/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/storage/internal/repository"
	sharedStorage "github.com/tenzoshare/tenzoshare/shared/pkg/storage"
)

// maxFilesPerCycle is a hard safety cap on the number of files deleted in a single
// worker cycle. If the eligibility query returns more than this, the cycle is capped
// and a critical warning is logged so operators can investigate. This prevents a
// misconfigured retention policy from wiping the entire file store at once.
const maxFilesPerCycle = 500

// Worker periodically deletes files that have exceeded the configured retention period.
type Worker struct {
	repo    *repository.FileRepository
	backend sharedStorage.Backend
	log     *zap.Logger
	ticker  *time.Ticker
}

// New creates a Worker. Call Run(ctx) to start it.
func New(repo *repository.FileRepository, backend sharedStorage.Backend, log *zap.Logger) *Worker {
	return &Worker{
		repo:    repo,
		backend: backend,
		log:     log,
	}
}

// Run starts the cleanup loop. It returns when ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	w.ticker = time.NewTicker(1 * time.Hour)
	defer w.ticker.Stop()

	// Delay the first cycle by 30 seconds so DB migrations have time to finish
	// before we attempt cross-schema queries on first boot.
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}

	w.runCycle(ctx)

	for {
		select {
		case <-ctx.Done():
			w.log.Info("cleanup worker stopped")
			return
		case <-w.ticker.C:
			w.runCycle(ctx)
		}
	}
}

func (w *Worker) runCycle(ctx context.Context) {
	cfg, err := w.repo.GetStorageConfig(ctx)
	if err != nil {
		w.log.Warn("cleanup: could not load storage config", zap.Error(err))
		return
	}
	if !cfg.RetentionEnabled {
		return
	}

	retDays := cfg.RetentionDays
	orphDays := cfg.OrphanRetentionDays
	if retDays <= 0 {
		retDays = 30
	}
	if orphDays <= 0 {
		orphDays = 90
	}

	candidates, err := w.repo.FindFilesEligibleForDeletion(ctx, retDays, orphDays)
	if err != nil {
		w.log.Error("cleanup: error finding eligible files", zap.Error(err))
		return
	}

	if len(candidates) == 0 {
		w.log.Debug("cleanup: no files eligible for deletion")
		return
	}

	// Safety cap: never delete more than maxFilesPerCycle files in a single run.
	// If the count exceeds the cap, log a critical warning so operators can review
	// the retention policy — this likely indicates misconfiguration.
	if len(candidates) > maxFilesPerCycle {
		w.log.Error("cleanup: eligible file count exceeds safety cap — capping this cycle",
			zap.Int("eligible", len(candidates)),
			zap.Int("cap", maxFilesPerCycle),
			zap.String("action", "review retention policy configuration"),
		)
		candidates = candidates[:maxFilesPerCycle]
	}

	w.log.Info("cleanup: starting purge cycle", zap.Int("candidates", len(candidates)))

	deleted := 0
	var freedBytes int64
	for _, fd := range candidates {
		if err := w.purgeFile(ctx, fd); err != nil {
			w.log.Warn("cleanup: failed to purge file",
				zap.String("file_id", fd.ID),
				zap.String("reason", fd.Reason),
				zap.Error(err))
			continue
		}
		deleted++
		freedBytes += fd.SizeBytes
	}

	w.log.Info("cleanup: purge cycle complete",
		zap.Int("deleted", deleted),
		zap.Int64("freed_bytes", freedBytes),
	)

	// Secondary pass: clean up MinIO objects for files that were soft-deleted by the
	// admin service (which can't reach MinIO directly). These files have deleted_at set
	// but no purge_log entry. Best-effort; errors are non-fatal.
	w.purgeOrphanedObjects(ctx)
}

func (w *Worker) purgeOrphanedObjects(ctx context.Context) {
	pending, err := w.repo.FindSoftDeletedPendingObjectPurge(ctx)
	if err != nil {
		w.log.Warn("cleanup: could not query pending object purges", zap.Error(err))
		return
	}
	for _, fd := range pending {
		if err := w.backend.Delete(ctx, fd.ObjectKey); err != nil {
			w.log.Warn("cleanup: orphaned object delete failed",
				zap.String("file_id", fd.ID),
				zap.String("object_key", fd.ObjectKey),
				zap.Error(err))
		}
		_ = w.repo.RecordPurge(ctx, fd, "system")
	}
	if len(pending) > 0 {
		w.log.Info("cleanup: purged orphaned objects", zap.Int("count", len(pending)))
	}
}

func (w *Worker) purgeFile(ctx context.Context, fd *domain.FileToDelete) error {
	// 1. Soft-delete the DB record first (idempotent).
	if err := w.repo.SoftDeleteByID(ctx, fd.ID); err != nil {
		return err
	}

	// 2. Remove the object from MinIO. Non-fatal if it already doesn't exist.
	if err := w.backend.Delete(ctx, fd.ObjectKey); err != nil {
		w.log.Warn("cleanup: object delete failed (object may already be gone)",
			zap.String("object_key", fd.ObjectKey),
			zap.Error(err))
	}

	// 3. Write to audit log.
	_ = w.repo.RecordPurge(ctx, fd, "system")

	w.log.Info("cleanup: purged file",
		zap.String("file_id", fd.ID),
		zap.String("filename", fd.Filename),
		zap.String("reason", fd.Reason),
		zap.Int64("size_bytes", fd.SizeBytes),
	)
	return nil
}
