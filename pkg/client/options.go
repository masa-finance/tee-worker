package client

type Options struct {
	ignoreTLSCert bool
}

type Option func(*Options)

func IgnoreTLSCert() Option {
	return func(o *Options) {
		o.ignoreTLSCert = true
	}
}

func NewOptions(opts ...Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
