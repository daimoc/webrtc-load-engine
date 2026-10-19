package scenario

import "webrtc-load-engine/src/platform"

// PlatformCatalog is implemented by execution starters that know several
// platforms (008 research R10). The service then validates test plans per
// platform at creation, hides their secrets in what it returns and logs,
// and runs each scenario with the adapter the catalog builds. Without one,
// only "jitsi" is accepted and the starter picks the adapter.
type PlatformCatalog interface {
	// Platforms returns the names accepted at creation.
	Platforms() []string
	// ValidateTestPlan checks a test plan for platform name at creation.
	ValidateTestPlan(name string, tp TestPlan) error
	// SecretKeys names the platform_config keys whose values are secret.
	SecretKeys(name string) []string
	// NewAdapter builds the adapter of a run from the stored test plan.
	NewAdapter(name string, tp TestPlan) (platform.PlatformAdapter, error)
}

// Redacted replaces secret platform_config values in what the service
// returns and logs.
const Redacted = "[redacted]"

// RedactTestPlan returns a copy of tp whose platform_config has the
// non-empty values of keys replaced by Redacted; tp is not changed.
func RedactTestPlan(tp TestPlan, keys []string) TestPlan {
	tp.PlatformConfig = redactPlatformConfig(tp.PlatformConfig, keys)
	return tp
}

// redactPlatformConfig returns a copy of config with the non-empty values
// of keys replaced by Redacted; config itself is not changed.
func redactPlatformConfig(config map[string]any, keys []string) map[string]any {
	if config == nil {
		return nil
	}
	out := make(map[string]any, len(config))
	for k, v := range config {
		out[k] = v
	}
	for _, k := range keys {
		if v, ok := out[k]; ok && v != nil && v != "" {
			out[k] = Redacted
		}
	}
	return out
}
