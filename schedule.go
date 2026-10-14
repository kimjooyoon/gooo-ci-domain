package cibudgetdomain

import (
	"fmt"
	"math"
)

func schedule(graph Graph, required []bool, costs []int64, workers int) ([]Slot, int64, error) {
	if workers < 1 || workers > 64 {
		return nil, 0, fmt.Errorf("worker population requires 1..64")
	}
	done, ends, available := make([]bool, len(required)), make([]int64, len(required)), make([]int64, workers)
	remaining := 0
	for index, selected := range required {
		if !selected {
			done[index] = true
		} else {
			remaining++
		}
	}
	slots := []Slot{}
	for remaining > 0 {
		index, start := nextReadyUnit(graph, required, done, ends)
		if index < 0 {
			return nil, 0, fmt.Errorf("required dependency cycle is unresolved")
		}
		worker := earliestWorker(available)
		if graph.Units[index].Kind != "source" {
			start = max(start, available[worker])
			if costs[index] > math.MaxInt64-start {
				return nil, 0, fmt.Errorf("schedule exceeds integer representation")
			}
			available[worker] = start + costs[index]
			slots = append(slots, Slot{index, worker, start, available[worker]})
		}
		ends[index], done[index] = start+costs[index], true
		remaining--
	}
	wall := int64(0)
	for _, end := range available {
		wall = max(wall, end)
	}
	return slots, wall, nil
}

func nextReadyUnit(graph Graph, required, done []bool, ends []int64) (int, int64) {
	for index, selected := range required {
		if !selected || done[index] {
			continue
		}
		start, ready := int64(0), true
		for _, dependency := range graph.Units[index].Dependencies {
			ready = ready && done[dependency]
			start = max(start, ends[dependency])
		}
		if ready {
			return index, start
		}
	}
	return -1, 0
}

func earliestWorker(available []int64) int {
	selected := 0
	for index, at := range available {
		if at < available[selected] {
			selected = index
		}
	}
	return selected
}
