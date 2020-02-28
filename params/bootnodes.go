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
	"enode://8ae015a8cb0d1e62b02330d4d2213d7ed58f233273076175842f6d4f474ca391a961f205ad3a81e2ae8dfa098740f62405fe871a444954ce214525b4eb0f2c59@44.231.57.184:36000",
	"enode://465f342ddc9b2b6803925d00f44e688ef0f43f17a2a050d1b2bedfce6e84f98dd35884833f2af0e537ecd009278d402299ff98f4fef80972a4f61f0ab2cc0fa9@54.184.8.205:36000",
	"enode://fb8360210d1eb680743e5e588b6645cf641cb6326276f6e254505c9dad72a3df99aabd3f7b5f41da983fcd34b148bbc2f25fc69eaaa3a890a733e0b9d6d63b5e@54.184.127.54:36000",
	"enode://0b16570d9ccfe0a6934a769759fc09edca9858948f32223e182ee131196afa044ded9ac3c8f12639641c136a1b1e2a2e2bdb13525d93c82f88e3496685daa7ed@54.244.161.112:36000",
	"enode://1108918bfb41db3eb3ceb04e0e3646983f36876bb5518c8bc83eecdddb6049e06f74fe63c7adeeb226b33106eca872cf011b97acf709efa13613516fab0a3be4@54.212.9.15:36000",
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
