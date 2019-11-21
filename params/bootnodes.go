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
	"enode://fd8e941cba9dda2ee0052b49b27468b66ea1e419bb5182071a0b13b2ca14ae2ea064c0408a6007e97e14c2066406a2ba652e0863a013184912c799d7eb366a5b@13.250.109.200:36000",
	"enode://2a64b96db089ae8e6a6aa1c9031eff2ff184b4c8905cb5472ca14f87f69fdda8556726635d1693924d98198727f62daf74495d067af2c2da96cc975a3c09f4a7@54.254.181.45:36000",
	"enode://63a5c908d79c04e45cca7005b9c6261c266aaf847e11c665d88819dbb52a5d9ee76d708ba8196c3371bd2a2a9f389cc64d62c877194d4eeec8a83a22a7fc7981@18.141.13.169",
	"enode://d40c6ed46e0049a849574b8515f52ca5dabd594a6c5639c10b6f019d2995cb8c9c977711cdf7239cee71434bff5e528a32e2016549ae67f71b01a0b0372e75de@54.255.208.200",
	"enode://217c146466d651b1407b9677621c2bd36a9153fe8c4f6d0d89370f99410842d17992e2bf4027c9bc32ab789da8925d9d5de59045082bc3268f33310ec713efdc@18.141.12.83",
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
