package client

import "time"

type Options struct {
	ignoreTLSCert       bool
	APIKey              string
	Timeout             time.Duration
	MaxConnsPerHost     int
	MaxIdleConnsPerHost int
	MaxIdleConns        int
	IdleConnTimeout     time.Duration
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

func Timeout(timeout time.Duration) Option {
	return func(o *Options) error {
		o.Timeout = timeout
		return nil
	}
}

// MaxConnsPerHost sets the maximum number of connections per host (in all states) in the connection pool. The default is 100.
func MaxConnsPerHost(conns uint) Option {
	return func(o *Options) error {
		o.MaxConnsPerHost = int(conns)
		return nil
	}
}

// MaxIdleConnsPerHost sets the maximum number of idle connections per host in the connection pool. The default is 10.
func MaxIdleConnsPerHost(conns uint) Option {
	return func(o *Options) error {
		o.MaxIdleConnsPerHost = int(conns)
		return nil
	}
}

// MaxIdleConns sets the maximum number of idle connections in the connection pool. The default is 100.
func MaxIdleConns(conns uint) Option {
	return func(o *Options) error {
		o.MaxIdleConns = int(conns)
		return nil
	}
}

// IdleConnTimeout sets the timeout before an idle connection pool connection closes itself. The default is 2 minutes.
func IdleConnTimeout(timeout time.Duration) Option {
	return func(o *Options) error {
		o.IdleConnTimeout = timeout
		return nil
	}
}

func NewOptions(opts ...Option) (*Options, error) {
	o := &Options{
		Timeout:             1 * time.Minute,
		MaxConnsPerHost:     100,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     2 * time.Minute,
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}
