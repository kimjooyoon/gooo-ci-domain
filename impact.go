package cibudgetdomain

import "fmt"

func validateGraph(graph Graph) ([][]int, error) {
	if graph.SourceDigest == "" || len(graph.Units) == 0 || len(graph.Units) > 4096 {
		return nil, fmt.Errorf("graph requires a source digest and 1..4096 units")
	}
	reverse, ids := make([][]int, len(graph.Units)), map[string]bool{}
	edges := 0
	for index, unit := range graph.Units {
		if unit.ID == "" || ids[unit.ID] || unit.Kind != "source" && unit.Kind != "build" && unit.Kind != "test" {
			return nil, fmt.Errorf("unit identity or kind differs")
		}
		ids[unit.ID] = true
		seen := map[int]bool{}
		for _, dependency := range unit.Dependencies {
			if dependency < 0 || dependency >= len(graph.Units) || dependency == index || seen[dependency] {
				return nil, fmt.Errorf("invalid or repeated dependency")
			}
			seen[dependency] = true
			reverse[dependency] = append(reverse[dependency], index)
			edges++
		}
	}
	if edges > 65536 {
		return nil, fmt.Errorf("dependency population exceeds 65536")
	}
	return reverse, nil
}

func impactClosure(graph Graph, reverse [][]int) ([]bool, []int, error) {
	if len(graph.Changed) == 0 {
		return nil, nil, fmt.Errorf("changed source units are missing")
	}
	affected, queue := make([]bool, len(graph.Units)), append([]int(nil), graph.Changed...)
	seeds := map[int]bool{}
	for _, index := range graph.Changed {
		if seeds[index] {
			return nil, nil, fmt.Errorf("changed source unit is repeated")
		}
		seeds[index] = true
	}
	for offset := 0; offset < len(queue); offset++ {
		index := queue[offset]
		if index < 0 || index >= len(affected) {
			return nil, nil, fmt.Errorf("changed source unit is absent")
		}
		if affected[index] {
			continue
		}
		affected[index] = true
		queue = append(queue, reverse[index]...)
	}
	tests := []int{}
	for index, unit := range graph.Units {
		if affected[index] && unit.Kind == "test" {
			tests = append(tests, index)
		}
	}
	return affected, tests, nil
}

func prerequisiteClosure(graph Graph, tests []int) []bool {
	required, queue := make([]bool, len(graph.Units)), append([]int(nil), tests...)
	for offset := 0; offset < len(queue); offset++ {
		index := queue[offset]
		if required[index] {
			continue
		}
		required[index] = true
		queue = append(queue, graph.Units[index].Dependencies...)
	}
	return required
}
