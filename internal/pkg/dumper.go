package fuzz

import (
	"context"
	"io"
	"sync"
	"time"

	api "github.com/osrg/gobgp/v3/api"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Dumper struct {
	cli      api.GobgpApiClient
	workers  int
	ctx      context.Context
	wg       sync.WaitGroup
	quitCh   chan struct{}
	interval time.Duration
	log      *zap.SugaredLogger
}

func NewDumper(ctx context.Context, client api.GobgpApiClient,
	workers int, dumperCh chan struct{}, logger *zap.SugaredLogger) *Dumper {
	return &Dumper{
		cli:      client,
		workers:  workers,
		ctx:      ctx,
		wg:       sync.WaitGroup{},
		quitCh:   dumperCh,
		interval: 100 * time.Millisecond,
		log:      logger,
	}
}

func (d *Dumper) dump() {
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	defer cancel()
	stream, err := d.cli.ListPath(ctx, &api.ListPathRequest{
		TableType: api.TableType_GLOBAL,
		Family: &api.Family{
			Afi:  api.Family_AFI_IP,
			Safi: api.Family_SAFI_UNICAST,
		},
		Name:     "",
		Prefixes: nil,
		SortType: api.ListPathRequest_PREFIX,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			d.log.Errorf("failed listing rib: %v\n", err)
			return
		}

		if st.Code() == codes.Canceled {
			d.log.Info("grpc stub canceled")
			return
		}
	}

	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			if st, ok := status.FromError(err); ok {
				d.log.Infof("grpc error: %s", st.Code())
			}
			break
		}
	}
}

func (d *Dumper) loop() {
	defer d.wg.Done()
	dumpTicker := time.NewTicker(d.interval)
	for {
		select {
		case <-dumpTicker.C:
			d.dump()
		case <-d.ctx.Done():
			d.log.Info("context done")
			return
		}
	}
}

func (d *Dumper) Run() {
	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go d.loop()
	}

	d.log.Info("awaiting for dumper routines")
	// wait for all dumper routines to quit
	d.wg.Wait()

	d.log.Info("all dumper routines finished")
	// signal caller that dumper has quitted
	d.quitCh <- struct{}{}
}
