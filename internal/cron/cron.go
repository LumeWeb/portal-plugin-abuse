package cron

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/cron/define"
	"go.lumeweb.com/portal-plugin-abuse/internal/cron/tasks"
	"go.lumeweb.com/portal/core"
)

// Cron implements Portal's Cronable interface for the abuse plugin
type Cron struct{}

// RegisterTasks registers all cron tasks with the Portal cron service
func (c Cron) RegisterTasks(crn core.CronService) error {
	// Register the scan task
	crn.RegisterTask(
		define.CronTaskScanName,
		core.CronTaskFuncHandler[*define.CronTaskScanArgs](tasks.CronTaskScan),
		core.CronTaskDefinitionOneTimeJob,
		define.CronTaskScanArgsFactory,
		false,
	)

	return nil
}

// ScheduleJobs schedules recurring jobs
// We don't have any recurring jobs for abuse, just ad-hoc scan tasks
func (c Cron) ScheduleJobs(_ core.CronService) error {
	return nil
}

// NewCron creates a new Cron instance
func NewCron(_ core.Context) *Cron {
	return &Cron{}
}
