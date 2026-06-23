package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/revunix/defqon1-recorder/internal/logging"
	"github.com/revunix/defqon1-recorder/internal/mixlr"
	"github.com/revunix/defqon1-recorder/internal/recorder"
	"github.com/revunix/defqon1-recorder/internal/status"
	"github.com/revunix/defqon1-recorder/internal/timetable"
	"github.com/revunix/defqon1-recorder/internal/util"
)

type Controller struct {
	client    *mixlr.Client
	recorder  *recorder.Manager
	timetable *timetable.Timetable
	status    *status.Registry
	log       logging.Logger
	channels  []string
}

func New(
	client *mixlr.Client,
	rec *recorder.Manager,
	tt *timetable.Timetable,
	reg *status.Registry,
	log logging.Logger,
	channels []string,
) *Controller {
	return &Controller{
		client:    client,
		recorder:  rec,
		timetable: tt,
		status:    reg,
		log:       log,
		channels:  channels,
	}
}

func (c *Controller) CheckOnce(ctx context.Context) {
	c.log.Info(fmt.Sprintf("--- Checking channels at %s ---",
		time.Now().In(util.Berlin()).Format("15:04:05")))

	var wg sync.WaitGroup
	for _, channel := range c.channels {
		channel := channel
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.checkChannel(ctx, channel)
		}()
	}
	wg.Wait()
}

func (c *Controller) checkChannel(ctx context.Context, channel string) {
	ch, err := c.client.Fetch(ctx, channel)
	if err != nil {
		c.log.Error(fmt.Sprintf("[%s] Fetch Error: %s", channel, err))
		// Keep the last known status on transient errors.
		return
	}

	stage := ch.Username
	if stage == "" {
		stage = channel
	}
	c.status.Set(channel, stage, ch.Live, ch.ListenerCount, ch.StreamURL)

	if ch.Live && ch.StreamURL != "" {
		if c.recorder.IsRecording(stage) {
			c.recorder.UpdateListeners(stage, ch.ListenerCount)
		} else {
			c.recorder.Start(stage, ch.StreamURL, ch.ListenerCount)
		}
		return
	}

	if c.recorder.IsRecording(stage) {
		c.log.Info(fmt.Sprintf("[%s] Offline. Stopping recording.", stage))
		c.recorder.Stop(stage)
	}
}

func (c *Controller) Run(ctx context.Context, checkInterval, stalledInterval time.Duration) {
	c.CheckOnce(ctx)

	check := time.NewTicker(checkInterval)
	stalled := time.NewTicker(stalledInterval)
	defer check.Stop()
	defer stalled.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-check.C:
			c.CheckOnce(ctx)
		case <-stalled.C:
			c.recorder.MonitorStalled()
		}
	}
}
