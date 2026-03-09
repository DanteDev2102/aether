package main

import (
	"context"
	"time"
)

type CronJob func(ctx context.Context, log Logger)

type cronEntry struct {
	interval time.Duration
	job      CronJob
	name     string
}

type CronManager struct {
	entries []cronEntry
	log     Logger
	ctx     context.Context
	cancel  context.CancelFunc
}

func newCronManager(log Logger) *CronManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &CronManager{
		entries: make([]cronEntry, 0),
		log:     log,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (c *CronManager) Add(name string, interval time.Duration, job CronJob) {
	c.entries = append(c.entries, cronEntry{
		interval: interval,
		job:      job,
		name:     name,
	})
	c.log.Infof("Cron job registered: %s (interval: %v)", name, interval)
}

func (c *CronManager) Start() {
	if len(c.entries) == 0 {
		return
	}
	
	c.log.Infof("Starting %d cron jobs...", len(c.entries))
	
	for _, entry := range c.entries {
		go c.runJob(entry)
	}
}

func (c *CronManager) Stop() {
	c.log.Info("Stopping all cron jobs...")
	c.cancel()
}

func (c *CronManager) runJob(entry cronEntry) {
	execute := func() {
		defer func() {
			if r := recover(); r != nil {
				c.log.Errorf("Panic recovered in cron job '%s': %v", entry.name, r)
			}
		}()
		entry.job(c.ctx, c.log)
	}

	ticker := time.NewTicker(entry.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			execute()
		case <-c.ctx.Done():
			c.log.Infof("Cron job stopped: %s", entry.name)
			return
		}
	}
}
