package executor

import (
	"context"

	"github.com/harshRZP/job-scheduler/internal/domain"
)

type JobExecutor interface {
	Execute(ctx context.Context, job domain.Job) domain.RunResult
}
