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
	wg       *sync.WaitGroup
	quitCh   chan struct{}
	interval time.Duration
	log      *zap.SugaredLogger
}

type Worker interface {
	Once()
	Loop()
}
