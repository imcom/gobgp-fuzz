package fuzz

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	api "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/apiutil"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"go.uber.org/zap"
	"inet.af/netaddr"
)

type Advertiser struct {
	worker

	Withdrawn   chan netaddr.IPPrefix
	InjectionCh chan netaddr.IPPrefix
	CidrPool    []string
}

var _ Worker = (*Advertiser)(nil)

func makePathRequest(prefix netaddr.IPPrefix) *api.Path {
	mask := prefix.Bits()
	ip := prefix.IP()
	nlri := bgp.NewIPAddrPrefix(mask, ip.String())
	var attrs []bgp.PathAttributeInterface
	// nexthop
	attrs = append(attrs, bgp.NewPathAttributeNextHop("169.254.0.1"))
	// community
	attrs = append(attrs, bgp.NewPathAttributeCommunities([]uint32{uint32(63005<<16 | 63005)}))
	// origin
	attrs = append(attrs, bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_INCOMPLETE))
	// as path
	asPathParams := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{63005, 63005})}
	attrs = append(attrs, bgp.NewPathAttributeAsPath(asPathParams))

	path, _ := apiutil.NewPath(nlri, false, attrs, time.Now())

	return path
}

func NewAdvertiser(ctx context.Context, client api.GobgpApiClient,
	concurrent int, logger *zap.SugaredLogger, name string, cidrPool []string, wg *sync.WaitGroup) *Advertiser {

	return &Advertiser{
		worker: worker{
			cli:      client,
			workers:  concurrent,
			ctx:      ctx,
			wg:       wg,
			interval: 100 * time.Millisecond,
			log:      logger,
			Name:     name,
		},
		Withdrawn:   make(chan netaddr.IPPrefix, 1000),
		InjectionCh: make(chan netaddr.IPPrefix, 1000),
		CidrPool:    cidrPool,
	}
}

func (d *Advertiser) Once() {
	b := netaddr.IPSetBuilder{}
	for _, prefix := range d.CidrPool {
		p := netaddr.MustParseIPPrefix(prefix)
		b.AddPrefix(p)
		left := int(p.Masked().Bits())
		right := 32 // assume ipv4 only
		rp := fmt.Sprintf("%s/%d", p.IP().String(), rand.Intn(right-left)+left)
		b.RemovePrefix(netaddr.MustParseIPPrefix(rp))
	}
	candidates, _ := b.IPSet()
	for _, prefix := range candidates.Prefixes() {
		d.InjectionCh <- prefix
	}
}

func (d *Advertiser) Loop() {
	go func() {
		d.wg.Add(1)
		defer d.wg.Done()
		d.Once()
		ticker := time.NewTicker(d.interval)

		for {
			select {
			case <-ticker.C:
				d.Once()
			case <-d.ctx.Done():
				d.log.Infof("context done returned from %s", d.Name)
				return
			}
		}
	}()

	for i := 0; i < d.workers; i++ {
		go func() {
			d.wg.Add(1)
			ctx, cancel := context.WithCancel(d.ctx)
			defer cancel()
			defer d.wg.Done()
			for {
				select {
				case prefix := <-d.InjectionCh:
					path := makePathRequest(prefix)
					if _, err := d.cli.AddPath(ctx, &api.AddPathRequest{
						TableType: api.TableType_GLOBAL,
						Path:      path,
						VrfId:     "", // not using Vrf
					}); err == nil && prefix.Bits()%(uint8(time.Now().Second())+1) == 0 {
						// randomly withdrawn
						d.Withdrawn <- prefix
					} else if err != nil {
						d.log.Error(err)
					}
				case <-d.ctx.Done():
					return
				}
			}
		}()

		go func() {
			d.wg.Add(1)
			ctx, cancel := context.WithCancel(d.ctx)
			defer cancel()
			defer d.wg.Done()
			for {
				select {
				case prefix := <-d.Withdrawn:
					// we know that this prefix is withdrawn
					// so we add it back to make it mess
					path := makePathRequest(prefix)
					d.cli.AddPath(ctx, &api.AddPathRequest{
						TableType: api.TableType_GLOBAL,
						Path:      path,
						VrfId:     "", // not using Vrf
					})
				case <-d.ctx.Done():
					return
				}
			}
		}()
	}
}
