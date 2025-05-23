package client

import "time"

type Options struct {
	ignoreTLSCert bool
	APIKey        string
	Timeout       time.Duration
}

type Option func(*Options) error

func IgnoreTLSCert() Option {
	return func(o *Options) error {
		o.ignoreTLSCert = true
		return nil
	}
}

// APIKey sets the API key for authentication
func APIKey(key string) Option {
	return func(o *Options) error {
		o.APIKey = key
		return nil
	}
}

func Timeout(timeout string) Option {
	return func(o *Options) error {
		duration, err := time.ParseDuration(timeout)
		if err != nil {
			return err
		}
		o.Timeout = duration
		return nil
	}
}

func NewOptions(opts ...Option) (*Options, error) {
	o := &Options{
		Timeout: 1 * time.Minute,
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}
