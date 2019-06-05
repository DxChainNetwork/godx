package storagehosttree

import (
	"errors"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

var evalFunc = func(storage.HostInfo) HostEvaluation {
	return randomCriteria()
}

var evalFunc2 = func(info storage.HostInfo) HostEvaluation {
	return partialRandomCriteria()
}

var Samplescans = storage.HostPoolScans{
	storage.HostPoolScan{
		Timestamp: time.Now(),
		Success:   false,
	},
}

var removeNodeIndex = 1

var tree = New(evalFunc)

func TestStorageHostTree_Insert(t *testing.T) {
	EnodeIDGenerater()
	for i, ip := range ips {
		hostinfo := createHostInfo(ip, EnodeID[i], Samplescans, true)
		err := tree.Insert(hostinfo)
		if err != nil {
			t.Errorf("error inserting %s. pubkey: %s", err.Error(), hostinfo.EnodeID)
		}
	}

	if len(tree.hostPool) != len(ips) {
		t.Errorf("error: the amount of storage host stored in the pool is expected to be %d, instead, got %d",
			len(ips), len(tree.hostPool))
	}

	err := evalVerification(tree.root)
	if err != nil {
		t.Errorf("evaluation verification failed: %s", err.Error())
	}

}

func TestStorageHostTree_HostInfoUpdate(t *testing.T) {
	ptr, exists := tree.hostPool[EnodeID[3]]
	if !exists {
		t.Fatalf("error: host does not exist")
	}
	archive := *ptr

	hostinfo := createHostInfo("104.238.46.129", EnodeID[3], Samplescans, true)
	err := tree.HostInfoUpdate(hostinfo)
	if err != nil {
		t.Fatalf("error: failed to update the storage host information %s", err.Error())
	}

	new := tree.hostPool[EnodeID[3]]
	if archive.entry.IP == new.entry.IP {
		t.Errorf("error: the ip address should be updated. expected: 104.238.46.129, got %s",
			archive.entry.IP)
	}

	ips[3] = "104.238.46.129"
}

func TestStorageHostTree_All(t *testing.T) {
	storageHosts := tree.All()
	if len(storageHosts) != len(ips) {
		t.Errorf("insufficient amount of storage hosts, expected %d, got %d",
			len(storageHosts), len(ips))
	}

	for i := 0; i < len(storageHosts)-1; i++ {
		eval1 := tree.hostPool[storageHosts[i].EnodeID].entry.eval
		eval2 := tree.hostPool[storageHosts[i+1].EnodeID].entry.eval
		if eval1.Cmp(eval2) < 0 {
			t.Errorf("the returned storage hosts should be in order, the host has higher evaluation should be in the front")
		}
	}
}

func TestStorageHostTree_Remove(t *testing.T) {
	err := tree.Remove(EnodeID[removeNodeIndex])
	if err != nil {
		t.Fatalf("error: %s", err.Error())
	}
	if _, exists := tree.hostPool[EnodeID[removeNodeIndex]]; exists {
		t.Errorf("failed to remove the node from the tree, the node still exists")
	}
}

func TestStorageHostTree_RetrieveHostInfo(t *testing.T) {
	if _, exist := tree.RetrieveHostInfo([32]byte{}); exist {
		t.Errorf("error: the node with \"the key does not exist\" should not exist")
	}

	if _, exist := tree.RetrieveHostInfo(EnodeID[4]); !exist {
		t.Errorf("error: the node with key %s should exist", ips[4])
	}
}

func TestStorageHostTree_SetEvaluationFunc(t *testing.T) {
	err := tree.SetEvaluationFunc(evalFunc2)
	if err != nil {
		t.Errorf("failed to set new evaluation function")
	}

	err = evalVerification(tree.root)
	if err != nil {
		t.Errorf("evaluation verification failed: %s", err.Error())
	}
}

func TestStorageHostTree_SelectRandom(t *testing.T) {
	infos := tree.SelectRandom(10, nil, nil)
	if len(infos) != 0 {
		t.Errorf("the returned host information should be none, because scans all failed")
	}
}

func createHostInfo(ip string, id enode.ID, scans storage.HostPoolScans, contract bool) storage.HostInfo {
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: contract,
		},

		IP:          ip,
		EnodeID:     id,
		ScanRecords: scans,
	}
}

func evalVerification(n *node) error {
	if n.left == nil {
		return nil
	}

	err := compareEval(n)
	if err != nil {
		return err
	}

	if n.left != nil {
		err := evalVerification(n.left)
		if err != nil {
			return err
		}
	}

	if n.right != nil {
		err := evalVerification(n.right)
		if err != nil {
			return err
		}
	}

	return nil
}

func compareEval(n *node) error {
	org := n.entry.eval
	if n.left != nil && n.right != nil {
		sum := n.left.evalTotal.Add(n.right.evalTotal)
		sum = org.Add(sum)
		if n.evalTotal.Cmp(sum) != 0 {
			return errors.New("error: parent evaluation should be sum of the children's evaluation")
		}
	} else if n.right == nil {
		sum := org.Add(n.left.evalTotal)
		if n.evalTotal.Cmp(sum) != 0 {
			return errors.New("error: parent evaluation should be sum of the children's evaluation")
		}
	}

	return nil
}
