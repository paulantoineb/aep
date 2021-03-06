/*
Copyright © 2017 the AEP authors.
This file is part of AEP.

AEP is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

AEP is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with AEP.  If not, see <http://www.gnu.org/licenses/>.
*/

package aeputil

import (
	"os"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/ctessum/aep"
	"github.com/ctessum/unit"
)

func TestScale(t *testing.T) {
	type config struct {
		Inventory InventoryConfig
	}
	r, err := os.Open("testdata/example_config.toml")
	if err != nil {
		t.Fatal(err)
	}

	c := new(config)

	// Read the configuration file into the configuration variable.
	if _, err = toml.DecodeReader(r, c); err != nil {
		t.Fatal(err)
	}
	emis, _, err := c.Inventory.ReadEmissions()
	if err != nil {
		t.Fatal(err)
	}

	sccDesc, err := os.Open("testdata/SCCDownload-2017-0622-080147.csv")
	if err != nil {
		t.Fatal(err)
	}

	f, err := ScaleNEIStateTrends("testdata/state_tier1_90-16.xlsx", sccDesc, 2014, 2016)
	if err != nil {
		t.Fatal(err)
	}

	beforeWant := map[aep.Pollutant]*unit.Unit{
		aep.Pollutant{Name: "NOX"}:   unit.New(1.9697839276290547e+07, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "VOC"}:   unit.New(650426.9504917137, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "PM2_5"}: unit.New(1.3253413523899838e+06, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "SO2"}:   unit.New(1.5806320939220862e+07, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "NH3"}:   unit.New(34.056105917699995, unit.Dimensions{4: 1}),
	}
	afterWant := map[aep.Pollutant]*unit.Unit{
		aep.Pollutant{Name: "NOX"}:   unit.New(1.797061138444698e+07, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "VOC"}:   unit.New(589712.1799472809, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "PM2_5"}: unit.New(1.167524338454476e+06, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "SO2"}:   unit.New(6.836759306135574e+06, unit.Dimensions{4: 1}),
		aep.Pollutant{Name: "NH3"}:   unit.New(36.49376855692243, unit.Dimensions{4: 1}),
	}
	before := sum(emis)
	if !reflect.DeepEqual(before, beforeWant) {
		t.Errorf("before scaling: want %v, have %v", beforeWant, before)
	}

	if err := Scale(emis, f); err != nil {
		t.Fatal(err)
	}
	after := sum(emis)
	if !reflect.DeepEqual(after, afterWant) {
		t.Errorf("after scaling: want %v, have %v", afterWant, after)
	}
}

func sum(d map[string][]aep.Record) map[aep.Pollutant]*unit.Unit {
	o := make(map[aep.Pollutant]*unit.Unit)
	for _, recs := range d {
		for _, rec := range recs {
			t := rec.Totals()
			for pol, v := range t {
				o[pol] = unit.Add(o[pol], v)
			}
		}
	}
	return o
}
