// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

// Pre-define test cases used message code for storage client
// side negotiation
const (
	// Contract Create Related Message Code
	FakeContractCreateRequestSendFailed = 0x00
	FakeContractCreateRequestFailed     = 0x01
	FakeContractCreateHostTooBusy       = 0x02

	FakeContractCreateRevisionSendFailed  = 0x03
	FakeContractCreateRevisionFailed      = 0x04
	FakeContractCreateRevisionHostTooBusy = 0x05

	FakeClientCommitSuccessFailed = 0x06

	FakeUploadRequestSendFailed  = 0x07
	FakeUploadRequestFailed      = 0x08
	FakeUploadRequestHostTooBusy = 0x09

	FakeUploadRevisionSignSendFailed  = 0x0A
	FakeUploadRevisionSignFailed      = 0x0B
	FakeUploadRevisionSignHostTooBusy = 0x0C
)

// Pre-define test case steps to make sure if it should be failed or success
const (
	FakeSetUpConnectionFailed = 0x50
)
