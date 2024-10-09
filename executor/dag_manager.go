// executor/dag_manager.go

package executor

type DAGManager interface {
	AddNode(name string, dependencies []string)
	TopologicalSort() ([]string, error)
}

type dagManager struct {
	graph map[string][]string
}

func NewDAGManager() DAGManager {
	return &dagManager{
		graph: make(map[string][]string),
	}
}

func (dm *dagManager) AddNode(name string, dependencies []string) {
	dm.graph[name] = dependencies
}

func (dm *dagManager) TopologicalSort() ([]string, error) {
	visited := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		visited[name] = true

		for _, dep := range dm.graph[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		order = append(order, name)
		return nil
	}

	for name := range dm.graph {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	// Reverse the order
	for i := 0; i < len(order)/2; i++ {
		j := len(order) - 1 - i
		order[i], order[j] = order[j], order[i]
	}

	return order, nil
}
