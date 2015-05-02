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

package main

import (
	"bitbucket.org/ctessum/aep/lib.aep"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strings"
	//"time"
)

func main() {
	log.Println("\n",
		"----------------------------------------------------------\n",
		"                        Welcome!\n",
		"             Program (A)ir (E)missions (P)rocessor\n",
		"Copyright 2012-2014 Regents of the University of Minnesota\n",
		"----------------------------------------------------------\n")

	// Read from configuration file and prepare sectors for processing
	var sectorFlag *string = flag.String("sectors", "all", "List of sectors to process, in quotes, separated by spaces")
	var configFile *string = flag.String("config", "none", "Path to configuration file")
	var reportOnly *bool = flag.Bool("reportonly", false, "Run html report server for results of previous run (do not calculate new results)")
	var testmode *bool = flag.Bool("testmode", false, "Run model with mass speciation and no VOC to TOG conversion so that results can be validated by go test")
	var seq *bool = flag.Bool("seq", false, "Run sectors in sequence instead of parallel to conserve memory")
	var slavesFlag *string = flag.String("slaves", "", "List of addresses of available slaves, in quotes, separated by spaces.")
	var masterAddress *string = flag.String("masteraddress", "", "What is the address of the master? Leave empty if this is not a slave.")
	flag.Parse()

	if *configFile == "none" {
		fmt.Println("Please set `-config' flag and run again: ie: aep -config=/path/to/config_file")
		fmt.Println("For more information try typing `aep --help'")
		return
	}
	e := new(aep.ErrCat) // error report

	// create list of sectors
	sectors := strings.Split(*sectorFlag, " ")

	// Are we doing distributed computing,
	// and is this the master?
	var slaves []string
	if *slavesFlag != "" {
		// create list of slaves
		slaves = strings.Split(*slavesFlag, " ")
	}

	// parse configuration file
	ConfigAll := aep.ReadConfigFile(configFile, testmode, slaves, e)

	runtime.GOMAXPROCS(ConfigAll.DefaultSettings.Ncpus)

	if *masterAddress != "" {
		// Set up a server to accept RPC requests;
		// this will block and run forever.
		aep.DistributedServer(ConfigAll.DefaultSettings)
	}

	// go to localhost:6060 in web browser to view report
	if *reportOnly {
		// The reportServer function will run forever.
		ConfigAll.DefaultSettings.ReportServer(*reportOnly)
	} else {
		// The reportServer function will run until the program is finished.
		go ConfigAll.DefaultSettings.ReportServer(*reportOnly)
	}
	defer ConfigAll.DefaultSettings.WriteReport()

	// Start server for retrieving profiles from the
	// SPECIATE database
	if ConfigAll.DefaultSettings.RunSpeciate {
		go ConfigAll.DefaultSettings.SpecProfiles(e)
	}

	// Set up spatial environment
	if ConfigAll.DefaultSettings.RunSpatialize {
		ConfigAll.DefaultSettings.SpatialSetupRegularGrid(e)
	}

	// Set up temporal and output processors
	var temporal *aep.TemporalProcessor
	if ConfigAll.DefaultSettings.RunTemporal {
		temporal = ConfigAll.DefaultSettings.NewTemporalProcessor()
	}

	e.Report() // Print errors, if any

	runChan := make(chan string, 1)
	if *seq { // run sectors in sequence to conserve memory
		for sector, c := range ConfigAll.Sectors {
			if sectors[0] == "all" || aep.IsStringInArray(sectors, sector) {
				go Run(c, runChan, temporal)
				message := <-runChan
				log.Println(message)
			}
		}
	} else { // run sectors in parallel
		// run sector subroutines
		n := 0
		for sector, c := range ConfigAll.Sectors {
			if sectors[0] == "all" || aep.IsStringInArray(sectors, sector) {
				go Run(c, runChan, temporal)
				n++
			}
		}
		// wait for calculations to complete
		for i := 0; i < n; i++ {
			message := <-runChan
			log.Println(message)
		}
	}

	// run output subroutines
	if ConfigAll.DefaultSettings.RunTemporal {
		outputter := ConfigAll.DefaultSettings.NewOutputter(temporal)
		outputter.Output()
	}

	log.Println("\n",
		"------------------------------------\n",
		"           AEP Completed!\n",
		"   Check above for error messages.\n",
		"------------------------------------\n")
}

func Run(c *aep.Context, runChan chan string,
	temporal *aep.TemporalProcessor) {

	log.Println("Running " + c.Sector + "...")
	aep.Status.Sectors[c.Sector] = "Running"
	msgchan := c.MessageChan()
	n := 0 // number of subroutines that we need to wait to finish

	discardChan := make(chan *aep.ParsedRecord)
	go DiscardRecords(discardChan)

	// only run inventory
	if c.RunSpeciate == false && c.RunSpatialize == false {
		go c.Inventory(discardChan)
		n++
	}

	// speciate but don't spatialize
	if c.RunSpeciate == true && c.RunSpatialize == false {
		ChanFromInventory := make(chan *aep.ParsedRecord, 1)
		go c.Inventory(ChanFromInventory)
		go c.Speciate(ChanFromInventory, discardChan)
		n += 2
	}

	// speciate and spatialize
	if c.RunSpeciate == true && c.RunSpatialize == true {
		ChanFromInventory := make(chan *aep.ParsedRecord, 1)
		go c.Inventory(ChanFromInventory)
		SpecSpatialChan := make(chan *aep.ParsedRecord, 1)
		go c.Speciate(ChanFromInventory, SpecSpatialChan)
		if c.RunTemporal { // only run temporal if spatializing
			SpatialTemporalChan := make(chan *aep.ParsedRecord, 1)
			go c.Spatialize(SpecSpatialChan, SpatialTemporalChan)
			temporal.NewSector(c, SpatialTemporalChan, discardChan)
			n++
		} else {
			go c.Spatialize(SpecSpatialChan, discardChan)
		}
		n += 3
	}
	// spatialize but don't speciate
	if c.RunSpeciate == false && c.RunSpatialize == true {
		ChanFromInventory := make(chan *aep.ParsedRecord, 1)
		go c.Inventory(ChanFromInventory)
		if c.RunTemporal { // only run temporal if spatializing
			SpatialTemporalChan := make(chan *aep.ParsedRecord, 1)
			go c.Spatialize(ChanFromInventory, SpatialTemporalChan)
			temporal.NewSector(c, SpatialTemporalChan, discardChan)
			n++
		} else {
			go c.Spatialize(ChanFromInventory, discardChan)
		}
		n += 2
	}

	// wait for calculations to complete
	for i := 0; i < n; i++ {
		message := <-msgchan
		c.Log(message, 1)
	}

	//aep.Status.Lock.Lock()
	if aep.Status.Sectors[c.Sector] != "Failed!" {
		aep.Status.Sectors[c.Sector] = "Finished"
	}
	//aep.Status.Lock.Unlock()
	runChan <- "Finished processing " + c.Sector
}

func DiscardRecords(inputChan chan *aep.ParsedRecord) {
	for _ = range inputChan {
		continue
	}
}