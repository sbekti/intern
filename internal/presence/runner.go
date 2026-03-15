package presence

import (
	"context"
	"log/slog"
	"time"

	"github.com/sbekti/intern-api/internal/config"
	"golang.org/x/sync/errgroup"
)

type Runner struct {
	logger  *slog.Logger
	cfg     config.PresenceConfig
	service *Service
}

func NewRunner(logger *slog.Logger, cfg config.PresenceConfig, service *Service) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		logger:  logger,
		cfg:     cfg,
		service: service,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	if !r.cfg.Enabled {
		r.logger.Info("presence worker disabled")
		return nil
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return r.runLoop(ctx, "radius", r.cfg.PollIntervalDefault, r.service.SyncRadius)
	})

	for _, source := range r.cfg.Sources {
		if source.Type != config.PresenceSourceTypeUnifi {
			continue
		}

		source := source
		group.Go(func() error {
			return r.runLoop(ctx, source.Key, source.PollInterval, func(loopCtx context.Context) error {
				return r.service.SyncUniFiSource(loopCtx, source)
			})
		})
	}

	return group.Wait()
}

func (r *Runner) runLoop(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		r.logger.Error("presence sync failed", "loop", name, "error", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				r.logger.Error("presence sync failed", "loop", name, "error", err)
			}
		}
	}
}
