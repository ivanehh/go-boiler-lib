package datamanagement

import (
	"fmt"

	"github.com/gookit/goutil/structs"
)

type HeaderError interface {
	AsMap() map[string]any
	Header() []string
	Other() []string
}

type ColumnsNotFoundErr struct {
	Available []string
	Required  []string
}

func (e *ColumnsNotFoundErr) Header() []string {
	return e.Available
}

func (e *ColumnsNotFoundErr) Other() []string {
	return e.Required
}

func (e *ColumnsNotFoundErr) Error() string {
	return fmt.Sprintf("not all required columns were found - req:%+v;available:%+v\n", e.Required, e.Available)
}

func (e *ColumnsNotFoundErr) AsMap() map[string]any {
	return structs.ToMap(e, structs.ExportPrivate)
}

type HeaderInterpretErr struct {
	Provided []string
	Found    []string
}

func (e *HeaderInterpretErr) Header() []string {
	return e.Provided
}

func (e *HeaderInterpretErr) Other() []string {
	return e.Found
}

func (e *HeaderInterpretErr) Error() string {
	return fmt.Sprintf("not all required columns were found - req:%+v;available:%+v\n", e.Provided, e.Found)
}

func (e *HeaderInterpretErr) AsMap() map[string]any {
	return structs.ToMap(e, structs.ExportPrivate)
}

type HeaderMismatchErr struct {
	Original []string
	Mismatch []string
}

func (e *HeaderMismatchErr) Header() []string {
	return e.Original
}

func (e *HeaderMismatchErr) Other() []string {
	return e.Mismatch
}

func (e *HeaderMismatchErr) Error() string {
	return fmt.Sprintf("another header was found after one had been selected;original:%+v;mismatch:%+v", e.Original, e.Mismatch)
}

func (e *HeaderMismatchErr) AsMap() map[string]any {
	return structs.ToMap(e, structs.ExportPrivate)
}
