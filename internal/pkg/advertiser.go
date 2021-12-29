package fuzz

import (
	"net"

	api "github.com/osrg/gobgp/v3/api"
)

type Advertiser struct {
	cli        api.GobgpApiClient
	workers    int
	Advertised []net.Addr
	Withdrawn  []net.Addr
}
