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
		if ok == false {
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
	for name, _ := range requirements {
		if isRequirementsAcyclicDfs(name, requirements, visited) == false {
			return false
		}
	}
	return true
}

func isRequirementsAcyclicDfs(root string, requirements map[string][]string, memory map[string]int) bool {
	state, ok := memory[root]
	if ok == true {
		if state == -1 {
			return false
		} else {
			return true
		}
	}
	memory[root] = -1
	for _, name := range requirements[root] {
		if isRequirementsAcyclicDfs(name, requirements, memory) == false {
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
	for name, _ := range orphanedRunning {
		services = append(services, name)
	}
	sort.Slice(services, func(i, j int) bool {
		return string(services[i]) < string(services[j])

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
	for name, _ := range states {
		getEnabledLeafs(name, states, requirements, visited, &results)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i] < results[j]
	})
	return results
}
func getEnabledLeafs(root string, states map[string]State, requirements map[string][]string, visited map[string]bool, results *[]string) bool {
	if p, ok := visited[root]; ok == true {
		return p
	}
	if !isStartedState(states[root]) {
		visited[root] = true
		return true
	}
	// it is started
	for _, requirement := range requirements[root] {
		if getEnabledLeafs(requirement, states, requirements, visited, results) == false {
			visited[root] = false
			return false
		}
	}
	*results = append(*results, root)
	visited[root] = false
	return false
}

/*
func mergeOneList(list []ServiceName, lists [][]ServiceName) []ServiceName {


}
*/
