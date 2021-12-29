package fuzz

import (
	"context"
	"sync"
	"time"

	api "github.com/osrg/gobgp/v3/api"
	"go.uber.org/zap"
)

type worker struct {
	Name     string
	cli      api.GobgpApiClient
	workers  int
	ctx      context.Context
	wg       sync.WaitGroup
	quitCh   chan struct{}
	interval time.Duration
	log      *zap.SugaredLogger
}

type Worker interface {
	Once()
	Loop(func())
}

func (w *worker) loop(fn func()) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	for {
		select {
		case <-ticker.C:
			fn()
		case <-w.ctx.Done():
			w.log.Info("context done")
			return
		}
	}
}

func (w *worker) Loop(fn func()) {
	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.loop(fn)
	}

	w.log.Infof("awaiting for %s routines", w.Name)
	// wait for all dumper routines to quit
	w.wg.Wait()

	w.log.Infof("all %s routines finished", w.Name)
	// signal caller that dumper has quitted
	w.quitCh <- struct{}{}
}
