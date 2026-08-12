package callstack

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dghubble/trie"
	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
)

func referencePathKey(nodes []types.CallStackNode) string {
	parts := make([]string, len(nodes))
	for i, node := range nodes {
		parts[i] = node.Frame.FileID + ":" + node.Frame.Lineno
	}
	return strings.Join(parts, "/")
}

func referenceGetCallStackPaths(cs types.CallStack) [][]types.CallStackNode {
	var paths [][]types.CallStackNode
	var traverse func(node types.CallStackNode, currentPath []types.CallStackNode)

	traverse = func(node types.CallStackNode, currentPath []types.CallStackNode) {
		path := append([]types.CallStackNode{}, currentPath...)
		path = append(path, node)

		if len(node.Children) == 0 {
			paths = append(paths, path)
			return
		}
		for _, child := range node.Children {
			traverse(child, path)
		}
	}

	if isEmptyFrame(cs.Root.Frame) {
		for _, child := range cs.Root.Children {
			traverse(child, nil)
		}
	} else {
		traverse(cs.Root, nil)
	}

	return paths
}

func randomCallStack(rng *rand.Rand, depth, maxWidth int, emptyRoot bool) types.CallStack {
	var build func(d int) types.CallStackNode
	build = func(d int) types.CallStackNode {
		n := types.CallStackNode{Frame: types.StackFrame{
			FileID: fmt.Sprintf("f%d", rng.Intn(5)),
			Lineno: fmt.Sprintf("%d", rng.Intn(50)),
		}}
		if d <= 0 {
			return n
		}
		for i := 0; i < rng.Intn(maxWidth+1); i++ {
			n.Children = append(n.Children, build(d-1))
		}
		return n
	}
	root := build(depth)
	if emptyRoot {
		root.Frame = types.StackFrame{}
	}
	return types.CallStack{Root: root}
}

func TestGetCallStackPathsMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 300; i++ {
		cs := randomCallStack(rng, 1+rng.Intn(5), 3, i%3 == 0)
		got, want := getCallStackPaths(cs), referenceGetCallStackPaths(cs)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: paths differ\ngot  %#v\nwant %#v", i, got, want)
		}
		for j := range want {
			if pathKey(want[j]) != referencePathKey(want[j]) {
				t.Fatalf("case %d path %d: key %q != reference %q", i, j, pathKey(want[j]), referencePathKey(want[j]))
			}
		}
	}
}

func referenceUnify(stacks []types.IdentifiedCallStack) []types.IdentifiedCallStack {
	stacksByID := make(map[types.CallID][]types.CallStack)
	for _, stack := range stacks {
		stacksByID[stack.CallID] = append(stacksByID[stack.CallID], stack.CallStack)
	}
	var result []types.IdentifiedCallStack
	for id, groupStacks := range stacksByID {
		if len(groupStacks) == 0 {
			continue
		}
		tr := trie.NewPathTrie()
		for _, cs := range groupStacks {
			for _, path := range referenceGetCallStackPaths(cs) {
				tr.Put(referencePathKey(path), path)
			}
		}
		result = append(result, types.IdentifiedCallStack{CallID: id, CallStack: reconstructCallStack(tr)})
	}
	return result
}

func byCallID(in []types.IdentifiedCallStack) []types.IdentifiedCallStack {
	out := append([]types.IdentifiedCallStack{}, in...)
	sort.Slice(out, func(i, j int) bool { return out[i].CallID < out[j].CallID })
	return out
}

func TestUnifyIdentifiedCallStacksMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 200; i++ {
		var stacks []types.IdentifiedCallStack
		for s := 0; s < 1+rng.Intn(4); s++ {
			stacks = append(stacks, types.IdentifiedCallStack{
				CallID:    types.CallID(fmt.Sprintf("id%d", rng.Intn(3))),
				CallStack: randomCallStack(rng, 1+rng.Intn(4), 3, i%4 == 0),
			})
		}
		got := byCallID(UnifyIdentifiedCallStacks(stacks))
		want := byCallID(referenceUnify(stacks))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: unified output differs from reference implementation", i)
		}
	}
}
