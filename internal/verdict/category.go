// Package verdict defines the classification vocabulary flakehunter uses and
// the mitigation advice attached to each category.
package verdict

import "fmt"

// Category is the root cause assigned to a flaky occurrence.
type Category string

// The closed set of categories. Anything outside this set is rejected by the
// schema validator, so a model cannot invent its own taxonomy.
const (
	NetworkTimeout      Category = "network_timeout"
	RaceCondition       Category = "race_condition"
	InfraFlake          Category = "infra_flake"
	ResourceExhaustion  Category = "resource_exhaustion"
	TestOrderDependency Category = "test_order_dependency"
	GenuineBug          Category = "genuine_bug"
	Unknown             Category = "unknown"
)

// AllCategories lists every valid category, in report order.
func AllCategories() []Category {
	return []Category{
		NetworkTimeout,
		RaceCondition,
		InfraFlake,
		ResourceExhaustion,
		TestOrderDependency,
		GenuineBug,
		Unknown,
	}
}

// Valid reports whether c is a recognised category.
func (c Category) Valid() bool {
	for _, known := range AllCategories() {
		if c == known {
			return true
		}
	}
	return false
}

// ParseCategory converts a string to a Category, erroring on anything outside
// the closed set.
func ParseCategory(s string) (Category, error) {
	c := Category(s)
	if !c.Valid() {
		return Unknown, fmt.Errorf("unknown category %q", s)
	}
	return c, nil
}

// Label returns a human-readable name for reports.
func (c Category) Label() string {
	switch c {
	case NetworkTimeout:
		return "Network timeout"
	case RaceCondition:
		return "Race condition"
	case InfraFlake:
		return "Infrastructure flake"
	case ResourceExhaustion:
		return "Resource exhaustion"
	case TestOrderDependency:
		return "Test order dependency"
	case GenuineBug:
		return "Genuine bug"
	case Unknown:
		return "Unknown"
	default:
		return string(c)
	}
}

// Severity ranks categories for report ordering: a genuine bug hiding behind a
// rerun matters more than a runner blip.
func (c Category) Severity() int {
	switch c {
	case GenuineBug:
		return 5
	case RaceCondition:
		return 4
	case TestOrderDependency:
		return 3
	case ResourceExhaustion:
		return 2
	case NetworkTimeout, InfraFlake:
		return 1
	default:
		return 0
	}
}

func (c Category) String() string { return string(c) }
