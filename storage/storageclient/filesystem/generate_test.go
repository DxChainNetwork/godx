// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"fmt"
	"testing"
)

func TestDirTree_RandomPath(t *testing.T) {
	numFiles, maxDepth := 100, 10
	tests := []struct {
		goDeepRate float32
		goWideRate float32
		checkFunc  func(dt dirTree) error
	}{
		{0, 0.5, checkAllFilesWithExpectDepth(0)},
		{1, 0.5, checkAllFilesWithExpectDepth(10)},
		{0.7, 0, checkGoWideRateZero},
		{0.5, 1, checkGoWideRateOne},
	}
	for i, test := range tests {
		dt := newDirTree(test.goDeepRate, test.goWideRate, maxDepth)
		for i := 0; i != numFiles; i++ {
			_, err := dt.randomPath()
			if err != nil {
				t.Fatal(err)
			}
			//fmt.Println(path.Path)
		}
		if err := test.checkFunc(dt); err != nil {
			t.Errorf("test %d: %v", i, err)
		}
	}
}

func checkAllFilesWithExpectDepth(expectDepth int) func(dt dirTree) error {
	return func(dt dirTree) error {
		depth, curNode := 0, dt.root
		return recursiveCheckForDepth(curNode, depth, expectDepth)
	}
}

func recursiveCheckForDepth(node *dirTreeNode, depth, expectDepth int) error {
	if depth == expectDepth {
		// reach the expect depth. Should have no dirs, and plenty of files
		if len(node.subDirs) != 0 {
			return fmt.Errorf("node have plenty subdirs at expect depth %v: %v", depth, node.dxPath.Path)
		}
		if len(node.files) == 0 {
			return fmt.Errorf("node have no files at expect depth %v: %v", depth, node.dxPath.Path)
		}
		return nil
	}
	// node should have no files
	if len(node.files) != 0 {
		return fmt.Errorf("node have files at depth %v: %v", depth, node.dxPath.Path)
	}
	// loop over subdirs to check for error
	for _, subDir := range node.subDirs {
		if err := recursiveCheckForDepth(subDir, depth+1, expectDepth); err != nil {
			return err
		}
	}
	return nil
}

// checkAllfilesSameDirectory checks the condition for goWideRate == 0
func checkGoWideRateZero(dt dirTree) error {
	return recursiveCheckGoWideRateZero(dt.root)
}

func recursiveCheckGoWideRateZero(node *dirTreeNode) error {
	if len(node.subDirs) > 1 {
		return fmt.Errorf("node have more than 1 directory entry: %v", node.dxPath.Path)
	}
	if len(node.subDirs) == 0 && len(node.files) == 0 {
		return fmt.Errorf("at branch end, got %v files", len(node.files))
	} else if len(node.subDirs) == 0 {
		return nil
	}
	var subDir *dirTreeNode
	for _, subDir = range node.subDirs {
		break
	}
	return recursiveCheckGoWideRateZero(subDir)
}

// checkGoWideRateOne checks the condition for goWideRate == 1
func checkGoWideRateOne(dt dirTree) error {
	for _, dir := range dt.root.subDirs {
		if err := recursiveCheckGoWideRateOne(dir); err != nil {
			return err
		}
	}
	return nil
}

func recursiveCheckGoWideRateOne(node *dirTreeNode) error {
	if len(node.subDirs) > 1 {
		return fmt.Errorf("node have more than 1 directory entry: %v", node.dxPath.Path)
	}
	if len(node.subDirs) == 0 && len(node.files) != 1 {
		return fmt.Errorf("at branch end, have %v files", len(node.files))
	}
	if len(node.subDirs) == 1 && len(node.files) != 0 {
		return fmt.Errorf("not at branch end, have %v files", len(node.files))
	}
	if len(node.subDirs) == 0 {
		return nil
	}
	var subDir *dirTreeNode
	for _, subDir = range node.subDirs {
		break
	}
	return recursiveCheckGoWideRateOne(subDir)
}
