// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"fmt"
	"net"
)

// Filter defines IP filter map. For any IP addresses with same IP Network will be marked
// and filter needed. IP address can be extracted from the enode information
type Filter struct {
	filterPool map[string]struct{}
}

// NewFilter will create and initialize a Filter object
func NewFilter() *Filter {
	return &Filter{
		filterPool: make(map[string]struct{}),
	}
}

// Add will add the IP Network of the IP address in to the filter
func (f *Filter) Add(ip string) {
	ipnet, err := IPNetwork(ip)
	if err != nil {
		return
	}

	// add the IP Network to the filter
	f.filterPool[ipnet.String()] = struct{}{}
}

// Filtered will check if an IP address uses a IP Network that is already in used
// return true indicates the IP Network is in use
func (f *Filter) Filtered(ip string) bool {
	ipnet, err := IPNetwork(ip)
	if err != nil {
		return false
	}

	if _, exists := f.filterPool[ipnet.String()]; exists {
		return true
	}

	return false
}

// Reset will clear the filter
func (f *Filter) Reset() {
	f.filterPool = make(map[string]struct{})
}

// IPNetwork will return the IP network used by an IP address
func IPNetwork(ip string) (ipnet *net.IPNet, err error) {
	cidr := fmt.Sprintf("%s/%d", ip, IPv4PrefixLength)
	_, ipnet, err = net.ParseCIDR(cidr)
	return
}
