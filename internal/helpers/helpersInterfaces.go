package helpers

type Config interface {
	Sources() Sources
}

type Sources interface {
	Databases() []IOWithAuth
	FTPs() []IONoAuth
}

type IOWithAuth interface {
	Enabled() bool
	Type() string
	Name() string
	Addr() string
	Auth() Credentials
}

type IONoAuth interface {
	Enabled() bool
	Type() []string
	Name() string
	Addr() string
}

type Credentials interface {
	Username() string
	Password() string
}

type Structurable interface {
	Mapable
	JSONable
}

type Mapable interface {
	AsMap() map[string]any
}

type JSONable interface {
	AsJSON() []byte
}
