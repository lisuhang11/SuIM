package webhook

import (
	"errors"
	"fmt"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

// BeforeConfig controls a synchronous callback that runs before a business action.
type BeforeConfig struct {
	Enable         bool `yaml:"enable" json:"enable"`
	Timeout        int  `yaml:"timeout" json:"timeout"`
	FailedContinue bool `yaml:"failedContinue" json:"failedContinue"`
}

// AfterConfig controls an asynchronous callback that runs after a business action.
type AfterConfig struct {
	Enable  bool `yaml:"enable" json:"enable"`
	Timeout int  `yaml:"timeout" json:"timeout"`
}

// Proceed keeps a before-callback error only when the callback did not ask the flow to continue.
func Proceed(err error) error {
	if err == nil || errors.Is(err, errorcode.ErrWebhookContinue) {
		return nil
	}
	return err
}

// RequireURL reports a missing base URL when any callback switch is on.
func RequireURL(url string, enabled ...bool) error {
	for _, on := range enabled {
		if on && url == "" {
			return fmt.Errorf("webhook url is required when a webhook is enabled")
		}
	}
	return nil
}
