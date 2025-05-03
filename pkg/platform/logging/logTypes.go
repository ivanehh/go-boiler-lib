package logging

import (
	"strconv"

	h "github.com/ivanehh/boiler/internal/helpers"
)

type MapableError interface {
	error
	h.Mapable
}

type ConfigurableError interface {
	ExtendOpts(opts ...any) error
}

type CommonLog struct {
	Wonum               string         `json:"wo"`
	Plant               string         `json:"plant"`
	Details             map[string]any `json:"details"`
	UnstructuredDetails string         `json:"unstructured_details"`
	Error               map[string]any `json:"err"`
	msg                 string         `json:"-"`
}

type CLOpt func(*CommonLog) error

type oNumber interface {
	int | uint | int64
}

func WithOnumPlant[on oNumber](onum on, plant string) CLOpt {
	return func(cl *CommonLog) error {
		onumStr := strconv.Itoa(int(onum))
		cl.Wonum = onumStr
		cl.Plant = plant
		return nil
	}
}

func WithDetails(details map[string]any) CLOpt {
	return func(cl *CommonLog) error {
		cl.Details = details
		return nil
	}
}

func WithUnstructuredDetails(d string) CLOpt {
	return func(cl *CommonLog) error {
		cl.UnstructuredDetails = d
		return nil
	}
}

type CommonError interface {
	MapableError
	ConfigurableError
}

func WithError[E CommonError](cerr E, errOpts ...any) CLOpt {
	return func(cl *CommonLog) error {
		cerr.ExtendOpts(errOpts...)
		cl.Error = cerr.AsMap()
		return nil
	}
}

func NewClog(opts ...CLOpt) CommonLog {
	cl := new(CommonLog)
	for _, opt := range opts {
		opt(cl)
	}
	return *cl
}
