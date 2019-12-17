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
	"enode://56edc0f928f4926ace684134392e8c3015997efd6c0bba74ac632a23f2accfac43f30d29c9421911b4461501270bf8424bb2cd1b546537ddd3b40d99fa4fb441@44.229.3.163:36000",
	"enode://af613df257a7f2b371653caafe90172d0be38f0d845f5ec4f8ca7557d5a3b63f707af790cd0615c13f328e12e5bbf1fa41352348456740847168ac26c0cbd108@44.229.3.35:36000",
	"enode://984ad4ac4329e237d3ef2a76a27a013c6c7d1420a6d4c76de0cc56417f1f80cd65f47fd3a559334b84e76da813a4f26997533bbf84a22e4dec135ae4f850dfd5@44.229.3.191:36000",
	"enode://cda2a596123b0e3d7545ef7b63bfe6f66e42e19fb35e92506f68f24e857fe26aa7b3c4274a5ad1c674d2e48d0167a326eb60e3edbb5e9038c376d38938069b10@44.229.3.179:36000",
	"enode://ae3da7096ad7689ea34a28eaedbdd15e4b695521a2ef3e4a2db8e95f5ab4a692fcf15839716d1b4f62e61d11b375c7647a002f43fa66f67d9074c526f5b9d557@44.229.3.43:36000",
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
