package xmlrpc // import "wonderland.org/geneos/xmlrpc"

import (
	"fmt"
	"sync"
	"time"
)

type Sampler struct {
	Client
	entityName  string
	samplerName string
	waitGroup   sync.WaitGroup
	hearbeats   []chan struct{}
}

func (s Sampler) ToString() string {
	return fmt.Sprintf("%s/%s.%s", s.URL(), s.EntityName(), s.SamplerName())
}

func (s *Sampler) IsValid() bool {
	DebugLogger.Print("called")

	res, err := s.samplerExists(s.EntityName(), s.SamplerName())
	if err != nil {
		return false
	}
	return res
}

// Getters only
// There are no setters as once created the struct should be immutable as
// otherwise it would not be safe in concurrency terms. The structs get
// copied around a bit

// EntityName returns the Entuty name as a string
func (s Sampler) EntityName() string {
	return s.entityName
}

// SamplerName returns the Sampler name as a string
func (s Sampler) SamplerName() string {
	return s.samplerName
}

// Parameter - Get a parameter from the Geneos sampler config as a string
// It would not be difficult to add numberic and other type getters
func (s Sampler) Parameter(name string) (string, error) {
	DebugLogger.Print("called")

	if !s.IsValid() {
		err := fmt.Errorf("Parameter(): sampler doesn't exist")
		return "", err
	}
	return s.getParameter(s.EntityName(), s.SamplerName(), name)
}

// start a goroutine to send a heartbeat every "seconds"
// unless already defined, save the waitgroup in sampler
// else increment the count
func (s *Sampler) xSignOn(seconds int) (err error) {
	err = s.signOn(s.EntityName(), s.SamplerName(), seconds)
	s.waitGroup.Add(1)
	go func(sampler *Sampler, seconds int) {
		for true {
			err := s.heartbeat(s.EntityName(), s.SamplerName())
			if err != nil {
				s.waitGroup.Done()
				break
			}
			time.Sleep(time.Duration(seconds) * time.Second)
		}
	}(s, seconds)
	return
}

func (s *Sampler) xSignOff() (err error) {
	if !s.IsValid() {
		err = fmt.Errorf("xSignOff(): sampler doesn't exist")
		return
	}
	s.waitGroup.Done()
	return s.signOff(s.EntityName(), s.SamplerName())
}

/*
NewDataview - Create a new Dataview on the Sampler with an optional initial table of data.

If supplied the data is in the form of rows, each of which is a slice of
strings containing cell data. The first row must be the column names and the first
string in each row must be the rowname (including the first row of column names).

The underlying API appears to accept incomplete data so you can just send a row
of column names followed by each row only contains the first N columns each.
*/
func (s Sampler) NewDataview(dataviewName string, groupName string, args ...[]string) (d *Dataview, err error) {
	DebugLogger.Print("called")
	if !s.IsValid() {
		err = fmt.Errorf("NewDataview(): sampler doesn't exist")
		ErrorLogger.Print(err)
		return
	}

	// try to remove it - failure shouldn't matter
	_ = s.removeView(s.EntityName(), s.SamplerName(), dataviewName, groupName)

	d, err = s.CreateDataview(dataviewName, groupName)
	if err != nil {
		ErrorLogger.Fatal(err)

		return
	}

	if len(args) > 0 {
		d.UpdateTable(args[0], args[1:]...)
	}
	return
}

// CreateDataview creates a new dataview struct and calls the API to create one on the
// Netprobe. It does NOT check for an existing dataview or remove it if one exists
func (s Sampler) CreateDataview(dataviewName string, groupName string) (d *Dataview, err error) {
	DebugLogger.Print("called")
	d = &Dataview{s, groupName + "-" + dataviewName}
	err = d.createView(s.EntityName(), s.SamplerName(), dataviewName, groupName)
	return
}
