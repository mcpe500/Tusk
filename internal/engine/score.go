package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

var scoreComponents = []string{
	"reliability",
	"interactiveLatency",
	"throughput",
	"memory",
	"startupShutdown",
	"storage",
	"thermal",
}

func Rank(evidence []Evidence, policy Policy) ([]Ranking, error) {
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	if len(evidence) == 0 {
		return nil, errors.New("no evidence to rank")
	}

	rankings := make([]Ranking, len(evidence))
	qualified := make([]int, 0, len(evidence))
	seen := make(map[string]struct{}, len(evidence))
	for i, item := range evidence {
		if item.CandidateID == "" {
			return nil, fmt.Errorf("evidence %d has no candidate ID", i)
		}
		if _, ok := seen[item.CandidateID]; ok {
			return nil, fmt.Errorf("duplicate evidence for %s", item.CandidateID)
		}
		seen[item.CandidateID] = struct{}{}
		rankings[i] = Ranking{CandidateID: item.CandidateID, Metrics: item.Metrics}
		reasons := qualificationFailures(item, policy)
		if len(reasons) > 0 {
			rankings[i].Reasons = reasons
			continue
		}
		rankings[i].Qualified = true
		rankings[i].Components = make(map[string]float64)
		qualified = append(qualified, i)
	}

	if len(qualified) > 0 {
		normalizeAndScore(evidence, rankings, qualified, policy)
	}
	sort.SliceStable(rankings, func(i, j int) bool {
		if rankings[i].Qualified != rankings[j].Qualified {
			return rankings[i].Qualified
		}
		if !almostEqual(rankings[i].Score, rankings[j].Score) {
			return rankings[i].Score > rankings[j].Score
		}
		if !almostEqual(rankings[i].Metrics.PeakMemoryMiB, rankings[j].Metrics.PeakMemoryMiB) {
			return rankings[i].Metrics.PeakMemoryMiB < rankings[j].Metrics.PeakMemoryMiB
		}
		if !almostEqual(rankings[i].Metrics.PersistentStorageMiB, rankings[j].Metrics.PersistentStorageMiB) {
			return rankings[i].Metrics.PersistentStorageMiB < rankings[j].Metrics.PersistentStorageMiB
		}
		return rankings[i].CandidateID < rankings[j].CandidateID
	})
	return rankings, nil
}

func qualificationFailures(item Evidence, policy Policy) []string {
	var reasons []string
	if !item.RequiredConformancePass {
		reasons = append(reasons, "required Podman conformance failed")
	}
	if !item.PlatformCompatible {
		reasons = append(reasons, "requested platform is incompatible")
	}
	if len(item.RejectionReasons) > 0 {
		reasons = append(reasons, item.RejectionReasons...)
	}
	if item.Reliability < policy.MinReliability {
		reasons = append(reasons, fmt.Sprintf("reliability %.3f is below %.3f", item.Reliability, policy.MinReliability))
	}
	if !validMetrics(item.Metrics) {
		reasons = append(reasons, "benchmark metrics are incomplete or invalid")
	}
	return reasons
}

func validMetrics(metrics Metrics) bool {
	return finitePositive(metrics.InteractiveLatencyMillis) &&
		finitePositive(metrics.WorkloadThroughput) &&
		finitePositive(metrics.PeakMemoryMiB) &&
		finitePositive(metrics.StartupShutdownMillis) &&
		finitePositive(metrics.PersistentStorageMiB) &&
		!math.IsNaN(metrics.ThermalRetention) &&
		!math.IsInf(metrics.ThermalRetention, 0) &&
		metrics.ThermalRetention > 0 && metrics.ThermalRetention <= 1
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func normalizeAndScore(evidence []Evidence, rankings []Ranking, indexes []int, policy Policy) {
	componentValues := map[string][]float64{
		"reliability":        {},
		"interactiveLatency": {},
		"throughput":         {},
		"memory":             {},
		"startupShutdown":    {},
		"storage":            {},
		"thermal":            {},
	}
	for _, index := range indexes {
		item := evidence[index]
		componentValues["reliability"] = append(componentValues["reliability"], item.Reliability)
		componentValues["interactiveLatency"] = append(componentValues["interactiveLatency"], item.Metrics.InteractiveLatencyMillis)
		componentValues["throughput"] = append(componentValues["throughput"], item.Metrics.WorkloadThroughput)
		componentValues["memory"] = append(componentValues["memory"], item.Metrics.PeakMemoryMiB)
		componentValues["startupShutdown"] = append(componentValues["startupShutdown"], item.Metrics.StartupShutdownMillis)
		componentValues["storage"] = append(componentValues["storage"], item.Metrics.PersistentStorageMiB)
		componentValues["thermal"] = append(componentValues["thermal"], item.Metrics.ThermalRetention)
	}

	for position, index := range indexes {
		total := 0.0
		for _, name := range scoreComponents {
			values := componentValues[name]
			higherIsBetter := name == "reliability" || name == "throughput" || name == "thermal"
			component := normalized(values[position], values, higherIsBetter)
			rankings[index].Components[name] = component * 100
			total += component * policy.Weights[name]
		}
		rankings[index].Score = total * 100
	}
}

func normalized(value float64, values []float64, higherIsBetter bool) float64 {
	minimum, maximum := values[0], values[0]
	for _, candidate := range values[1:] {
		if candidate < minimum {
			minimum = candidate
		}
		if candidate > maximum {
			maximum = candidate
		}
	}
	if almostEqual(minimum, maximum) {
		return 1
	}
	result := (value - minimum) / (maximum - minimum)
	if !higherIsBetter {
		result = 1 - result
	}
	return result
}

func validatePolicy(policy Policy) error {
	if policy.Name == "" || policy.Version < 1 {
		return errors.New("policy name and positive version are required")
	}
	if policy.MinReliability < 0 || policy.MinReliability > 1 {
		return errors.New("minimum reliability must be between 0 and 1")
	}
	total := 0.0
	for _, name := range scoreComponents {
		weight, ok := policy.Weights[name]
		if !ok || weight < 0 {
			return fmt.Errorf("policy has invalid %s weight", name)
		}
		total += weight
	}
	if !almostEqual(total, 1) {
		return fmt.Errorf("policy weights must total 1, got %.6f", total)
	}
	return nil
}

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) <= 1e-9
}

func PolicyForName(name string) (Policy, error) {
	policy := BalancedPolicy()
	switch name {
	case "balanced":
		return policy, nil
	case "speed":
		policy.Weights = map[string]float64{
			"reliability":        0.25,
			"interactiveLatency": 0.30,
			"throughput":         0.25,
			"memory":             0.05,
			"startupShutdown":    0.10,
			"storage":            0.02,
			"thermal":            0.03,
		}
	case "low-memory":
		policy.Weights = map[string]float64{
			"reliability":        0.25,
			"interactiveLatency": 0.10,
			"throughput":         0.05,
			"memory":             0.35,
			"startupShutdown":    0.10,
			"storage":            0.12,
			"thermal":            0.03,
		}
	case "compatibility":
		policy.Weights = map[string]float64{
			"reliability":        0.35,
			"interactiveLatency": 0.10,
			"throughput":         0.10,
			"memory":             0.10,
			"startupShutdown":    0.05,
			"storage":            0.05,
			"thermal":            0.25,
		}
	default:
		return Policy{}, fmt.Errorf("unsupported policy %q", name)
	}
	return policy, nil
}
