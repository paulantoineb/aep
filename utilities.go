/*
Copyright (C) 2012 the AEP authors.
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

package aep

import (
	"fmt"
	"math"
	"strings"
	"time"

	"bitbucket.org/ctessum/sparse"

	"github.com/ctessum/unit"
)

// MatchCode finds code in matchmap.
// For cases where a specific code needs to be matched with a more
// general code. For instance, if code is "10101" and matchmap is
// {"10102":"xxx","10100":"yyyy"}, "10100" will be returned as the
// closest match to the input code. "10102" will never be returned,
// even if the "10100" item didn't exist. Returns an error if there
// is no match.
func MatchCode(code string, matchmap map[string]interface{}) (matchedCode string, matchVal interface{}, err error) {
	var ok bool
	l := len(code)
	for i := l - 1; i >= -1; i-- {
		matchedCode = code[0:i+1] + strings.Repeat("0", l-i-1)
		matchVal, ok = matchmap[matchedCode]
		if ok {
			return
		}
	}
	err = fmt.Errorf("No matching code for %v", code)
	return
}

// MatchCodeDouble finds code1 and code2 in matchmap.
func MatchCodeDouble(code1, code2 string, matchmap map[string]map[string]interface{}) (matchedCode1, matchedCode2 string, matchVal interface{}, err error) {
	l1 := len(code1)
	l2 := len(code2)
	for i := l1 - 1; i >= -1; i-- {
		matchedCode1 = code1[0:i+1] + strings.Repeat("0", l1-i-1)
		match1, ok := matchmap[matchedCode1]
		if ok {
			for i := l2 - 1; i >= -1; i-- {
				matchedCode2 = code2[0:i+1] + strings.Repeat("0", l2-i-1)
				matchVal, ok = match1[matchedCode2]
				if ok {
					return
				}
			}
		}
	}
	err = fmt.Errorf("No matching codes for %v, %v", code1, code2)
	return
}

// IsStringInArray returns whether s is a member of a.
func IsStringInArray(a []string, s string) bool {
	for _, val := range a {
		if val == s {
			return true
		}
	}
	return false
}

func absBias(a, b float64) (o float64) {
	o = math.Abs(a-b) / b
	return
}

// Country represents a country where emissions occur.
type Country int

// These are the currently supported countries.
const (
	USA               Country = 0
	Canada                    = 1
	Mexico                    = 2
	Cuba                      = 3
	Bahamas                   = 4
	Haiti                     = 5
	DominicanRepublic         = 6
	Unknown                   = -1
)

func (c Country) String() string {
	switch c {
	case USA:
		return "USA"
	case Canada:
		return "CA"
	case Mexico:
		return "MEXICO"
	case Cuba:
		return "CUBA"
	case Bahamas:
		return "BAHAMAS"
	case Haiti:
		return "HAITI"
	case DominicanRepublic:
		return "DOMINICANREPUBLIC"
	default:
		panic(fmt.Errorf("Unknown country %d", c))
	}
}

func getCountryCode(country Country) string {
	return fmt.Sprintf("%d", country)
}
func getCountryFromID(code string) Country {
	switch code {
	case "0":
		return USA
	case "1":
		return Canada
	case "2":
		return Mexico
	case "3":
		return Cuba
	case "4":
		return Bahamas
	case "5":
		return Haiti
	case "6":
		return DominicanRepublic
	default:
		err := fmt.Errorf("Unknown country code %v.", code)
		panic(err)
	}
}

func countryFromName(name string) (Country, error) {
	switch name {
	case "USA", "US":
		return USA, nil
	case "CANADA", "CA", "CAN":
		return Canada, nil
	case "MEXICO", "MEX", "MX":
		return Mexico, nil
	case "CUBA":
		return Cuba, nil
	case "BAHAMAS":
		return Bahamas, nil
	case "HAITI":
		return Haiti, nil
	case "DOMINICANREPUBLIC":
		return DominicanRepublic, nil
	default:
		return Unknown, fmt.Errorf("Unkown country '%s'", name)
	}
}

// EmissionsTotal returns the total combined emissions in recs.
func EmissionsTotal(recs []Record) map[Pollutant]*unit.Unit {
	o := make(map[Pollutant]*unit.Unit)
	for _, r := range recs {
		t := r.Totals()
		for p, e := range t {
			if o[p] == nil {
				o[p] = e.Clone()
			} else {
				o[p].Add(e)
			}
		}
	}
	return o
}

// EmissionsGriddedAtTime returns the sum of the emissions in recs at
// the given time.
func EmissionsGriddedAtTime(recs []Record, t time.Time, o Outputter, sp *SpatialProcessor, tp *TemporalProcessor, partialMatch bool) (map[Pollutant][]*sparse.SparseArray, error) {
	out := make(map[Pollutant][]*sparse.SparseArray)

	for _, r := range recs {
		emis, err := EmisAtTime(r, t, tp, partialMatch)
		if err != nil {
			return nil, err
		}
		for gi := range sp.Grids {
			// Get the vertical layer index.
			var k int
			switch r.(type) {
			case PointSource:
				k, err = o.PlumeRise(r.(PointSource), sp, gi)
				if err != nil {
					return nil, err
				}
			}

			// Get the spatial surrogate.
			srg, _, inGrid, err := r.Spatialize(sp, gi)
			if err != nil {
				return nil, err
			}
			if !inGrid {
				continue
			}
			for pol, val := range emis {
				if _, ok := out[pol]; !ok {
					// Initialize arrays for this pollutant
					out[pol] = make([]*sparse.SparseArray, len(sp.Grids))
					for i, grid := range sp.Grids {
						out[pol][i] = sparse.ZerosSparse(o.Layers(), grid.Ny, grid.Nx)
					}
				}
				for srgi, srgVal := range srg.Elements {
					indices := srg.IndexNd(srgi)
					out[pol][gi].AddVal(val.Value()*srgVal, k, indices[0], indices[1])
				}
			}
		}
	}
	return out, nil
}
