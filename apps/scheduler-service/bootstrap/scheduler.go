package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Scheduler wraps robfig/cron with distributed locking via Redis SET NX.
// Only one pod will execute each job at a time, making it safe for multi-replica deployments.
type Scheduler struct {
	cron   *cron.Cron
	redis  *redis.Client
	logger *zap.Logger

	// job registry for ListJobs API
	jobs []JobMeta
}

type JobMeta struct {
	Name     string
	Schedule string
}

func NewScheduler(redisClient *redis.Client, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		redis:  redisClient,
		logger: logger,
	}
}

// AddJob registers a job with distributed lock guard.
func (s *Scheduler) AddJob(name, schedule string, fn func(ctx context.Context)) error {
	_, err := s.cron.AddFunc(schedule, func() {
		ctx := context.Background()
		lockKey := fmt.Sprintf("cron:%s", name)

		// Parse schedule to get interval for TTL
		p, err := cron.ParseStandard(schedule)
		if err != nil {
			s.logger.Error("parsing cron schedule", zap.String("job", name), zap.Error(err))
			return
		}
		now := time.Now()
		next := p.Next(now)
		ttl := next.Sub(now)

		// Distributed lock: only 1 pod runs the job per interval
		ok, err := s.redis.SetNX(ctx, lockKey, 1, ttl).Result()
		if err != nil {
			s.logger.Error("acquiring cron lock", zap.String("job", name), zap.Error(err))
			return
		}
		if !ok {
			s.logger.Debug("cron lock held by another pod — skipping", zap.String("job", name))
			return
		}

		s.logger.Info("running cron job", zap.String("job", name))
		fn(ctx)
	})
	if err != nil {
		return fmt.Errorf("adding cron job %s: %w", name, err)
	}
	s.jobs = append(s.jobs, JobMeta{Name: name, Schedule: schedule})
	return nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) Jobs() []JobMeta {
	return s.jobs
}
