package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicateOrder(t *testing.T) {
	testCases := map[string]struct {
		input    []ServiceName
		expected []ServiceName
	}{
		"empty": {
			input:    []ServiceName{},
			expected: []ServiceName{},
		},
		"no dup one elem": {
			input:    []ServiceName{"a"},
			expected: []ServiceName{"a"},
		},
		"no dup": {
			input:    []ServiceName{"a", "b"},
			expected: []ServiceName{"a", "b"},
		},
		"all dup": {
			input:    []ServiceName{"a", "a", "a"},
			expected: []ServiceName{"a"},
		},
		"big": {
			input:    []ServiceName{"a", "b", "a", "c", "c", "a", "b", "e", "a"},
			expected: []ServiceName{"a", "b", "c", "e"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			input := append([]ServiceName{}, tc.input...)
			input = deduplicateOrder(input)
			assert.Equal(t, tc.expected, input)

		})
	}

}

func TestInitOrder(t *testing.T) {
	testCases := map[string]struct {
		init         ServiceName
		requirements map[ServiceName][]ServiceName
		expected     []ServiceName
	}{
		"one parent": {
			init: "s",
			requirements: map[ServiceName][]ServiceName{
				"s": []ServiceName{
					"p",
				},
			},
			expected: []ServiceName{"p", "s"},
		},
		"two parents": {
			init: "s",
			requirements: map[ServiceName][]ServiceName{
				"s": []ServiceName{
					"a", "b",
				},
			},
			expected: []ServiceName{"a", "b", "s"},
		},
		"diamond": {
			init: "s",
			requirements: map[ServiceName][]ServiceName{
				"s": []ServiceName{
					"a", "b",
				},
				"a": []ServiceName{
					"c",
				},
				"b": []ServiceName{
					"c",
				},
			},
			expected: []ServiceName{"c", "a", "b", "s"},
		},
		"no local parenty": {
			init: "s",
			requirements: map[ServiceName][]ServiceName{
				"s": []ServiceName{
					"a", "b",
				},
				"b": []ServiceName{
					"a",
				},
			},
			expected: []ServiceName{"a", "b", "s"},
		},
		"big": {
			init: "s",
			requirements: map[ServiceName][]ServiceName{
				"s": []ServiceName{
					"a", "b", "c",
				},
				"b": []ServiceName{
					"a", "d",
				},
				"c": []ServiceName{
					"d",
				},
			},
			expected: []ServiceName{"a", "d", "b", "c", "s"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := InitOrder(tc.init, tc.requirements)
			assert.Equal(t, tc.expected, result)

		})
	}
}
func TestAcyclic(t *testing.T) {
	testCases := map[string]struct {
		requirements map[ServiceName][]ServiceName
		expected     bool
	}{
		"acyclic simple": {
			requirements: map[ServiceName][]ServiceName{
				"a": []ServiceName{
					"b",
				},
			},
			expected: true,
		},
		"cyclic simple": {
			requirements: map[ServiceName][]ServiceName{
				"a": []ServiceName{
					"a",
				},
			},
			expected: false,
		},
		"acyclic": {
			requirements: map[ServiceName][]ServiceName{
				"a": []ServiceName{
					"b", "c",
				},
				"b": []ServiceName{
					"c",
				},
				"c": []ServiceName{
					"d",
				},
			},
			expected: true,
		},
		"cyclic": {
			requirements: map[ServiceName][]ServiceName{
				"a": []ServiceName{
					"b", "c",
				},
				"b": []ServiceName{
					"c",
				},
				"c": []ServiceName{
					"a",
				},
			},
			expected: false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := IsRequirementsAcyclic(tc.requirements)
			assert.Equal(t, tc.expected, result)

		})
	}
}

func TestGetOrphanedStartedServices(t *testing.T) {
	testCases := map[string]struct {
		requirements map[ServiceName][]ServiceName
		states       map[ServiceName]State
		expected     []ServiceName
	}{
		"with target that has no requirements": {
			requirements: map[ServiceName][]ServiceName{
				"a": []ServiceName{
					"b", "c",
				},
			},
			states: map[ServiceName]State{
				"a": StateRunning,
				"b": StateRunning,
				"d": StateRunning,
			},
			expected: []ServiceName{"a", "d"},
		},
		"with target that has requirements but disabled": {
			requirements: map[ServiceName][]ServiceName{
				"a": []ServiceName{
					"b", "c",
				},
			},
			states: map[ServiceName]State{
				"c": StateStarted,
				"b": StateRunning,
				"d": StateRunning,
			},
			expected: []ServiceName{"b", "c", "d"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := GetOrphanedStartedServices(tc.states, tc.requirements)
			assert.Equal(t, tc.expected, result)

		})
	}
}
