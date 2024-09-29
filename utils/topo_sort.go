package utils

import (
	"errors"
	"fmt"
)

type Graph[Key comparable] map[Key][]Key

var ErrCyclicDependecies = errors.New("cyclic dependencies")

func StableTopologicalSortWithSortedKeys[Key comparable](graph Graph[Key], sortedKeys []Key) ([]Key, error) {
	if len(sortedKeys) != len(graph) {
		return nil, fmt.Errorf("wrong sorted keys info: len(sortedKeys) != len(graph): %v != %v", len(sortedKeys), len(graph))
	}

	inDegree := make(map[Key]int)

	for _, key := range sortedKeys {
		dependencies := graph[key]

		for _, dependency := range dependencies {
			inDegree[dependency]++
		}
	}

	queue := make([]Key, 0)
	for _, node := range sortedKeys {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]Key, 0)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(graph) {
		return nil, fmt.Errorf("%w: %v, %v", ErrCyclicDependecies, result, graph)
	}

	return result, nil
}
