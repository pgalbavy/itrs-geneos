package xmlrpc // import "wonderland.org/geneos/xmlrpc"

import (
	"fmt"
	"strings"
)

// Dataview struct encapsulates the Sampler it belongs to and adds the
// name. The name is the aggregated for of [group]-name the "-" is always
// present
type Dataview struct {
	Sampler
	dataviewName string // [group]-name
}

// ToString returns a human readable string to identify the dataview, mainly for debugging
func (d Dataview) ToString() string {
	return fmt.Sprintf("%s/%s.%s.%s", d.URL(), d.EntityName(), d.SamplerName(), d.DataviewName())
}

// IsValid checks if the dataview is (still) valid
func (d Dataview) IsValid() bool {
	res, err := d.viewExists(d.EntityName(), d.SamplerName(), d.DataviewName())
	if err != nil {
		ErrorLogger.Print(err)
		return false
	}
	return res
}

// DataviewName returns to aggregated dataview name (including the optional group)
func (d Dataview) DataviewName() string {
	return d.dataviewName
}

// DataviewGroupNames returns the dataview name and group as two strings, the group
// may be empty
func (d Dataview) DataviewGroupNames() (string, string) {
	s := strings.SplitN(d.dataviewName, "-", 2)
	return s[1], s[0]
}

// SetDataviewName sets the aggregated dataview name given the name and group
// XXX No validation or checking is done
func (d *Dataview) SetDataviewName(dataview string, groupname string) {
	d.dataviewName = groupname + "-" + dataview
}

// Close removes the dataview from the sampler
// It does not cleanup the data structure
func (d Dataview) Close() (err error) {
	if !d.IsValid() {
		return
	}
	view, group := d.DataviewGroupNames()
	err = d.removeView(d.EntityName(), d.SamplerName(), view, group)
	return
}

// UpdateCell sets the value of an existing dataview cell given the row and column name
// The value is formatted using %v so this can be passed any concrete value
//
// No validation is done on args
func (d Dataview) UpdateCell(rowname string, columnname string, value interface{}) (err error) {
	if !d.IsValid() {
		err = fmt.Errorf("UpdateCell(): dataview doesn't exist")
		return
	}
	cellname := rowname + "." + columnname
	s := fmt.Sprintf("%v", value)
	err = d.updateTableCell(d.EntityName(), d.SamplerName(), d.DataviewName(), cellname, s)
	return
}

// UpdateTable replaces the contents of the dataview table but will not work if
// the column names have changed. The underlying API requires the caller to remove the
// original dataview unless you are simply adding new columns
//
// The arguments are a mandatory slice of column names followed by any number
// of rows in the form of a variadic list of slices of strings
func (d Dataview) UpdateTable(columns []string, values ...[]string) (err error) {
	if !d.IsValid() {
		err = fmt.Errorf("UpdateTable(%q): dataview doesn't exist", d.DataviewName())
		return
	}
	var table [][]string
	table = append([][]string{columns}, values...)
	err = d.updateEntireTable(d.EntityName(), d.SamplerName(), d.DataviewName(), table)
	return
}

func (d Dataview) AddRow(name string) (err error) {
	if !d.IsValid() {
		err = fmt.Errorf("AddRows(): dataview doesn't exist")
		return
	}
	err = d.addTableRow(d.EntityName(), d.SamplerName(), d.DataviewName(), name)
	return
}

func (d Dataview) UpdateRow(name string, args ...interface{}) (err error) {
	if !d.IsValid() {
		err = fmt.Errorf("UpdateRow(): dataview doesn't exist")
		return
	}
	var s []string
	for _, v := range args {
		s = append(s, fmt.Sprintf("%v", v))
	}
	err = d.updateTableRow(d.EntityName(), d.SamplerName(), d.DataviewName(), name, s)
	return
}

func (d Dataview) RowNames() (rownames []string, err error) {
	if !d.IsValid() {
		err = fmt.Errorf("RowNames(): dataview doesn't exist")
		return
	}
	rownames, err = d.getRowNames(d.EntityName(), d.SamplerName(), d.DataviewName())
	if err != nil {
		return
	}
	return
}

func (d Dataview) RowNamesOlderThan(unixtime int64) (rownames []string, err error) {
	rownames, err = d.getRowNamesOlderThan(d.EntityName(), d.SamplerName(), d.DataviewName(), unixtime)
	if err != nil {
		return
	}
	return
}

func (d Dataview) CountRows() (int, error) {
	if !d.IsValid() {
		err := fmt.Errorf("CountRows(): dataview doesn't exist")
		return 0, err
	}
	return d.getRowCount(d.EntityName(), d.SamplerName(), d.DataviewName())
}

func (d Dataview) RemoveRow(name string) (err error) {
	if !d.IsValid() {
		err = fmt.Errorf("RemoveRow(): dataview doesn't exist")
		return
	}
	err = d.removeTableRow(d.EntityName(), d.SamplerName(), d.DataviewName(), name)
	return
}

func (d Dataview) AddColumn(name string) (err error) {
	if !d.IsValid() {
		err = fmt.Errorf("AddColumn(): dataview doesn't exist")
		return
	}
	err = d.addTableColumn(d.EntityName(), d.SamplerName(), d.DataviewName(), name)
	return
}

// You cannot remove an existing column. You have to recreate the Dataview

func (d Dataview) ColumnNames() (columnnames []string, err error) {
	if !d.IsValid() {
		err = fmt.Errorf("ColumnNames(): dataview doesn't exist")
		return
	}
	columnnames, err = d.getColumnNames(d.EntityName(), d.SamplerName(), d.DataviewName())
	if err != nil {
		return
	}
	return
}

func (d Dataview) CountColumns() (int, error) {
	if !d.IsValid() {
		err := fmt.Errorf("CountColumns(): dataview doesn't exist")
		return 0, err
	}
	return d.getColumnCount(d.EntityName(), d.SamplerName(), d.DataviewName())
}

// create and optional populate headline
// this is also the entry point to update the value of a headline

func (d Dataview) Headline(name string, args ...string) (err error) {
	if !d.IsValid() {
		err = fmt.Errorf("Headline(): dataview doesn't exist")
		return
	}
	res, err := d.headlineExists(d.EntityName(), d.SamplerName(), d.DataviewName(), name)
	if err != nil {
		return
	}
	if res == false {
		err = d.addHeadline(d.EntityName(), d.SamplerName(), d.DataviewName(), name)
	}
	if err != nil {
		return
	}
	if len(args) > 0 {
		s := fmt.Sprintf("%v", args[0])
		err = d.updateHeadline(d.EntityName(), d.SamplerName(), d.DataviewName(), name, s)
		if err != nil {
			return
		}
	}
	return
}

func (d Dataview) CountHeadlines() (int, error) {
	if !d.IsValid() {
		err := fmt.Errorf("CountHeadlines(): dataview doesn't exist")
		return 0, err
	}
	return d.getHeadlineCount(d.EntityName(), d.SamplerName(), d.DataviewName())
}

func (d Dataview) HeadlineNames() (headlinenames []string, err error) {
	if !d.IsValid() {
		err = fmt.Errorf("HeadlineNames(): dataview doesn't exist")
		return
	}
	headlinenames, err = d.getHeadlineNames(d.EntityName(), d.SamplerName(), d.DataviewName())
	if err != nil {
		return
	}
	return
}

func (d Dataview) RemoveHeadline(name string) error {
	res, err := d.headlineExists(d.EntityName(), d.SamplerName(), d.DataviewName(), name)
	if res == false {
		return err
	}
	return d.removeHeadline(d.EntityName(), d.SamplerName(), d.DataviewName(), name)
}
