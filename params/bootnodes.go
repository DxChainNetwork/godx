// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

// MainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the main Ethereum network.
var MainnetBootnodes = []string{
	"enode://52928f3b2875f8c26e0b6ec7921fa9e49ed64105117ad3ac2d3f62591e6132f14e312410bd38a550d3351831ada767131975b65785626b263c3c5a2a1e86e720@18.237.16.215:36000",
	"enode://abe6480e26404a9e1a4b3d79f1723ae623fbedcab4ddcd1481caf9408850686a7bd2edf00fa9dfbd8777a834fb3d97d122a4c1fff5abfe33ec8d0490e78d9e9f@54.212.128.142:36000",
	"enode://cf66a76107ec24a24c2cd6d38086ed1c0083f12aa7c0228df3fc57ed805b14824544e89f11889078bc420a213320abc734fb0d15fc1174b50267cb6fe2a4228b@54.187.162.162:36000",
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// Ropsten test network.
var TestnetBootnodes = []string{}

// RinkebyBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// Rinkeby test network.
var RinkebyBootnodes = []string{}

// DiscoveryV5Bootnodes are the enode URLs of the P2P bootstrap nodes for the
// experimental RLPx v5 topic-discovery network.
var DiscoveryV5Bootnodes = []string{}
