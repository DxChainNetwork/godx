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
	"enode://d14731a00f672b7d06e2af698eddf6ab7168cfef614465156f5f82afd5e3a8189aaf407490ee267d18a77dad509e59664aee2fd9e7783896c1b7c6a3d11c046c@18.237.242.241:36000",
	"enode://2ee7b4e59e6a82a084ab668707538fccbff2480e42bad1cd787f62cfb7d64be0860638979e701c7f43f57cb920d5f07893c80a6774573cc80e1f32669636579d@34.222.227.194:36000",
	"enode://5af0ddb41d14893d3989048fb3bcb87906da363c9b1be64c0ff7a32c6d991865e3376ad9ca6a5fa5d91f447442e857e1e26f9ccf93d81a6db1ab37a08c27564d@54.213.99.178:36000",
	"enode://019eebba31516cad21977d1de87ffeb97f6bf6a3627d599c63e112d228ccf7f227e37ce29b1892c736f6bdcc950c9322e10af8d22f32d4e4b676b27409b03f53@54.184.141.27:36000",
	"enode://b6f47e76d52bfb1e1703cb33b9a60b15c8bddd813f524bc295fc2285918ee3469d974864c731a0c900d79ffe7b1aed988343c4f5241eee9851fa22068fed9995@13.56.181.169:36000",
	//"enode://81981170fb0d5b906d0e9e31cf41252f022d8130a406ae0db70ae5fb8e0033e423c036e02dcb3abf6fbc6be5836818ba92ea911724690aab322a60a73694ec01@34.212.1.21:36000",
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
