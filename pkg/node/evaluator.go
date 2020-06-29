package node

import (
	"time"

	"github.com/briankopp/node-quarantine/pkg/config"
)

// Evaluator is something that keeps track of cordon candidates
type Evaluator interface {
	GetCordonCandidate(now time.Time) *string
	DidCordonNode(candidate string)
	UpdateUnderUtilizedNodes(utils []Utilization, now time.Time)
}

type standardEvaluator struct {
	config             config.Settings
	underUtilizedNodes map[string]utilAndTime
}

type utilAndTime struct {
	Utilization float64
	Time        time.Time
}

// Utilization holds a name and max utilization (fraction) for a node
type Utilization struct {
	Name           string
	MaxUtilization float64
}

// NewEvaluator creates a new evaluator
func NewEvaluator(config config.Settings) Evaluator {
	return &standardEvaluator{
		config:             config,
		underUtilizedNodes: make(map[string]utilAndTime, 0),
	}
}

// DidCordonNode calls
func (m *standardEvaluator) DidCordonNode(candidate string) {
	_, found := m.underUtilizedNodes[candidate]
	if found {
		delete(m.underUtilizedNodes, candidate)
	}
}

// GetCordonCandidate looks at the internal list of under-utilized nodes
// and decides which one should be marked for being cordoned
func (m *standardEvaluator) GetCordonCandidate(now time.Time) *string {
	candidateNodes := []string{}
	for key, value := range m.underUtilizedNodes {
		if value.Time.Before(now.Add(-1 * m.config.UnusedAge)) {
			candidateNodes = append(candidateNodes, key)
		}
	}

	// if no candidates, return nil
	if len(candidateNodes) == 0 {
		return nil
	}

	candidate := ""
	lowestUtilization := float64(1)
	for _, name := range candidateNodes {
		value, _ := m.underUtilizedNodes[name]
		if value.Utilization < lowestUtilization {
			candidate = name
		}
	}

	// should also never happen
	if candidate == "" {
		return nil
	}

	return &candidate
}

// UpdateUnderUtilizedNodes takes a list of node utilizations and updates
// an internal list of underutilized nodes that are being watched
func (m *standardEvaluator) UpdateUnderUtilizedNodes(utils []Utilization, now time.Time) {
	// update node list with current utilization values
	for _, node := range utils {
		// if node is over utilization, kick it out of the
		// under-utilization list
		if node.MaxUtilization >= m.config.UtilizationThreshold {
			delete(m.underUtilizedNodes, node.Name)
			continue
		}

		// else, if not in under-utilized list, add it
		_, found := m.underUtilizedNodes[node.Name]
		if !found {
			m.underUtilizedNodes[node.Name] = utilAndTime{Time: now, Utilization: node.MaxUtilization}
		}
	}

	// delete nodes that are no longer even in the running
	for nodeName := range m.underUtilizedNodes {
		found := false
		for _, node := range utils {
			if node.Name == nodeName {
				found = true
				break
			}
		}
		if !found {
			delete(m.underUtilizedNodes, nodeName)
		}
	}
}
