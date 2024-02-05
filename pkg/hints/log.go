package hints

import (
	"context"
	"fmt"
	"os"

	"k8s.io/klog/v2"
)

// EnvRecommendationsAreStrict is the environment variable that can be set
// to make developer recommendations a panic.  It can be set by developers
// while developing to help them keep up with changes to
// kubebuilder-declarative-pattern and any upstream changes.
const EnvRecommendationsAreStrict = "FAIL_ON_DEVELOPER_RECOMMENDATIONS"

// DeveloperRecommendation will log a recommendation for developers;
// helping developers keep up with changes here and upstream.
// If EnvRecommendationsAreStrict is set, we will panic.
func DeveloperRecommendation(ctx context.Context, msg string, keysAndValues ...any) {
	log := klog.FromContext(ctx)
	log.Info(msg, keysAndValues...)
	if os.Getenv(EnvRecommendationsAreStrict) != "" {
		s := fmt.Sprintf("recommendation for developers with %s environment variable set, so treating as a failure: %s", EnvRecommendationsAreStrict, msg)
		for i := 0; (i + 1) < len(keysAndValues); i++ {
			if i != 0 {
				s += ", "
			}
			s += fmt.Sprintf("%v:%v", keysAndValues[i], keysAndValues[i+1])
		}
		panic(s)
	}
}
