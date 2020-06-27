package node

import (
	"time"

	"github.com/briankopp/node-quarantine/pkg/config"
	v1 "k8s.io/api/core/v1"
)

type Evaluator interface {
	GetCordonCandidate(now time.Time) *string
	DidCordonNode(candidate string)
	UpdateUnderUtilizedNodes(utils []Utilization, now time.Time)
}

type StandardEvaluator struct {
	config             config.Settings
	underUtilizedNodes map[string]utilAndTime
}

type utilAndTime struct {
	Utilization float64
	Time        time.Time
}

type Utilization struct {
	Name           string
	MaxUtilization float64
}

// NewEvaluator creates a new evaluator
func NewEvaluator(config config.Settings) Evaluator {
	return &StandardEvaluator{
		config:             config,
		underUtilizedNodes: make(map[string]utilAndTime, 0),
	}
}

// DidCordonNode calls
func (m *StandardEvaluator) DidCordonNode(candidate string) {
	_, found := m.underUtilizedNodes[candidate]
	if found {
		delete(m.underUtilizedNodes, candidate)
	}
}

// Get cordon candidate looks at the internal list of under-utilized nodes
// and decides which one should be marked for being cordoned
func (m *StandardEvaluator) GetCordonCandidate(now time.Time) *string {
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
func (m *StandardEvaluator) UpdateUnderUtilizedNodes(utils []Utilization, now time.Time) {
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

// GetNodeUtilizations calculates the node utilizations. Note this
// is slightly more conservative than cluster autoscaler, which excludes
// daemonsets and mirrored pods
func GetNodeUtilizations(nodes *v1.NodeList) []Utilization {
	nodeUtils := []Utilization{}

	for _, node := range nodes.Items {
		util := float64(100)

		cpuAvail := node.Status.Allocatable.Cpu()
		cpuCapacity := node.Status.Capacity.Cpu()
		if cpuAvail != nil && cpuCapacity != nil {
			cpuAvailMilli := float64(cpuAvail.MilliValue())
			cpuCapacityMilli := float64(cpuCapacity.MilliValue())
			if cpuAvailMilli > 0 && cpuCapacityMilli > 0 {
				cpuUtil := 1 - cpuAvailMilli/cpuCapacityMilli
				if cpuUtil < util {
					util = cpuUtil
				}
			}
		}

		memAvail := node.Status.Allocatable.Memory()
		memCapacity := node.Status.Capacity.Memory()
		if memAvail != nil && memCapacity != nil {
			memAvailMilli := float64(memAvail.MilliValue())
			memCapacityMilli := float64(memCapacity.MilliValue())
			if memAvailMilli > 0 && memCapacityMilli > 0 {
				memUtil := 1 - memAvailMilli/memCapacityMilli
				if memUtil < util {
					memUtil = util
				}
			}
		}
		nodeUtils = append(nodeUtils, Utilization{Name: node.Name, MaxUtilization: util})
	}

	return nodeUtils
}
