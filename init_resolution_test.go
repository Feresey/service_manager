package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicateOrder(t *testing.T) {
	testCases := map[string]struct {
		input    []string
		expected []string
	}{
		"empty": {
			input:    []string{},
			expected: []string{},
		},
		"no dup one elem": {
			input:    []string{"a"},
			expected: []string{"a"},
		},
		"no dup": {
			input:    []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		"all dup": {
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
		"big": {
			input:    []string{"a", "b", "a", "c", "c", "a", "b", "e", "a"},
			expected: []string{"a", "b", "c", "e"},
		},
	}
	for name := range testCases {
		tc := testCases[name]

		t.Run(name, func(t *testing.T) {
			input := deduplicateOrder(tc.input)
			assert.Equal(t, tc.expected, input)
		})
	}
}

func TestInitOrder(t *testing.T) {
	testCases := map[string]struct {
		init         string
		requirements map[string][]string
		expected     []string
	}{
		"one parent": {
			init: "s",
			requirements: map[string][]string{
				"s": {
					"p",
				},
			},
			expected: []string{"p", "s"},
		},
		"two parents": {
			init: "s",
			requirements: map[string][]string{
				"s": {
					"a", "b",
				},
			},
			expected: []string{"a", "b", "s"},
		},
		"diamond": {
			init: "s",
			requirements: map[string][]string{
				"s": {
					"a", "b",
				},
				"a": {
					"c",
				},
				"b": {
					"c",
				},
			},
			expected: []string{"c", "a", "b", "s"},
		},
		"no local parenty": {
			init: "s",
			requirements: map[string][]string{
				"s": {
					"a", "b",
				},
				"b": {
					"a",
				},
			},
			expected: []string{"a", "b", "s"},
		},
		"big": {
			init: "s",
			requirements: map[string][]string{
				"s": {
					"a", "b", "c",
				},
				"b": {
					"a", "d",
				},
				"c": {
					"d",
				},
			},
			expected: []string{"a", "d", "b", "c", "s"},
		},
	}
	for name := range testCases {
		tc := testCases[name]

		t.Run(name, func(t *testing.T) {
			result := InitOrder(tc.init, tc.requirements)
			assert.Equal(t, tc.expected, result)
		})
	}
}
func TestAcyclic(t *testing.T) {
	testCases := map[string]struct {
		requirements map[string][]string
		expected     bool
	}{
		"acyclic simple": {
			requirements: map[string][]string{
				"a": {
					"b",
				},
			},
			expected: true,
		},
		"cyclic simple": {
			requirements: map[string][]string{
				"a": {
					"a",
				},
			},
			expected: false,
		},
		"acyclic": {
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
				"b": {
					"c",
				},
				"c": {
					"d",
				},
			},
			expected: true,
		},
		"cyclic": {
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
				"b": {
					"c",
				},
				"c": {
					"a",
				},
			},
			expected: false,
		},
	}
	for name := range testCases {
		tc := testCases[name]

		t.Run(name, func(t *testing.T) {
			result := IsRequirementsAcyclic(tc.requirements)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetOrphanedStartedServices(t *testing.T) {
	testCases := map[string]struct {
		requirements map[string][]string
		states       map[string]State
		expected     []string
	}{
		"with target that has no requirements": {
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
			},
			states: map[string]State{
				"a": StateRunning,
				"b": StateRunning,
				"d": StateRunning,
			},
			expected: []string{"a", "d"},
		},
		"with target that has requirements but disabled": {
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
			},
			states: map[string]State{
				"c": StateStarted,
				"b": StateRunning,
				"d": StateRunning,
			},
			expected: []string{"b", "c", "d"},
		},
	}
	for name := range testCases {
		tc := testCases[name]

		t.Run(name, func(t *testing.T) {
			result := GetOrphanedStartedServices(tc.states, tc.requirements)
			assert.Equal(t, tc.expected, result)
		})
	}
}
func TestGetEnabledLeafs(t *testing.T) {
	testCases := map[string]struct {
		requirements map[string][]string
		states       map[string]State
		expected     []string
	}{
		"with target that has no requirements": {
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
			},
			states: map[string]State{
				"a": StateRunning,
				"b": StateRunning,
				"d": StateRunning,
			},
			expected: []string{"b", "d"},
		},
		"with target that has requirements but disabled": {
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
			},
			states: map[string]State{
				"a": StateStarted,
				"d": StateRunning,
			},
			expected: []string{"a", "d"},
		},
		"big example": {
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
				"b": {
					"d", "e",
				},
				"c": {
					"f", "t", "g",
				},
			},
			states: map[string]State{
				"a": StateRunning,
				"c": StateRunning,
				"t": StateRunning,
				"f": StateRunning,
			},
			expected: []string{"f", "t"},
		},
		"One on": {
			requirements: map[string][]string{
				"a": {},
			},
			states: map[string]State{
				"a": StateStarted,
			},
			expected: []string{"a"},
		},
		"One off": {
			requirements: map[string][]string{
				"a": {},
			},
			states: map[string]State{
				"a": StateDead,
			},
			expected: []string{},
		},
		"Empty": {
			requirements: map[string][]string{},
			states:       map[string]State{},
			expected:     []string{},
		},
	}
	for name := range testCases {
		tc := testCases[name]

		t.Run(name, func(t *testing.T) {
			result := GetEnabledLeafs(tc.states, tc.requirements)
			assert.Equal(t, tc.expected, result)
		})
	}
}
func TestGetDisabledLeafs(t *testing.T) {
	testCases := map[string]struct {
		root         string
		requirements map[string][]string
		states       map[string]State
		expected     []string
	}{
		"with target that has no requirements": {
			root: "a",
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
			},
			states: map[string]State{
				"a": StateDead,
				"c": StateStarted,
				"b": StateRunning,
				"d": StateRunning,
				"e": StateDead,
			},
			expected: []string{"c"},
		},
		"with target that has requirements but disabled": {
			root: "a",
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
			},
			states: map[string]State{
				"a": StateDead,
				"d": StateRunning,
			},
			expected: []string{"b", "c"},
		},
		"big example": {
			root: "a",
			requirements: map[string][]string{
				"a": {
					"b", "c",
				},
				"b": {
					"d", "e",
				},
				"c": {
					"f", "t", "g",
				},
			},
			states: map[string]State{
				"b": StateRunning,
				"g": StateRunning,
			},
			expected: []string{"f", "t"},
		},
		"One on": {
			root: "a",
			requirements: map[string][]string{
				"a": {},
			},
			states: map[string]State{
				"a": StateRunning,
			},
			expected: []string{},
		},
		"One off": {
			root: "a",
			requirements: map[string][]string{
				"a": {},
			},
			states: map[string]State{
				"a": StateDead,
			},
			expected: []string{"a"},
		},
	}
	for name := range testCases {
		tc := testCases[name]

		t.Run(name, func(t *testing.T) {
			result := GetDisabledLeafsFromRoot(tc.root, tc.states, tc.requirements)
			assert.Equal(t, tc.expected, result)
		})
	}
}
