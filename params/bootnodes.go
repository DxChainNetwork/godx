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
	"enode://dd876f54fa5cff74eb4c703110f081e19401b154932150cbdd9f6764a254c34d6a9db31517d9dc1f136655f423191bf825cc64affa999f24540cfb1b1b8f08e6@52.9.177.26:36000",
	"enode://b0b28d11d2b87e66c256941eca3285500b459c8452a6ef38999179ba32f896bf4860dd822f8fdf9d081386c77076b9dde37c3337679d6cff6b5271433a4f8817@13.52.191.86:36000",
	"enode://30f4950031e2e1488070cbf0041308b1553cb0b3a539bef132d93437a47f23fd40c265c76b149161997becec3db18df283b74884e2d5398f21e132599e6f147c@13.52.194.229:36000",
	"enode://a1fd95166bf42a1d6238790f498555ea1d72ac5448594071d345b59a9649e28bbca3bbbda07787ee6f7070a8d026e1ffb41ea15c7e573ba9d2ec0ca5cc60ac52@13.52.208.156:36000",
	"enode://4a4b62e24769022030fcb47260f1c46e4c796298b892ca0d3de8ef44f34e835f372d40862b665dd883f0a2509d0e7c2f9fd7d6cae597088ea26d006f9c3d271f@13.56.190.57:36000",
	"enode://d1bae01d296fe857178968582939f81782acdcf295e0c1b73a6269e9d40986cea45b7fc1ac3429a8b2d9d02d33ea32d94a8f3c4ea5aa297be0dbbb71a20bc6e5@52.52.198.191:36000",
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
