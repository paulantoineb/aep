/*
Copyright (C) 2012-2014 Regents of the University of Minnesota.
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
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/ctessum/unit"
)

// Status holds information on the progress or status of a job.
type Status struct {
	// Name and Code hold information about the job
	Name, Code string

	// Status holds information about the status of the job.
	Status string

	// Progress holds information about how close the job is to completion.
	Progress float64
}

type statuses []Status

func (s statuses) Len() int           { return len(s) }
func (s statuses) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s statuses) Less(i, j int) bool { return s[i].Name < s[j].Name }

// An InventoryReport report holds information about raw inventory data.
type InventoryReport struct {
	Files []*InventoryFile
}

// AddData adds file(s) to the report.
func (ir *InventoryReport) AddData(files ...*InventoryFile) {
	ir.Files = append(ir.Files, files...)
}

// TotalsTable returns a table of the total emissions in this report, where
// the rows are the files and the columns are the pollutants, arranged
// alphabetically.
func (ir *InventoryReport) TotalsTable() Table {
	return ir.table(getTotals)
}

// DroppedTotalsTable returns a table of the total emissions in this report, where
// the rows are the files and the columns are the pollutants, arranged
// alphabetically.
func (ir *InventoryReport) DroppedTotalsTable() Table {
	return ir.table(getDroppedTotals)
}

func (ir *InventoryReport) table(df func(*InventoryFile) map[Pollutant]*unit.Unit) Table {
	t := make([][]string, len(ir.Files)+1)

	// get pollutants
	pols := make(map[string]int)
	dims := make(map[string]unit.Dimensions)
	for _, f := range ir.Files {
		for pol, val := range df(f) {
			pols[pol.Name] = 0
			if d, ok := dims[pol.Name]; !ok {
				dims[pol.Name] = val.Dimensions()
			} else {
				if !val.Dimensions().Matches(d) {
					panic(fmt.Errorf("dimensions mismatch: '%v' != '%v'", val.Dimensions(), d))
				}
			}
		}
	}
	for pol := range pols {
		t[0] = append(t[0], pol)
	}
	sort.Strings(t[0])
	t[0] = append([]string{"Group", "File"}, t[0]...)
	for i, pol := range t[0] {
		pols[pol] = i
	}
	for i, pol := range t[0] {
		if dims[pol] != nil {
			t[0][i] += fmt.Sprintf(" (%s)", dims[pol].String())
		}
	}
	for i, f := range ir.Files {
		t[i+1] = make([]string, len(pols))
		t[i+1][0], t[i+1][1] = f.Group, f.Name
		for pol, val := range df(f) {
			t[i+1][pols[pol.Name]] = fmt.Sprintf("%g", val.Value())
		}
	}

	return t
}

// A Table holds a text representation of report data.
type Table [][]string

// Tabbed creates a tab-separated table.
func (t Table) Tabbed(w io.Writer) (n int, err error) {
	ww := new(tabwriter.Writer)
	ww.Init(w, 0, 2, 0, '\t', 0)
	var nn int
	for _, l := range t {
		for _, r := range l {
			nn, err = fmt.Fprint(ww, r+"\t")
			if err != nil {
				return
			}
			n += nn
		}
		nn, err = fmt.Fprint(ww, "\n")
		if err != nil {
			return
		}
		n += nn
	}
	return
}

// SCCDescription reads a SMOKE sccdesc file, which gives descriptions
// for each SCC code. The returned data is in the form map[SCC]description.
func SCCDescription(r io.Reader) (map[string]string, error) {
	sccDesc := make(map[string]string)
	buf := bufio.NewReader(r)
	for {
		record, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return sccDesc, fmt.Errorf("In SCCdescription: %s; record: %s", err, record)
			}
		}
		// Get rid of comments at end of line.
		if i := strings.Index(record, "!"); i != -1 {
			record = record[0:i]
		}
		if record[0] != '#' {
			splitLine := strings.Split(record, ",")
			SCC := strings.Trim(splitLine[0], "\"")
			// Add zeros to 8 digit SCCs so that all SCCs are 10 digits
			// If SCC is less than 8 digits, add 2 zeros to the front and the rest to
			// the end.
			if len(SCC) == 8 {
				SCC = "00" + SCC
			} else if len(SCC) == 7 {
				SCC = "00" + SCC + "0"
			} else if len(SCC) == 6 {
				SCC = "00" + SCC + "00"
			} else if len(SCC) == 5 {
				SCC = "00" + SCC + "000"
			} else if len(SCC) == 4 {
				SCC = "00" + SCC + "0000"
			} else if len(SCC) == 3 {
				SCC = "00" + SCC + "00000"
			} else if len(SCC) == 2 {
				SCC = "00" + SCC + "000000"
			}
			sccDesc[SCC] = cleanDescription(splitLine[1])
		}
	}
	return sccDesc, nil
}

// SICDesc reads an SIC description file, which gives descriptions for each SIC code.
func (c *Context) SICDesc() (map[string]string, error) {
	sicDesc := make(map[string]string)
	var record string
	fid, err := os.Open(c.SicDesc)
	if err != nil {
		return sicDesc, errors.New("SICdesc: " + err.Error() + "\nFile= " + c.SicDesc + "\nRecord= " + record)
	}
	defer fid.Close()
	buf := bufio.NewReader(fid)
	for {
		record, err = buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			} else {
				return sicDesc, errors.New(c.SicDesc + "\n" + record + "\n" + err.Error() + "\nFile= " + c.SicDesc + "\nRecord= " + record)
			}
		}
		if record[0] != '#' {
			sicDesc[strings.Trim(record[0:4], " ")] =
				cleanDescription(record[5:])
		}
	}
	return sicDesc, err
}

// NAICSDesc reads a NAICS description file, which gives descriptions for each NAICS code.
func (c *Context) NAICSDesc() (map[string]string, error) {
	naicsDesc := make(map[string]string)
	var record string
	fid, err := os.Open(c.NaicsDesc)
	if err != nil {
		return naicsDesc, errors.New("NAICSdesc: " + err.Error() + "\nFile= " + c.NaicsDesc + "\nRecord= " + record)
	}
	defer fid.Close()
	buf := bufio.NewReader(fid)
	for {
		record, err = buf.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				err = nil
				break
			} else {
				return naicsDesc, errors.New(record + "\n" + err.Error() + "\nFile= " + c.NaicsDesc + "\nRecord= " + record)
			}
		}
		if record[0] != '#' {
			splitLine := strings.Split(record, ",")
			naicsDesc[strings.Trim(splitLine[0], "\"")] =
				cleanDescription(splitLine[1])
		}
	}
	return naicsDesc, err
}

func cleanDescription(d string) string {
	return "\"" + strings.Replace(strings.Trim(d, "\n"), "\"", "", -1) + "\""
}
<<<<<<< HEAD

// HTML report server

type htmlData struct {
	PageName      string
	IncludeStatus bool
	IsConfigure   bool
	IsStatus      bool
	IsInv         bool
	IsSpec        bool
	IsSpatial     bool
	IsTemporal    bool
}

func newHtmlData() *htmlData {
	data := new(htmlData)
	if !Report.ReportOnly {
		data.IncludeStatus = true
	}
	return data
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	pageData := newHtmlData()
	pageData.PageName = "Configuration"
	pageData.IsConfigure = true
	renderHeaderFooter(w, "header", pageData)
	renderHeaderFooter(w, "nav", pageData)
	renderBodyTemplate(w, "config")
	renderHeaderFooter(w, "footer", pageData)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	if Report.ReportOnly {
		http.Redirect(w, r, "/config", http.StatusFound)
	}
	pageData := newHtmlData()
	pageData.PageName = "Status"
	pageData.IsStatus = true
	renderHeaderFooter(w, "header", pageData)
	renderHeaderFooter(w, "nav", pageData)
	renderStatusTemplate(w)
	renderHeaderFooter(w, "footer", pageData)
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	pageData := newHtmlData()
	pageData.PageName = "Inventory"
	pageData.IsInv = true
	renderHeaderFooter(w, "header", pageData)
	renderHeaderFooter(w, "nav", pageData)
	const body1 = `
<div class="container">
	<h4>Inventory</h4>
	<p>(See file Report.json in the log directory for more detail)</p>
	<div class="row">
		<div class="span12">
			<h5>Kept pollutants</h5>
			<p>These are the emissions that go on to the next modeling step, as specified in the PolsToKeep setting in the configuration file.</p>`
	fmt.Fprint(w, body1)
	dr := Report.PrepDataReport("Inventory", "Totals")
	DrawTable("%.4g", true, dr, w)
	const body2 = `
		</div>
	</div>
	<div class="row">
		<div class="span12">
			<h5>Dropped pollutants</h5>
			<p>These are the pollutants that are not specified in the PolsToKeep setting in the configuration file.</p>`
	fmt.Fprint(w, body2)
	dr = Report.PrepDataReport("Inventory", "DroppedTotals")
	DrawTable("%.4g", true, dr, w)
	const body3 = `
		</div>
	</div>
</div>`
	fmt.Fprint(w, body3)
	renderHeaderFooter(w, "footer", pageData)
}

func speciateHandler(w http.ResponseWriter, r *http.Request) {
	pageData := newHtmlData()
	pageData.PageName = "Speciation"
	pageData.IsSpec = true
	renderHeaderFooter(w, "header", pageData)
	renderHeaderFooter(w, "nav", pageData)
	const body1 = `
<div class="container">
	<h4>Speciation</h4>
	<p>(See file Report.json in the log directory for more detail)</p>
	<div class="row">
		<div class="span12">
			<h5>Kept pollutants</h5>
			<p>These are the emissions that go on to the next modeling step.</p>`
	fmt.Fprint(w, body1)
	dr := Report.PrepDataReport("Speciation", "Kept")
	DrawTable("%.4g", true, dr, w)
	const body2 = `
		</div>
	</div>
	<div class="row">
		<div class="span12">
			<h5>Pollutants dropped due to double counting</h5>
			<p>When there is a general species group (like VOCs) that is speciated into specific chemicals, but some of the specific chemicals are also explicitely tracked in the inventory, there is a possibility for double counting. So for records that include both the general species group and specific data for some of its components the specific components that are explicitly included in the inventory are dropped from the speciation of the general group.</p>`
	fmt.Fprint(w, body2)
	dr = Report.PrepDataReport("Speciation", "DoubleCounted")
	DrawTable("%.4g", true, dr, w)
	const body3 = `
		</div>
	</div>
	<div class="row">
		<div class="span12">
			<h5>Pollutants dropped due to lack of a species group</h5>
			<p>The individual chemicals are lumped into groups for representation in air quality model chemical mechanisms. These are the emissions that are speciated from general groups (such as VOCs) but do not fit into any of the specified groups.</p>`
	fmt.Fprint(w, body3)
	dr = Report.PrepDataReport("Speciation", "Ungrouped")
	DrawTable("%.4g", true, dr, w)
	const body4 = `
		</div>
	</div>
</div>`
	fmt.Fprint(w, body4)
	renderHeaderFooter(w, "footer", pageData)
}

func spatialHandler(w http.ResponseWriter, r *http.Request) {
	pageData := newHtmlData()
	pageData.PageName = "Gridding"
	pageData.IsSpatial = true
	renderHeaderFooter(w, "header", pageData)
	renderHeaderFooter(w, "nav", pageData)
	const body1 = `
<div class="container">
	<h4>Gridding</h4>
	<p>(See file Report.json in the log directory for more detail)</p>`
	fmt.Fprint(w, body1)
	const body2 = `
	<div class="row">
		<div class="span12">`
	const body3 = `
		</div>
	</div>`

	for _, grid := range Report.GridNames {
		fmt.Fprint(w, body2)
		fmt.Fprintf(w, "<h5>Fraction of emissions within domain %v</h5>\n", grid)
		dr := Report.PrepDataReport("Spatialization", grid)
		DrawTable("%.3g%%", false, dr, w)
		fmt.Fprint(w, body3)
	}
	fmt.Fprint(w, "\n</div")
	renderHeaderFooter(w, "footer", pageData)
}

func temporalHandler(w http.ResponseWriter, r *http.Request) {
	pageData := newHtmlData()
	pageData.PageName = "Temporal"
	pageData.IsTemporal = true
	renderHeaderFooter(w, "header", pageData)
	renderHeaderFooter(w, "nav", pageData)
	const body1 = `
<div class="container">
	<h4>Temporal</h4>
	<p>(See file Report.json in the log directory for more detail)</p>`
	fmt.Fprint(w, body1)
	const body2 = `
	<div class="row">
		<div class="span12">`
	const body3 = `
		</div>
	</div>`

	for _, grid := range Report.GridNames {
		fmt.Fprint(w, body2)
		fmt.Fprintf(w, "<h5>Fraction of spatial emissions that have been "+
			"temporalized %v</h5>\n", grid)
		dr := Report.PrepDataReport("Temporalization", grid)
		DrawTable("%.3g%%", false, dr, w)
		fmt.Fprint(w, body3)
	}
	fmt.Fprint(w, "\n</div")
	renderHeaderFooter(w, "footer", pageData)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if Report.ReportOnly {
		http.Redirect(w, r, "/config", http.StatusFound)
	} else {
		http.Redirect(w, r, "/status", http.StatusFound)
	}
}

/*func init() {
	pkg, err := build.Import("github.com/ctessum/aep", "", build.FindOnly)
	if err != nil {
		panic(err)
	}
	reportfiles = filepath.Join(pkg.SrcRoot, pkg.ImportPath, "reportfiles")
	template.Must(templates.Funcs(
		template.FuncMap{"class": TableClass}).ParseFiles(
		filepath.Join(reportfiles, "config.html"),
		filepath.Join(reportfiles, "status.html"),
		filepath.Join(reportfiles, "header.html"),
		filepath.Join(reportfiles, "nav.html"),
		filepath.Join(reportfiles, "footer.html")))
}*/

var (
	templates   = template.New("x")
	reportfiles = ""
)

func TableClass(in string) string {
	if strings.Index(in, "Running") >= 0 {
		return "warning"
	} else if in == "Failed!" {
		return "error"
	} else if in == "Finished" {
		return "success"
	} else if in == "Ready" {
		return "success"
	} else if in == "Generating" {
		return "warning"
	} else if in == "Waiting to generate" {
		return "info"
	} else {
		return "info"
	}
}

func renderBodyTemplate(w http.ResponseWriter, tmpl string) {
	err := templates.ExecuteTemplate(w, tmpl+".html", Report)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
func renderStatusTemplate(w http.ResponseWriter) {
	//Status.Lock.Lock()
	//Status.SrgProgress = SrgProgress
	//Status.Lock.Unlock()
}

func renderHeaderFooter(w http.ResponseWriter, tmpl string, data *htmlData) {
	err := templates.ExecuteTemplate(w, tmpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type dataReport struct {
	sectors, pols []string
	units         map[string]string
	data          map[string]map[string]float64
	polTotals     map[string]float64
}

func (dr *dataReport) Len() int { return len(dr.pols) }
func (dr *dataReport) Swap(i, j int) {
	dr.pols[i], dr.pols[j] = dr.pols[j], dr.pols[i]
}
func (dr *dataReport) Less(i, j int) bool {
	// descending order
	return dr.polTotals[dr.pols[i]] > dr.polTotals[dr.pols[j]]
}

func (dr *dataReport) sortByTotals() {
	dr.polTotals = make(map[string]float64)
	for _, d1 := range dr.data {
		for pol, d2 := range d1 {
			dr.polTotals[pol] += d2
		}
	}
	sort.Sort(dr)
}

// Organize data in report for making a table or plot.
// If procType="Spatializaton", then countType is actually the domain
// (d01, d02 etc).
func (r *ReportHolder) PrepDataReport(procType, countType string) *dataReport {

	dr := new(dataReport)
	dr.sectors = make([]string, 0)
	dr.pols = make([]string, 0)
	dr.units = make(map[string]string, 0)
	dr.data = make(map[string]map[string]float64)

	switch procType {
	case "Temporalization":
		if r.TemporalResults != nil {
			// Calculate total spatialized emissions
			spatialData := make(map[string]float64)
			for _, sectorDataPeriod := range r.SectorResults {
				numPeriods := float64(len(sectorDataPeriod))
				for _, sectorData := range sectorDataPeriod {
					for pol, inData := range sectorData.SpatialResults.
						InsideDomainTotals[countType] {
						if !IsStringInArray(dr.pols, pol.Name) {
							dr.pols = append(dr.pols, pol.Name)
							dr.units[pol.Name] = inData.Units
						}
						// Total emissions inside the domain
						spatialData[pol.Name] += inData.Val / numPeriods
					}
				}
			}
			dr.sectors = append(dr.sectors, "total")
			dr.data["total"] = make(map[string]float64)
			r.TemporalResults.mu.Lock()
			for pol, d := range r.TemporalResults.Data {
				tData := d[countType]
				// Percent of spatialized emissions that have been temporalized
				dr.data["total"][pol] = floats.Sum(tData) / spatialData[pol] * 100.
			}
			r.TemporalResults.mu.Unlock()
		}
	default:
		for sector, sectorDataPeriod := range r.SectorResults {
			dr.data[sector] = make(map[string]float64)
			dr.sectors = append(dr.sectors, sector)
			numPeriods := float64(len(sectorDataPeriod))
			for _, sectorData := range sectorDataPeriod {
				switch procType {
				case "Speciation":
					if sectorData.SpeciationResults != nil {
						switch countType {
						case "Kept":
							for pol, poldata := range sectorData.
								SpeciationResults.Kept {
								if !IsStringInArray(dr.pols, pol) {
									dr.pols = append(dr.pols, pol)
									dr.units[pol] = poldata.Units
								}
								dr.data[sector][pol] += poldata.Val / numPeriods
							}
						case "DoubleCounted":
							for pol, poldata := range sectorData.SpeciationResults.
								DoubleCounted {
								if !IsStringInArray(dr.pols, pol) {
									dr.pols = append(dr.pols, pol)
									dr.units[pol] = poldata.Units
								}
								dr.data[sector][pol] += poldata.Val / numPeriods
							}
						case "Ungrouped":
							for pol, poldata := range sectorData.SpeciationResults.
								Ungrouped {
								if !IsStringInArray(dr.pols, pol) {
									dr.pols = append(dr.pols, pol)
									dr.units[pol] = poldata.Units
								}
								dr.data[sector][pol] += poldata.Val / numPeriods
							}
						}
					}
				case "Spatialization":
					if sectorData.SpatialResults != nil {
						for pol, inData := range sectorData.SpatialResults.
							InsideDomainTotals[countType] {
							outData := sectorData.SpatialResults.
								OutsideDomainTotals[countType][pol]
							if !IsStringInArray(dr.pols, pol.Name) {
								dr.pols = append(dr.pols, pol.Name)
								dr.units[pol.Name] = inData.Units
							}
							// Fraction inside the domain
							dr.data[sector][pol.Name] += inData.Val /
								(inData.Val + outData.Val) *
								100. / numPeriods
						}
					}
				}
			}
		}
	}
	sort.Strings(dr.pols)
	sort.Strings(dr.sectors)
	return dr
}

func DrawTable(format string, includeTotals bool, dr *dataReport,
	w io.Writer) {
	if includeTotals {
		dr.sortByTotals()
	}
	fmt.Fprint(w, "<table class=\"table table-striped\">\n<thead>\n<tr><td></td>")
	for _, pol := range dr.pols {
		fmt.Fprintf(w, "<td>%v</td>", pol)
	}
	fmt.Fprint(w, "</tr>\n<tr><td></td>")
	for _, pol := range dr.pols {
		fmt.Fprintf(w, "<td>%v</td>", dr.units[pol])
	}
	fmt.Fprint(w, "</tr></thead>\n<tbody>\n")
	for _, sector := range dr.sectors {
		fmt.Fprintf(w, "<tr><td>%v</td>", sector)
		for _, pol := range dr.pols {
			fmt.Fprintf(w, "<td>"+format+"</td>", dr.data[sector][pol])
		}
		fmt.Fprint(w, "</tr>\n")
	}
	if includeTotals {
		fmt.Fprint(w, "<tr><td><strong>Totals</strong></td>")
		for _, pol := range dr.pols {
			fmt.Fprintf(w, "<td><strong>"+format+"</strong></td>",
				dr.polTotals[pol])
		}
	}
	fmt.Fprint(w, "</tbody></table>\n")
}

func (c *Context) ReportServer(reportOnly bool) {
	// read in the report
	if reportOnly {
		file := filepath.Join(c.outputDir, "Report.json")
		f, err := os.Open(file)
		if err != nil {
			panic(err)
		}
		reader := bufio.NewReader(f)
		bytes, err := ioutil.ReadAll(reader)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(bytes, Report)
		if err != nil {
			panic(err)
		}
		Report.ReportOnly = reportOnly
	}
	Report.ReportOnly = reportOnly
	Report.Config = c
	http.Handle("/css/", http.FileServer(http.Dir(reportfiles)))
	http.Handle("/js/", http.FileServer(http.Dir(reportfiles)))
	http.HandleFunc("/inventory", inventoryHandler)
	http.HandleFunc("/speciate", speciateHandler)
	http.HandleFunc("/spatial", spatialHandler)
	http.HandleFunc("/temporal", temporalHandler)
	http.HandleFunc("/config", configHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/", rootHandler)
	http.ListenAndServe(":6060", nil)
}

// Write out the report
func (c *Context) WriteReport() {
	b, err := json.MarshalIndent(Report, "", "  ")
	if err != nil {
		panic(err)
	}
	f, err := os.Create(filepath.Join(
		c.outputDir, "Report.json"))
	if err != nil {
		panic(err)
	}
	_, err = f.Write(b)
	if err != nil {
		panic(err)
	}
	f.Close()
}
