package samplers // import "wonderland.org/geneos/samplers"

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"wonderland.org/geneos"
	"wonderland.org/geneos/plugins"
	"wonderland.org/geneos/xmlrpc"
)

var (
	Logger      = geneos.Logger
	DebugLogger = geneos.DebugLogger
	ErrorLogger = geneos.ErrorLogger
)

type SamplerInstance interface {
	New(plugins.Connection, string, string) *SamplerInstance
	InitSampler(*SamplerInstance) (err error)
	DoSample(*SamplerInstance) (err error)
}

// All plugins share common settings
type Samplers struct {
	plugins.Plugins
	name        string
	group       string
	dataview    *xmlrpc.Dataview
	interval    time.Duration
	columns     Columns
	columnnames []string
	sortcolumn  string
}

// Columns is a common type for the map of rows for output.
type Columns map[string]columndetails

// columndetails has to be it's own type so that it can be used in maps
type columndetails struct {
	tags     string                   // copy of tags for now
	name     string                   // display name of column. name="OMIT" mean not rendered
	number   int                      // column index - convenience for now
	format   string                   // alterative Printf format, default is %v
	convfunc func(interface{}) string // this may happen - not used
	sort     sortType                 // if this is the sorting column then what type from above
}

const (
	// sort=[+|-][num] = sort by this column optionally asc/desc and optionally numeric, one or the other
	sorting = "sort"
	// format is a fmt.Printf format string for the data and defaults to %v
	format = "format"
)

type sortType int

const (
	sortNone sortType = iota
	sortAsc
	sortDesc
	sortAscNum
	sortDescNum
)

// these two internal functions implement the redirection required to
// call initialisation and sample routines from plugins. without
// there, using direct calls, the process will crash if one of the functions
// isn't defined and there is no way to check before calling. this also
// allows for future shared initialisation code

func (p *Samplers) initSamplerInternal() error {
	if v, ok := interface{}(p.Plugins).(interface{ InitSampler() error }); ok {
		return v.InitSampler()
	}
	return nil
}

func (p *Samplers) dosample() error {
	if v, ok := interface{}(p.Plugins).(interface{ DoSample() error }); ok {
		return v.DoSample()
	}
	return nil
}

func (p *Samplers) SetName(name string, group string) {
	p.name = name
	p.group = group
}

func (p Samplers) Name() (name string, group string) {
	return p.name, p.group
}

func (p *Samplers) SetInterval(interval time.Duration) {
	p.interval = interval
	return
}

func (p Samplers) Interval() time.Duration {
	return p.interval
}

func (p *Samplers) SetColumnNames(columnnames []string) {
	p.columnnames = columnnames
	return
}

func (p Samplers) ColumnNames() []string {
	return p.columnnames
}

func (p *Samplers) SetColumns(columns Columns) {
	p.columns = columns
	return
}

func (p Samplers) Columns() Columns {
	return p.columns
}

func (p *Samplers) SetSortColumn(column string) {
	p.sortcolumn = column
	return
}

func (p Samplers) SortColumn() string {
	return p.sortcolumn
}

func (p Samplers) Dataview() *xmlrpc.Dataview {
	return p.dataview
}

func (p *Samplers) InitDataviews(c plugins.Connection) (err error) {
	d, err := c.NewDataview(p.name, p.group)
	if err != nil {
		return
	}
	p.dataview = d
	return
}

func (p *Samplers) Start(wg *sync.WaitGroup) (err error) {
	if p.dataview == nil {
		err = fmt.Errorf("Start(): Dataview not defined")
		return
	}
	err = p.initSamplerInternal()
	if err != nil {
		return
	}
	wg.Add(1)
	go func() {
		tick := time.NewTicker(p.Interval())
		defer tick.Stop()
		for {
			<-tick.C
			err := p.dosample()
			if err != nil {
				break
			}
		}
		wg.Done()
		fmt.Printf("sampler %q exiting\n", p.Dataview().ToString())

	}()
	return
}

func (s *Samplers) Close() error {
	if s.dataview == nil {
		return nil
	}
	return s.dataview.Close()
}

// the methods below are helpers for common cases of needing to render a struct of data as
// a row etc.

/*
ColumnInfo is a helper function that takes a (flat) struct as input
and returns an ordered slice of column names ready to update a dataview.
Normally called once per sampler during initialisation.

The column names are the display names in the struct tags or the field name
otherwise. The internal method parsetags() is where the valid options are
defined in detail. More docs to follow.

The input is a type or an zero-ed struct as this method only checks the struct
tags and doesn't care about the data

*/
func (s Samplers) ColumnInfo(rowdata interface{}) (cols Columns,
	columnnames []string, sorting string, err error) {
	rv := reflect.Indirect(reflect.ValueOf(rowdata))
	if rv.Kind() != reflect.Struct {
		err = fmt.Errorf("rowdata is not a struct")
		return
	}

	rt := rv.Type()
	cols = make(Columns, rt.NumField())
	sorting = rt.Field(0).Name

	for i := 0; i < rt.NumField(); i++ {
		column := columndetails{}
		fieldname := rt.Field(i).Name
		if tags, ok := rt.Field(i).Tag.Lookup("column"); ok {
			column, err = parseTags(fieldname, tags)
			if err != nil {
				return
			}
			// check for already set values and error
			if column.sort != sortNone {
				sorting = fieldname
			}
			column.number = i
		} else {
			column.name = fieldname
			column.number = i
			column.format = "%v"
		}
		// A column marked "OMIT" is still useable but is not included
		// in the column names
		if column.name != "OMIT" {
			columnnames = append(columnnames, column.name)
		}
		cols[fieldname] = column
	}

	return
}

/*
UpdateTableFromMap - Given a map of structs representing rows of data,
render a simple table update by converting all data and sorting the rows
by the sort column in the initialised ColumnNames member of the Sampler

Sorting the data is only to define the "natural sort order" of the data
as it appears in a Geneos Dataview without further client-side sorting.
*/
func (s *Samplers) UpdateTableFromMap(data interface{}) error {
	table, _ := s.RowsFromMap(data)
	return s.Dataview().UpdateTable(s.ColumnNames(), table...)
}

/*
RowFromMap is a helper function that takes a tagged (flat) struct as input
and formats a row (slice of strings) using tags. TBD, but selecting the
rowname, the sorting, the format and type conversion, scaling and labels (percent,
MB etc.)

The data passed should NOT include column heading slice as it will be
regenerated from the Columns data

*/
func (s Samplers) RowsFromMap(rowdata interface{}) (rows [][]string, err error) {
	c := s.Columns()
	r := reflect.Indirect(reflect.ValueOf(rowdata))
	if r.Kind() != reflect.Map {
		err = fmt.Errorf("non Map passed")
		return
	}

	for _, k := range r.MapKeys() {
		var cells []string
		rawcells, _ := rowFromStruct(c, r.MapIndex(k))
		t := reflect.Indirect(r.MapIndex(k)).Type()
		for i := range rawcells {
			fieldname := t.Field(i).Name
			format := c[fieldname].format
			if c[fieldname].name == "OMIT" {
				continue
			}
			cells = append(cells, fmt.Sprintf(format, rawcells[i]))
		}
		rows = append(rows, cells)
	}

	rows = c.sortRows(rows, s.SortColumn())

	return
}

/*
UpdateTableFromSlice - Given an ordered slice of structs of data the
method renders a simple table of data as defined in the Columns
part of Samplers

*/
func (s Samplers) UpdateTableFromSlice(rowdata interface{}) error {
	table, _ := s.RowsFromSlice(rowdata)
	return s.Dataview().UpdateTable(s.ColumnNames(), table...)
}

// RowsFromSlice - results are not resorted, they are assumed to be in the order
// required
func (s Samplers) RowsFromSlice(rowdata interface{}) (rows [][]string, err error) {
	c := s.Columns()

	rd := reflect.Indirect(reflect.ValueOf(rowdata))
	if rd.Kind() != reflect.Slice {
		err = fmt.Errorf("non Slice passed")
		return
	}

	for i := 0; i < rd.Len(); i++ {
		v := rd.Index(i)
		t := v.Type()

		rawcells, _ := rowFromStruct(c, v)
		var cells []string
		for i := range rawcells {
			fieldname := t.Field(i).Name
			format := c[fieldname].format
			if c[fieldname].name == "OMIT" {
				continue
			}
			cells = append(cells, fmt.Sprintf(format, rawcells[i]))
		}
		rows = append(rows, cells)
	}

	return
}

/*
UpdateTableFromMapDelta
*/
func (s *Samplers) UpdateTableFromMapDelta(newdata, olddata interface{}, interval time.Duration) error {
	table, _ := s.RowsFromMapDelta(newdata, olddata, interval)
	return s.Dataview().UpdateTable(s.ColumnNames(), table...)
}

// RowsFromMapDelta takes two sets of data and calculates the difference between them.
// Only numeric data is changed, any non-numeric fields are left
// unchanges and taken from newrowdata only. If an interval is supplied (non-zero) then that is used as
// a scaling value otherwise the straight numeric difference is calculated
//
// This is for data like sets of counters that are absolute values over time
func (s Samplers) RowsFromMapDelta(newrowdata, oldrowdata interface{},
	interval time.Duration) (rows [][]string, err error) {

	c := s.Columns()

	// if no interval is supplied - the same as an interval of zero
	// then set 1 second as the interval as the divisor below takes
	// the number of seconds as the value, hence cancelling itself out
	if interval == 0 {
		interval = 1 * time.Second
	}

	rnew := reflect.Indirect(reflect.ValueOf(newrowdata))
	if rnew.Kind() != reflect.Map {
		err = fmt.Errorf("non map passed")
		return
	}

	rold := reflect.Indirect(reflect.ValueOf(oldrowdata))
	if rold.Kind() != reflect.Map {
		err = fmt.Errorf("non map passed")
		return
	}

	for _, k := range rnew.MapKeys() {
		rawold, _ := rowFromStruct(c, rold.MapIndex(k))
		rawcells, _ := rowFromStruct(c, rnew.MapIndex(k))
		var cells []string
		t := reflect.Indirect(rnew.MapIndex(k)).Type()
		for i := range rawcells {
			fieldname := t.Field(i).Name
			format := c[fieldname].format
			if c[fieldname].name == "OMIT" {
				continue
			}

			// calc diff here
			oldcell, newcell := rawold[i], rawcells[i]
			if reflect.TypeOf(oldcell) != reflect.TypeOf(newcell) {
				err = fmt.Errorf("non-matching types in data")
				return
			}
			// can these fields be converted to float (the concrete value)
			// this is not the same as parsing a string as float, but the
			// actual struct field being numeric
			newfloat, nerr := toFloat(newcell)
			oldfloat, oerr := toFloat(oldcell)
			if nerr == nil && oerr == nil {
				cells = append(cells, fmt.Sprintf(format, (newfloat-oldfloat)/interval.Seconds()))
			} else {
				// if we fail to convert then just render the new values directly
				cells = append(cells, fmt.Sprintf(format, newcell))
			}
		}
		rows = append(rows, cells)
	}

	rows = c.sortRows(rows, s.SortColumn())

	return
}

func toFloat(f interface{}) (float64, error) {
	var ft = reflect.TypeOf(float64(0))
	v := reflect.ValueOf(f)
	v = reflect.Indirect(v)
	if !v.Type().ConvertibleTo(ft) {
		return 0, fmt.Errorf("cannot convert %v to float", v.Type())
	}
	fv := v.Convert(ft)
	return fv.Float(), nil
}

func (c Columns) sortRows(rows [][]string, sortcol string) [][]string {
	sorttype, sortby := c[sortcol].sort, c[sortcol].number

	sort.Slice(rows, func(a, b int) bool {
		switch sorttype {
		case sortDesc:
			return rows[a][sortby] >= rows[b][sortby]
		case sortAscNum:
			// err is ignored, zero is a valid answer if the
			// contents are not a float
			an, _ := strconv.ParseFloat(rows[a][sortby], 64)
			bn, _ := strconv.ParseFloat(rows[b][sortby], 64)
			if an == bn {
				return rows[a][sortby] < rows[b][sortby]
			} else {
				return an < bn
			}
		case sortDescNum:
			// err is ignored, zero is a valid answer if the
			// contents are not a float
			an, _ := strconv.ParseFloat(rows[a][sortby], 64)
			bn, _ := strconv.ParseFloat(rows[b][sortby], 64)
			if an == bn {
				return rows[a][sortby] >= rows[b][sortby]
			} else {
				return an >= bn
			}
		// case sortNone, sortAsc: - the default
		default:
			return rows[a][sortby] < rows[b][sortby]
		}
	})
	return rows
}

// pivot the struct members to a slice of their values ready to be
// processed to a slice of strings
func rowFromStruct(c Columns, rv reflect.Value) (row []interface{}, err error) {
	rv = reflect.Indirect(rv)
	if rv.Kind() != reflect.Struct {
		err = fmt.Errorf("row data not a struct")
		return
	}

	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		row = append(row, rv.Field(i).Interface())
	}

	return
}

func parseTags(fieldname string, tag string) (cols columndetails, err error) {
	// non "zero" default
	cols.tags = tag
	cols.name = fieldname
	cols.format = "%v"

	tags := strings.Split(tag, ",")
	for _, t := range tags {
		//fmt.Printf("checking tag %q -> %q\n", fieldname, t)
		i := strings.IndexByte(t, '=')
		if i == -1 {
			if cols.name != fieldname {
				// err, already defined
				err = fmt.Errorf("column name %q redefined more than once", cols.name)
				return
			}
			cols.name = t
			continue
		}
		prefix := t[:i]

		switch prefix {
		case sorting:
			cols.sort = sortAsc
			if t[i+1] == '-' {
				cols.sort = sortDesc
			}
			if strings.HasSuffix(t[i+1:], "num") {
				if cols.sort == sortAsc {
					cols.sort = sortAscNum
				} else {
					cols.sort = sortDescNum
				}
			}

		case format:
			// no validation
			cols.format = t[i+1:]
		}
	}
	return
}
