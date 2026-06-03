package clock

import (
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin(options ...Option) *pluginbinding.Plugin {
	var cfg Config
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	provider := newProvider(cfg)
	spec := manifestSpec()
	return pluginbinding.Define(spec,
		pluginbinding.RegisterContextProvider(spec.Context[0], provider.Build),
	)
}

type Option func(*Config)

func WithNow(now func() time.Time) Option {
	return func(cfg *Config) {
		cfg.Now = now
	}
}

func WithTimezone(tz string) Option {
	return func(cfg *Config) {
		cfg.TZ = tz
	}
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
