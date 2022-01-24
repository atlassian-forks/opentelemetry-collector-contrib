package featureflag

import (
	"errors"
	"gopkg.in/launchdarkly/go-sdk-common.v2/lduser"
	ld "gopkg.in/launchdarkly/go-server-sdk.v5"
	"time"
)

func New(ffKey string) (FeatureFlag, error) {
	var config ld.Config
	if len(ffKey) == 0 {
		config.Offline = true
	}

	ldClient, err := ld.MakeCustomClient(ffKey, config, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &featureFlag{
		Client: ldClient,
	}, nil
}

type FeatureFlag interface {
	EnabledForService(string) (bool, error)
}

var _ FeatureFlag = (*featureFlag)(nil)

type featureFlag struct {
	Client *ld.LDClient
}

func (ld *featureFlag) EnabledForService(serviceID string) (bool, error) {
	if ld.Client == nil {
		return false, errors.New("LaunchDarkly is not initialized")
	}
	return ld.Client.BoolVariation("OBC-253-generate-metrics-from-span-of-service-proxy",
		lduser.NewUser(serviceID), false)
}
