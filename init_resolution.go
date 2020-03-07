package main

import (
	"sort"
)

func InitOrder(init string, requirements map[string][]string) []string {
	order := []string{}
	dfsPostOrder(init, requirements, &order)

	return deduplicateOrder(order)
}

func dfsPostOrder(root string, requirements map[string][]string, order *[]string) {
	for _, parent := range requirements[root] {
		dfsPostOrder(parent, requirements, order)
	}

	*order = append(*order, root)
}

func deduplicateOrder(order []string) []string {
	founded := map[string]struct{}{}
	o := order[:0]

	for _, e := range order {
		_, ok := founded[e]
		if !ok {
			o = append(o, e)
			founded[e] = struct{}{}
		}
	}

	return o
}

// -1 we in
//  1 visited
func IsRequirementsAcyclic(requirements map[string][]string) bool {
	visited := map[string]int{}

	for name := range requirements {
		if !isRequirementsAcyclicDfs(name, requirements, visited) {
			return false
		}
	}

	return true
}

func isRequirementsAcyclicDfs(root string, requirements map[string][]string, memory map[string]int) bool {
	state, ok := memory[root]
	if ok {
		return state != -1
	}

	memory[root] = -1

	for _, name := range requirements[root] {
		if !isRequirementsAcyclicDfs(name, requirements, memory) {
			return false
		}
	}

	memory[root] = 1

	return true
}

func GetOrphanedStartedServices(states map[string]State, requirements map[string][]string) []string {
	orphanedRunning := map[string]struct{}{}

	for name, state := range states {
		if isStartedState(state) {
			orphanedRunning[name] = struct{}{}
		}
	}

	for name, state := range states {
		if !isStartedState(state) {
			continue
		}

		for _, parent := range requirements[name] {
			delete(orphanedRunning, parent)
		}
	}

	services := make([]string, 0, len(orphanedRunning))

	for name := range orphanedRunning {
		services = append(services, name)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i] < services[j]
	})

	return services
}

func GetEnabledLeafsFromRoot(root string, states map[string]State, requirements map[string][]string) []string {
	results := make([]string, 0)
	visited := make(map[string]bool)
	getEnabledLeafs(root, states, requirements, visited, &results)

	sort.Slice(results, func(i, j int) bool {
		return results[i] < results[j]
	})

	return results
}
func GetEnabledLeafs(states map[string]State, requirements map[string][]string) []string {
	results := make([]string, 0)
	visited := make(map[string]bool)

	for name := range states {
		getEnabledLeafs(name, states, requirements, visited, &results)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i] < results[j]
	})

	return results
}
func getEnabledLeafs(root string,
	states map[string]State,
	requirements map[string][]string,
	visited map[string]bool,
	results *[]string) bool {
	if p, ok := visited[root]; ok {
		return p
	}
	//if !isStartedState(states[root]) {
	if states[root] == StateDead {
		visited[root] = true
		return true
	}
	// it is started
	for _, requirement := range requirements[root] {
		if !getEnabledLeafs(requirement, states, requirements, visited, results) {
			visited[root] = false
			return false
		}
	}

	*results = append(*results, root)
	visited[root] = false

	return false
}

func GetDisabledLeafsFromRoot(root string, states map[string]State, requirements map[string][]string) []string {
	results := make([]string, 0)
	getDisabledLeafsFromRoot(root, states, requirements, &results)
	sort.Slice(results, func(i, j int) bool {
		return results[i] < results[j]
	})

	return results
}

//   A
//  /|
// F B
//  /|\
// C D E
func getDisabledLeafsFromRoot(
	root string,
	states map[string]State,
	requirements map[string][]string,
	results *[]string,
) bool {
	if states[root] == StateRunning {
		return true
	}

	isLeafsEnabled := true

	for _, requirement := range requirements[root] {
		if !getDisabledLeafsFromRoot(requirement, states, requirements, results) {
			isLeafsEnabled = false
		}
	}

	if isLeafsEnabled {
		*results = append(*results, root)
	}

	return false
}
