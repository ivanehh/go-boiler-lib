package datamanagement

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	internErrs "github.com/ivanehh/boiler/internal/errors"
	"github.com/pbnjay/grate"
	_ "github.com/pbnjay/grate/simple"
	_ "github.com/pbnjay/grate/xls"
)

type (
	DataframeOpt func(d *Dataframe) error
	BadDataframe struct{}
	Column       struct {
		name string
		idx  int
	}

	Record []string
)

type Dataframe struct {
	Columns     []Column
	Rows        []Record
	CleanerFunc func(Record) Record
	cleaned     bool
}

// DfRowsAsStructList the dataframe as a []sType representation; sType must have 'df' tags
func DfRowsAsStructList[sType any](d *Dataframe) ([]sType, error) {
	var err error
	result := make([]sType, len(d.Rows))
	rPointers := make([]*sType, len(d.Rows))
	for idx := range rPointers {
		rPointers[idx] = new(sType)
	}
	for idx, s := range rPointers {
		sValue := reflect.ValueOf(s).Elem()
		sType := sValue.Type()
		for i := range sValue.NumField() {
			field := sValue.Field(i)
			fieldTag := strings.ToLower(sType.Field(i).Tag.Get("df"))
			if len(fieldTag) == 0 || fieldTag == "-" {
				continue
			}
			if !slices.Contains(d.Header(), fieldTag) {
				continue
			}
			for cid := range d.Columns {
				if d.Columns[cid].name == fieldTag {
					switch field.Kind() {
					case reflect.String:
						field.SetString(d.Rows[idx][cid])
						rPointers[idx] = s
					case reflect.Float64, reflect.Float32:
						var fv float64
						fv, err = strconv.ParseFloat(d.Rows[idx][cid], 64)
						if err != nil {
							return nil, err
						}
						field.SetFloat(fv)
						rPointers[idx] = s
					case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
						var iv int64
						iv, err = strconv.ParseInt(d.Rows[idx][cid], 10, 64)
						if err != nil {
							return nil, err
						}
						field.SetInt(iv)
						rPointers[idx] = s
					case reflect.Uint:
						var uiv uint64
						uiv, err = strconv.ParseUint(d.Rows[idx][cid], 10, 64)
						if err != nil {
							return nil, err
						}
						field.SetUint(uiv)
						rPointers[idx] = s
					}
					break
				}
			}
		}
	}
	for idx, r := range rPointers {
		result[idx] = *r
	}
	return result, nil
}

func (d *Dataframe) Header() []string {
	header := make([]string, len(d.Columns))
	for i := range d.Columns {
		header[i] = d.Columns[i].name
	}
	return header
}

func withRecordsFromData(b []byte, newLine string, valueSep string) DataframeOpt {
	return func(d *Dataframe) error {
		records := bytes.Split(b, []byte(newLine))
		for _, r := range records {
			dfRecord := make(Record, 0)
			values := bytes.Split(r, []byte(valueSep))
			for _, v := range values {
				dfRecord = append(dfRecord, string(v))
			}
			if d.CleanerFunc != nil {
				dfRecord = d.CleanerFunc(dfRecord)
			}
			d.Rows = append(d.Rows, dfRecord)
		}
		return nil
	}
}

func recordsFromFiles(filePaths []string) DataframeOpt {
	return func(d *Dataframe) error {
		var head []string
		for idx, fp := range filePaths {
			source, err := grate.Open(fp)
			if err != nil {
				return err
			}
			sheets, err := source.List()
			if err != nil {
				return err
			}
			data, err := source.Get(sheets[0])
			if err != nil {
				return err
			}
			/*
				this part is a bit awkward
				if we are not at the first file then we want to skip the header
			*/
			if idx != 0 {
				for data.Next() {
					// advance rows as long as they are empty
					if len(data.Strings()[0]) < 1 || len(data.Strings()[0]) == 0 {
						continue
					}
					// do not generate dataframe for file sets that do not have identical headers
					if head != nil {
						var record Record
						r := data.Strings()
						// INFO: The code until the end of this closure deals with the sanitization of the provided files
						// this part is for csv files
						if strings.Contains(r[0], ",") {
							if record = d.CleanerFunc(strings.Split(r[0], ",")); len(record) > 0 {
								if slices.Compare(head, record) != 0 {
									return &internErrs.HeaderMismatchErr{
										Original: head,
										Mismatch: record,
									}
								}
							}
						} else { // this part is for excel files
							if record = d.CleanerFunc(r); len(record) > 0 {
								if slices.Compare(head, record) != 0 {
									return &internErrs.HeaderMismatchErr{
										Original: head,
										Mismatch: record,
									}
								}
							}
						}
					}
					break
				}
			}
			for data.Next() {
				r := data.Strings()
				var cr Record
				if strings.Contains(r[0], ",") {
					if cr = d.CleanerFunc(strings.Split(r[0], ",")); len(cr) > 0 {
						d.Rows = append(d.Rows, cr)
					}
				} else {
					if cr = d.CleanerFunc(r); len(cr) > 0 {
						d.Rows = append(d.Rows, cr)
					}
				}
				// set the default header for this dataframe
				if slices.ContainsFunc(cr, func(e string) bool {
					return strings.EqualFold(e, "date")
				}) && head == nil {
					head = d.CleanerFunc(cr)
				}
			}
		}
		return nil
	}
}

// func cleanRecord(r []string) Record {
// 	newR := make(Record, 0)
// 	for idx := range r {
// 		if len(r[idx]) > 0 {
// 			newR = append(newR, strings.Trim(r[idx], " +-"))
// 		}
// 	}
// 	return newR
// }

// WithProvidedColumns does not remove the first row of the dataframe!
func WithProvidedColumns(h []string) DataframeOpt {
	return func(d *Dataframe) error {
		if len(h) != len(d.Rows[0]) {
			return &internErrs.HeaderInterpretErr{Provided: h, Found: d.Rows[0]}
		}

		for idx, str := range h {
			d.Columns = append(d.Columns, Column{
				name: strings.ToLower(strings.ReplaceAll(str, " ", "")),
				idx:  idx,
			})
		}
		return nil
	}
}

// WithInterpretedColumns uses the first row of the dataframe to interpret the column names; it then removes the row from the dataframe; this is the default behavior
func WithInterpretedColumns() DataframeOpt {
	return func(d *Dataframe) error {
		for idx, str := range d.Rows[0] {
			d.Columns = append(d.Columns, Column{
				name: strings.ToLower(strings.ReplaceAll(str, " ", "")),
				idx:  idx,
			})
		}
		d.Rows = d.Rows[1:]
		return nil
	}
}

// Drop a range of rows from the dataframe
func (d *Dataframe) Drop(i ...int) {
	slices.Sort(i)
	d.Rows = slices.Delete(d.Rows, i[0], i[len(i)-1])
	newRows := make([]Record, 0)
	for _, row := range d.Rows {
		if len(row) == 0 {
			continue
		}
		newRows = append(newRows, row)
	}
	d.Rows = newRows
}

func (d *Dataframe) Get(row int, columns ...string) (*Dataframe, error) {
	var r []string
	var result Record
	r = d.Rows[row]
	dnew := new(Dataframe)
	if len(columns) == 0 {
		dnew.Columns = d.Columns
		dnew.Rows = []Record{d.Rows[row]}
		return dnew, nil
	}
	for _, c := range d.Columns {
		if slices.ContainsFunc(columns, func(e string) bool {
			return func(dfc string) bool {
				return strings.EqualFold(e, dfc)
			}(c.name)
		}) {
			result = append(result, r[c.idx])
			dnew.Columns = append(dnew.Columns, c)
		}
	}
	if len(dnew.Columns) != len(columns) {
		return nil, &internErrs.ColumnsNotFoundErr{
			Available: d.Header(),
			Required:  columns,
		}
	}
	dnew.Rows = []Record{result}
	return dnew, nil
}

var (
	ErrBadRowIdx = errors.New("bad row index")
	ErrBadRow    = errors.New("mismatch between row and dataframe format")
)

// SetRecord replaces the record and the provided row with the provided record
func (d *Dataframe) SetRecord(row int, record Record) error {
	if row > len(d.Rows)-1 {
		return fmt.Errorf("%w:%d", ErrBadRowIdx, row)
	}
	if len(record) != len(d.Header()) {
		return fmt.Errorf("%w:record length:%d does not match dataframe header length:%d", ErrBadRow, len(record), len(d.Header()))
	}
	d.Rows[row] = record
	return nil
}

var ErrIncompatibleDataframes = errors.New("incompaitble dataframes")

func compareHeaders(header1, header2 []string) int {
	if len(header1) != len(header2) {
		return -1
	}
	for idx, col := range header1 {
		if !slices.Contains(header2, col) {
			return idx
		}
	}
	return 0
}

func (d *Dataframe) Append(candidate *Dataframe) (*Dataframe, error) {
	if v := compareHeaders(d.Header(), candidate.Header()); v != 0 {
		if v == -1 {
			return d, fmt.Errorf("%w: headers are of different length: host:%d candidate:%d", ErrIncompatibleDataframes, len(d.Header()), len(candidate.Header()))
		}
		return d, fmt.Errorf("%w: mismatch at idx:%d", ErrIncompatibleDataframes, v)
	}
	for _, rec := range candidate.Rows {
		if cleanRec := d.CleanerFunc(rec); len(cleanRec) != 0 {
			d.Rows = append(d.Rows, cleanRec)
		}
	}
	return d, nil
}

func NewDataframeFromFiles(filesPaths []string, cleaner func(Record) Record, opts ...DataframeOpt) (*Dataframe, error) {
	df := new(Dataframe)
	// INFO: A hacky solution to avoid a nil cleanerfunc
	df.CleanerFunc = func(r Record) Record {
		return r
	}
	if opts == nil {
		opts = make([]DataframeOpt, 0)
	}

	// INFO: A bit hacky but this is how we ensure that the data is loaded first
	if cleaner != nil {
		df.CleanerFunc = cleaner
	}
	opts = append(opts, recordsFromFiles(filesPaths))
	slices.Reverse(opts)

	for _, opt := range opts {
		err := opt(df)
		if err != nil {
			return nil, err
		}
	}
	return df, nil
}

type ByteDefinition struct {
	// the data that should be in the dataframe
	Data []byte
	// separator between each record
	LineSep string
	// separator between values in a record
	ValSep string
}

func NewDataframeFromData(b ByteDefinition, cleaner func(Record) Record, opts ...DataframeOpt) (*Dataframe, error) {
	df := new(Dataframe)
	if opts == nil {
		opts = make([]DataframeOpt, 0)
	}
	if cleaner != nil {
		df.CleanerFunc = cleaner
	}

	// INFO: A bit hacky but this is how we ensure that the data is loaded first
	opts = append(opts, withRecordsFromData(b.Data, b.LineSep, b.ValSep))
	slices.Reverse(opts)

	for _, opt := range opts {
		err := opt(df)
		// TODO: Should we quit dataframe construction and return if a dataframe opt fails?
		if err != nil {
			return nil, err
		}
	}

	return df, nil
}
