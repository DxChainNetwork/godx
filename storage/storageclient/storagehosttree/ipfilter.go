// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"fmt"
	"net"
)

// Filter defines IP filter map. For any IP addresses with same subnet will be marked
// and filter needed
// ip address can be extracted from the enode information
type Filter struct {
	filter map[string]struct{}
}

// NewFilter will create and initialize a Filter object
func (f *Filter) NewFilter() *Filter {
	return &Filter{
		filter: make(map[string]struct{}),
	}
}

// Add will add the subnet of the IP address in to the filter
func (f *Filter) Add(ip string) {
	sub, err := IPSubnet(ip)
	if err != nil {
		return
	}

	// add the subnet to the filter
	f.filter[sub.String()] = struct{}{}
}

// Filtered will check if an IP address uses a subnet that is already in used
// return true indicates the subnet is in use
func (f *Filter) Filtered(ip string) bool {
	sub, err := IPSubnet(ip)
	if err != nil {
		return false
	}

	if _, exists := f.filter[sub.String()]; exists {
		return true
	}

	return false
}

// Reset will clear the filter
func (f *Filter) Reset() {
	f.filter = make(map[string]struct{})
}

// IPSubnet will return the subnet used by an IP address
func IPSubnet(ip string) (sub *net.IPNet, err error) {
	cidr := fmt.Sprintf("%s/%d", ip, IPv4PrefixLength)
	_, sub, err = net.ParseCIDR(cidr)
	return
}
