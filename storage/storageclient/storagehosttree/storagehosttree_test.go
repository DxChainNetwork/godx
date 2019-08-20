// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

type (
	hostInfo struct {
		ip   string
		eval int64
	}

	fakeEvaluator struct {
		evalMap map[enode.ID]int64
	}
)

var (
	hostDataSet = map[enode.ID]hostInfo{
		enode.ID([32]byte{1}): {"99.0.86.9", 1},
		enode.ID([32]byte{2}): {"104.143.92.125", 2},
		enode.ID([32]byte{3}): {"104.237.91.15", 3},
		enode.ID([32]byte{4}): {"185.192.69.89", 4},
		enode.ID([32]byte{5}): {"104.238.46.146", 5},
		enode.ID([32]byte{6}): {"104.238.46.156", 6},
	}

	hostDataSet2 = map[enode.ID]hostInfo{
		enode.ID([32]byte{1}): {"99.0.86.9", 1},
		enode.ID([32]byte{2}): {"104.143.92.125", 1},
		enode.ID([32]byte{3}): {"104.237.91.15", 1},
		enode.ID([32]byte{4}): {"185.192.69.89", 1},
		enode.ID([32]byte{5}): {"104.238.46.146", 1},
		enode.ID([32]byte{6}): {"104.238.46.156", 1},
	}

	hostDataSet3 = map[enode.ID]hostInfo{
		enode.ID([32]byte{1}): {"99.0.86.9", 0},
		enode.ID([32]byte{2}): {"104.143.92.125", 0},
		enode.ID([32]byte{3}): {"104.237.91.15", 0},
		enode.ID([32]byte{4}): {"185.192.69.89", 0},
		enode.ID([32]byte{5}): {"104.238.46.146", 0},
		enode.ID([32]byte{6}): {"104.238.46.156", 1},
	}
)

// newFakeEvaluator returns a new fakeEvaluator with evaluated weight given by ips.
func newFakeEvaluator(dataSet map[enode.ID]hostInfo) *fakeEvaluator {
	evalMap := make(map[enode.ID]int64)
	for id, info := range dataSet {
		evalMap[id] = info.eval
	}
	return &fakeEvaluator{
		evalMap: evalMap,
	}
}

func (fe *fakeEvaluator) Evaluate(info storage.HostInfo) int64 {
	if weight, exist := fe.evalMap[info.EnodeID]; exist {
		return weight
	}
	return int64(0)
}

// totalWeight return the total weight in the eval map
func (fe *fakeEvaluator) totalWeight() int64 {
	res := int64(0)
	for _, weight := range fe.evalMap {
		res += weight
	}
	return res
}

// newTestStorageHostTree returns a new tree with evaluator with some entries already inserted
func newTestStorageHostTree(evaluator Evaluator) (*StorageHostTree, error) {
	tree := New(evaluator)
	for id, info := range hostDataSet {
		hostInfo := createHostInfo(info.ip, id, true)
		if err := tree.Insert(hostInfo); err != nil {
			return nil, err
		}
	}
	return tree, nil
}

func TestStorageHostTree_Insert(t *testing.T) {
	fe := newFakeEvaluator(hostDataSet)
	tree, err := newTestStorageHostTree(fe)
	if err != nil {
		t.Fatalf("error new test tree: %v", err)
	}
	if len(tree.hostPool) != len(ips) {
		t.Errorf("error: the amount of storage host stored in the pool is expected to be %d, instead, got %d",
			len(ips), len(tree.hostPool))
	}

	err = treeValidation(tree.root, fe.totalWeight())
	if err != nil {
		t.Errorf("evaluation verification failed: %s", err.Error())
	}
}

func TestStorageHostTree_HostInfoUpdate(t *testing.T) {
	fe := newFakeEvaluator(hostDataSet)
	tree, err := newTestStorageHostTree(fe)
	if err != nil {
		t.Fatalf("error newNode test tree: %v", err)
	}
	// pick the node to modify. Archive the oldNode entry
	id := enode.ID([32]byte{1})
	ptr, exists := tree.hostPool[id]
	if !exists {
		t.Fatalf("error: host does not exist")
	}
	oldNode := *ptr
	// Update the IP address
	newIP := "104.238.46.129"
	hostInfo := createHostInfo(newIP, id, true)
	err = tree.HostInfoUpdate(hostInfo)
	if err != nil {
		t.Fatalf("error: failed to update the storage host information %s", err.Error())
	}
	newNode := tree.hostPool[id]
	// Check the values between oldNode and newNode
	if oldNode.entry.IP == newNode.entry.IP {
		t.Errorf("error: the ip address should be updated. expected: %s, got %s",
			newIP, oldNode.entry.IP)
	}
	if err = treeValidation(tree.root, fe.totalWeight()); err != nil {
		t.Errorf("evaluation verification failed: %s", err.Error())
	}
}

func TestStorageHostTree_All(t *testing.T) {
	tree, err := newTestStorageHostTree(newFakeEvaluator(hostDataSet))
	if err != nil {
		t.Fatalf("error new test tree: %v", err)
	}
	// Test all function
	storageHosts := tree.All()
	if len(storageHosts) != len(hostDataSet) {
		t.Errorf("insufficient amount of storage hosts, expected %d, got %d",
			len(storageHosts), len(ips))
	}
	// Check whether the host infos are expected
	for _, host := range storageHosts {
		info, exist := tree.hostPool[host.EnodeID]
		if !exist {
			t.Fatalf("host %v not exist", host.EnodeID)
		}
		if info.entry.IP != hostDataSet[host.EnodeID].ip {
			t.Errorf("host %v ip not expected. Got %v, Expect %v", host.EnodeID, info.entry.IP,
				hostDataSet[host.EnodeID].ip)
		}
	}
}

func TestStorageHostTree_Remove(t *testing.T) {
	fe := newFakeEvaluator(hostDataSet)
	tree, err := newTestStorageHostTree(fe)
	if err != nil {
		t.Fatalf("error new test tree: %v", err)
	}

	idToRemove := enode.ID([32]byte{1})
	if err = tree.Remove(idToRemove); err != nil {
		t.Fatalf("error: %s", err.Error())
	}
	if _, exists := tree.hostPool[idToRemove]; exists {
		t.Errorf("failed to remove the node from the tree, the node still exists")
	}
	if err = treeValidation(tree.root, fe.totalWeight()-hostDataSet[idToRemove].eval); err != nil {
		t.Fatalf("After remove, tree not valid: %v", err)
	}
}

func TestStorageHostTree_RetrieveHostInfo(t *testing.T) {
	// Define the constants to be used in this test
	notExistID := enode.ID([32]byte{10})

	fe := newFakeEvaluator(hostDataSet)
	tree, err := newTestStorageHostTree(fe)
	if err != nil {
		t.Fatalf("error new test tree: %v", err)
	}

	if _, exist := tree.RetrieveHostInfo(notExistID); exist {
		t.Errorf("error: the node with \"the key does not exist\" should not exist")
	}

	for id := range hostDataSet {
		if _, exist := tree.RetrieveHostInfo(id); !exist {
			t.Errorf("error: the node with key %s should exist", ips[4])
		}
	}
}

func TestStorageHostTree_SetEvaluationFunc(t *testing.T) {
	fe := newFakeEvaluator(hostDataSet)
	tree, err := newTestStorageHostTree(fe)
	if err != nil {
		t.Fatalf("error new test tree: %v", err)
	}

	fe2 := newFakeEvaluator(hostDataSet2)
	err = tree.SetEvaluator(fe2)
	if err != nil {
		t.Errorf("failed to set new evaluation function")
	}

	err = treeValidation(tree.root, fe2.totalWeight())
	if err != nil {
		t.Errorf("evaluation verification failed: %s", err.Error())
	}
}

// TestStorageHostTree_SelectRandomNumber test the number of host returned by SelectRandom:
// 1. The number to select is smaller than host number, selected host number should be num to select
// 2. The number to select is larger than host number, selected host number should be total number of hosts
func TestStorageHostTree_SelectRandomSize(t *testing.T) {
	tests := []struct {
		numHostsToSelect int
		expectedNum      int
	}{
		{3, 3},
		{10, 6},
	}
	for _, test := range tests {
		fe := newFakeEvaluator(hostDataSet)
		tree, err := newTestStorageHostTree(fe)
		if err != nil {
			t.Fatalf("error new test tree: %v", err)
		}
		infos := tree.SelectRandom(test.numHostsToSelect, nil, nil)
		if len(infos) != test.expectedNum {
			t.Errorf("info size not expected. Got %v, Expect %v", len(infos), test.expectedNum)
		}
	}
}

// TestStorageHostTree_SelectRandomWeight test whether the select random is based on the weight.
// Use an input with one host with weight 1 and rest with weight 0. The host with weight 1 should
// be always selected.
func TestStorageHostTree_SelectRandomWeight(t *testing.T) {
	data := hostDataSet3
	// Check whether the input dataset has the expected property
	var selectedID enode.ID
	var selectedInfo hostInfo
	for id, info := range data {
		if info.eval != 0 {
			emptyHostInfo := hostInfo{}
			if selectedID != enode.ID([32]byte{}) || selectedInfo != emptyHostInfo {
				t.Fatal("invalid input data")
			}
			selectedID = id
			selectedInfo = info
		}
	}
	fe := newFakeEvaluator(hostDataSet3)
	tree, err := newTestStorageHostTree(fe)
	if err != nil {
		t.Fatalf("error new test tree: %v", err)
	}
	for i := 0; i != 10; i++ {
		infos := tree.SelectRandom(1, nil, nil)
		if len(infos) != 1 {
			t.Fatalf("unexpected selected host info size. Expect %v, Got %v", 1, len(infos))
		}
		if infos[0].EnodeID != selectedID {
			t.Errorf("Unexpected node to be selected. Expect %v, Got %v", infos[0].EnodeID, selectedID)
		}
	}
}

func createHostInfo(ip string, id enode.ID, accept bool) storage.HostInfo {
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: accept,
		},
		IP:      ip,
		EnodeID: id,
		ScanRecords: storage.HostPoolScans{storage.HostPoolScan{
			Timestamp: time.Now(),
			Success:   true,
		}},
	}
}

// treeValidation validates the tree given the root node. If not valid, return an error.
//   1. Check whether the data structure is consistent
//   2. Check whether the root has expected total
func treeValidation(root *node, expectedRootTotal int64) error {
	if err := treeConsistenceValidation(root); err != nil {
		return err
	}
	if root.evalTotal != expectedRootTotal {
		return fmt.Errorf("root total not expected. Got %v, Expect %v", root.evalTotal, expectedRootTotal)
	}
	return nil
}

// treeConsistenceValidation checks whether the tree is consistence in weight.
func treeConsistenceValidation(n *node) error {
	if n.left == nil {
		return nil
	}
	err := compareEval(n)
	if err != nil {
		return err
	}
	if n.left != nil {
		err := treeConsistenceValidation(n.left)
		if err != nil {
			return err
		}
	}
	if n.right != nil {
		err := treeConsistenceValidation(n.right)
		if err != nil {
			return err
		}
	}
	return nil
}

func compareEval(n *node) error {
	org := n.entry.eval
	if n.left != nil && n.right != nil {
		sum := n.left.evalTotal + n.right.evalTotal
		sum = org + sum
		if n.evalTotal != sum {
			return errors.New("error: parent evaluation should be sum of the children's evaluation")
		}
	} else if n.right == nil {
		sum := org + n.left.evalTotal
		if n.evalTotal != sum {
			return errors.New("error: parent evaluation should be sum of the children's evaluation")
		}
	}

	return nil
}
