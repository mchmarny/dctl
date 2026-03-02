// Package score implements the standalone reputation scoring model
// using a v3 risk-weighted categorical algorithm with five categories:
// code provenance, identity, engagement, community, and behavioral.
// It exposes [Compute], [Signals], category weights, and [ModelVersion].
package score
