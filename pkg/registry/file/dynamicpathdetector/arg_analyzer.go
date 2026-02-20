package dynamicpathdetector

// ArgAnalyzer is a trie-based analyzer for exec argument vectors.
// Each exec binary (Path) gets its own trie root. Each argument position
// is a trie level. When unique values at a position exceed the threshold,
// that position collapses to DynamicIdentifier (⋯).
type ArgAnalyzer struct {
	roots     map[string]*ArgNode
	threshold int
}

type ArgNode struct {
	Children map[string]*ArgNode
}

func NewArgAnalyzer(threshold int) *ArgAnalyzer {
	return &ArgAnalyzer{
		roots:     make(map[string]*ArgNode),
		threshold: threshold,
	}
}

// AddArgs inserts an argument vector into the trie for the given exec path.
func (a *ArgAnalyzer) AddArgs(args []string, execPath string) {
	if len(args) == 0 {
		return
	}
	root, ok := a.roots[execPath]
	if !ok {
		root = &ArgNode{Children: make(map[string]*ArgNode)}
		a.roots[execPath] = root
	}
	node := root
	for _, arg := range args {
		if node.Children == nil {
			node.Children = make(map[string]*ArgNode)
		}
		child, ok := node.Children[arg]
		if !ok {
			child = &ArgNode{Children: make(map[string]*ArgNode)}
			node.Children[arg] = child
		}
		node = child
	}
}

// AnalyzeArgs returns the collapsed argument vector for the given exec path.
// Positions where unique values exceed the threshold are replaced with DynamicIdentifier.
func (a *ArgAnalyzer) AnalyzeArgs(args []string, execPath string) []string {
	if len(args) == 0 {
		return args
	}
	root, ok := a.roots[execPath]
	if !ok {
		return args
	}
	result := make([]string, len(args))
	node := root
	for i, arg := range args {
		if node == nil || node.Children == nil {
			result[i] = arg
			continue
		}
		if len(node.Children) > a.threshold {
			result[i] = DynamicIdentifier
			// Follow the dynamic path if it exists, otherwise try the exact child
			if dynChild, ok := node.Children[DynamicIdentifier]; ok {
				node = dynChild
			} else if child, ok := node.Children[arg]; ok {
				node = child
			} else {
				node = nil
			}
		} else {
			result[i] = arg
			if child, ok := node.Children[arg]; ok {
				node = child
			} else {
				node = nil
			}
		}
	}
	return result
}
