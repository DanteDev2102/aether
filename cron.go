package aether

import (
	"context"
	"time"
	"sync"
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
	wg      sync.WaitGroup
}

func newCronManager(log Logger) *CronManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &CronManager{
		entries: make([]cronEntry, 0),
		log:     log,
		ctx:     ctx,
		cancel:  cancel,
		wg: 	 sync.WaitGroup{}
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
		c.wg.Add(1)
		go func(e cronEntry) {
			defer c.wg.Done()
			c.runJob(e)
		}(entry)
	}
}

func (c *CronManager) Stop() {
	c.log.Info("Stopping all cron jobs...")
	c.cancel()
	c.wg.Wait()
	c.log.Info("All cron jobs stopped gracefully.")
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

	execute()

	for {
		timer := time.NewTimer(entry.interval)
		
		select {
		case <-timer.C:
			execute()
		case <-c.ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			c.log.Infof("Cron job stopped: %s", entry.name)
			return
		}
	}
}
