package loops

import "runtime/debug"

// Version is the loops-go module version, resolved from build info when
// the SDK is imported as a dependency. It falls back to "dev" when running
// from a local checkout (including this module's own tests) or when build
// info is otherwise unavailable. It is appended to the default User-Agent
// header sent by [Client] (e.g. "loops-go/v0.2.0").
var Version = readVersion()

// SpecVersion is the Loops OpenAPI spec version this SDK is built against.
const SpecVersion = "1.21.14"

const modulePath = "github.com/loops-so/loops-go"

func readVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	for _, dep := range info.Deps {
		if dep.Path == modulePath && dep.Version != "" {
			return dep.Version
		}
	}
	return "dev"
}
