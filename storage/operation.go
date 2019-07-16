// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storage

import (
	"errors"
	"time"

	"github.com/DxChainNetwork/godx/p2p"
)

const (
	ContractOP OpCode = iota
	ConfigOP
)

type OpCode byte

type Operations map[OpCode]Operation

type Operation struct {
	msg chan p2p.Msg
}

func NewOperations(opCode OpCode) Operations {
	operations := make(Operations)
	operations[opCode] = NewOperation()
	return operations
}

func NewOperation() Operation {
	return Operation{
		msg: make(chan p2p.Msg, 1),
	}
}

func (op *Operation) WaitMsgReceive() (p2p.Msg, error) {
	timeout := time.After(1 * time.Minute)
	var msg p2p.Msg
	select {
	case msg = <-op.msg:
		return msg, nil
	case <-timeout:
		return p2p.Msg{}, errors.New("timeout, failed to get the error message")
	}
}

func (op *Operation) Done(msg p2p.Msg) error {
	timeout := time.After(20 * time.Millisecond)
	select {
	case op.msg <- msg:
		return nil
	case <-timeout:
		return errors.New("operation finished but failed to send the message")
	}
}
