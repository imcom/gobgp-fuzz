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

func NewDumper(ctx context.Context, client api.GobgpApiClient,
	workers int, dumperCh chan struct{}, logger *zap.SugaredLogger, name string) *Dumper {
	return &Dumper{worker{
		cli:      client,
		workers:  workers,
		ctx:      ctx,
		wg:       sync.WaitGroup{},
		quitCh:   dumperCh,
		interval: 100 * time.Millisecond,
		log:      logger,
		Name:     name,
	}}
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
