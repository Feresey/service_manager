package main

import (
	"sort"
)

func InitOrder(init ServiceName, requirements map[ServiceName][]ServiceName) []ServiceName {
	order := []ServiceName{}
	dfsPostOrder(init, requirements, &order)
	return deduplicateOrder(order)
}

func dfsPostOrder(root ServiceName, requirements map[ServiceName][]ServiceName, order *[]ServiceName) {
	for _, parent := range requirements[root] {
		dfsPostOrder(parent, requirements, order)
	}
	*order = append(*order, root)
}

func deduplicateOrder(order []ServiceName) []ServiceName {
	founded := map[ServiceName]struct{}{}
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
func IsRequirementsAcyclic(requirements map[ServiceName][]ServiceName) bool {
	visited := map[ServiceName]int{}
	for name, _ := range requirements {
		if isRequirementsAcyclicDfs(name, requirements, visited) == false {
			return false
		}
	}
	return true
}

func isRequirementsAcyclicDfs(root ServiceName, requirements map[ServiceName][]ServiceName, memory map[ServiceName]int) bool {
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

func GetOrphanedStartedServices(states map[ServiceName]State, requirements map[ServiceName][]ServiceName) []ServiceName {
	orphanedRunning := map[ServiceName]struct{}{}
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
	services := make([]ServiceName, 0, len(orphanedRunning))
	for name, _ := range orphanedRunning {
		services = append(services, name)
	}
	sort.Slice(services, func(i, j int) bool {
		return string(services[i]) < string(services[j])

	})
	return services
}

/*
func mergeOneList(list []ServiceName, lists [][]ServiceName) []ServiceName {


}
*/
