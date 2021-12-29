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
	worker
}

var _ Worker = (*Dumper)(nil)

func NewDumper(ctx context.Context, client api.GobgpApiClient,
	concurrent int, logger *zap.SugaredLogger, name string, wg *sync.WaitGroup) *Dumper {
	return &Dumper{
		worker{
			cli:      client,
			workers:  concurrent,
			ctx:      ctx,
			wg:       wg,
			interval: 100 * time.Millisecond,
			log:      logger,
			Name:     name,
		},
	}
}

func (d *Dumper) Once() {
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
			d.log.Errorf("failed to list rib: %v\n", err)
			return
		}

		if st.Code() == codes.Canceled {
			d.log.Debugf("grpc stub %s", st.Code())
			return
		}
	}

	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			if st, ok := status.FromError(err); ok {
				d.log.Debugf("expected grpc return: %s", st.Code())
			}
			break
		}
	}
}

func (d *Dumper) Loop() {
	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go func(id int) {
			defer d.wg.Done()
			ticker := time.NewTicker(d.interval)
			for {
				select {
				case <-ticker.C:
					d.Once()
				case <-d.ctx.Done():
					d.log.Infof("context done returned from %s-%d", d.Name, id)
					return
				}
			}
		}(i)
	}
}
