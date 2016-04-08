package aep

import (
	"fmt"
	"math"

	"github.com/ctessum/geom"
	"github.com/ctessum/geom/proj"
	"github.com/ctessum/unit"
	"github.com/ctessum/unit/badunit"
)

// PointSourceData holds information specific to point emissions sources.
type PointSourceData struct {
	// PlantID is the Plant Identification Code (15 characters maximum) (required,
	// this is the same as the State Facility Identifier in the NIF)
	PlantID string

	// PointID is the Point Identification Code (15 characters maximum) (required,
	// this is the same as the Emission Unit ID in the NIF)
	PointID string

	// StackID is the Stack Identification Code (15 characters maximum) (recommended,
	// this is the same as the Emissions Release Point ID in the NIF)
	StackID string

	// Segement is the DOE Plant ID (15 characters maximum) (recommended, this is the
	// same as the Process ID in the NIF)
	Segment string

	// Plant Name (40 characters maximum) (recommended)
	Plant string

	// DOE Plant ID (generally recommended, and required if matching
	// to hour-specific CEM data)
	ORISFacilityCode string

	// Boiler Identification Code (recommended)
	ORISBoilerID string

	// Stack Height (m) (required)
	StackHeight *unit.Unit

	// Stack Diameter (m) (required)
	StackDiameter *unit.Unit

	// Stack Gas Exit Temperature (K) (required)
	StackTemp *unit.Unit

	// Stack Gas Flow Rate (m3/sec) (optional)
	StackFlow *unit.Unit

	// Stack Gas Exit Velocity (m/sec) (required)
	StackVelocity *unit.Unit

	// Point holds the location of the emissions source
	geom.Point

	// SR holds the spatial reference system of the coordinates of this point
	proj.SR
}

var longlat, nad27, nad83 proj.SR

func init() {
	var err error
	longlat, err = proj.FromProj4("+proj=longlat")
	if err != nil {
		panic(err)
	}
	nad27, err = proj.FromProj4("+proj=longlat +datum=NAD27")
	if err != nil {
		panic(err)
	}
	nad83, err = proj.FromProj4("+proj=longlat +datum=NAD83")
	if err != nil {
		panic(err)
	}

}

func (r *PointSourceData) setupLocation(xloc, yloc, ctype, utmz, datum string) error {
	if ctype != "L" {
		return fmt.Errorf("ctype needs to equal `L'. It is instead `%v'",
			ctype)
	}

	switch trimString(datum) {
	case "", "003":
		r.SR = longlat
	case "001": // NAD27
		r.SR = nad27
	case "002": // NAD83
		r.SR = nad83
	default:
		return fmt.Errorf("aep.PointSourceData.setupLocation: invalid datum '%s'", datum)
	}

	x, err := stringToFloat(xloc)
	if err != nil {
		return err
	}
	y, err := stringToFloat(yloc)
	if err != nil {
		return err
	}
	r.Point = geom.Point{X: x, Y: y}
	return nil
}

func circleArea(d *unit.Unit) *unit.Unit {
	return unit.Div(unit.Mul(d, d), unit.New(4/math.Pi, unit.Dimless))
}

func (r *PointSourceData) fixStack() {
	if r.StackVelocity.Value() == 0 && r.StackFlow.Value() != 0 && r.StackDiameter.Value() != 0 {
		r.StackVelocity = unit.Div(r.StackFlow, circleArea(r.StackDiameter))
	} else if r.StackVelocity.Value() != 0 && r.StackFlow.Value() == 0 {
		r.StackFlow = unit.Mul(r.StackVelocity, circleArea(r.StackDiameter))
	}
}

// Key returns a unique key for this record.
func (r *PointSourceData) Key() string {
	return r.PlantID + r.PointID + r.StackID + r.Segment
}

func (r *PointSourceData) setStackParams(height, diam, temp, flow, vel string) error {
	sh, err := stringToFloat(height)
	if err != nil {
		return err
	}
	r.StackHeight = badunit.Foot(sh)

	d, err := stringToFloat(diam)
	if err != nil {
		return err
	}
	r.StackDiameter = badunit.Foot(d)

	t, err := stringToFloat(temp)
	if err != nil {
		return err
	}
	r.StackTemp = badunit.Fahrenheit(t)

	f, err := stringToFloat(flow)
	if err != nil {
		return err
	}
	r.StackFlow = badunit.Foot3PerSecond(f)

	v, err := stringToFloat(vel)
	if err != nil {
		return err
	}
	r.StackVelocity = badunit.FootPerSecond(v)

	r.fixStack()
	return nil
}

// PointData returns the data specific to point sources.
func (r *PointSourceData) PointData() *PointSourceData {
	return r
}
