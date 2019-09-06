// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

func ContractDownloadHandler(np NegotiationProtocol, sp storage.Peer, downloadReqMsg p2p.Msg) {
	var negotiateErr error
	var nd downloadNegotiationData
	defer handleNegotiationErr(&negotiateErr, sp, np)

}
