package client

type Options struct {
	ignoreTLSCert bool
	APIKey        string
}

type Option func(*Options)

func IgnoreTLSCert() Option {
	return func(o *Options) {
		o.ignoreTLSCert = true
	}
}

// APIKey sets the API key for authentication
func APIKey(key string) Option {
	return func(o *Options) {
		o.APIKey = key
	}
}

func NewOptions(opts ...Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
