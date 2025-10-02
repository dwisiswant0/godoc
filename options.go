package godoc

import "context"

// Option is a function that configures a Godoc instance.
type Option func(*Godoc)

// WithGOOS sets the GOOS for the Godoc instance.
func WithGOOS(goos string) Option {
	return func(g *Godoc) {
		g.goos = goos
	}
}

// WithGOARCH sets the GOARCH for the Godoc instance.
func WithGOARCH(goarch string) Option {
	return func(g *Godoc) {
		g.goarch = goarch
	}
}

// WithWorkdir sets the working directory for the Godoc instance.
func WithWorkdir(dir string) Option {
	return func(g *Godoc) {
		g.workdir = dir
	}
}

// WithContext sets the base context used for package loading and external commands.
func WithContext(ctx context.Context) Option {
	return func(g *Godoc) {
		if ctx == nil {
			g.ctx = context.Background()
			return
		}

		g.ctx = ctx
	}
}

// SetOptions applies the given options to the [Godoc] instance.
//
// Note that applying options may override previously set values.
func (g *Godoc) SetOptions(opts ...Option) {
	for _, opt := range opts {
		opt(g)
	}
}
