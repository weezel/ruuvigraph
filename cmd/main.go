package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"weezel/ruuvigraph/pkg/logging"
	"weezel/ruuvigraph/pkg/plot"
	"weezel/ruuvigraph/pkg/ruuvi"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"
	"github.com/peterhellberg/ruuvitag"
	"github.com/pkg/errors"
)

var (
	aliasesFile      = flag.String("alias", "aliases.conf", "Aliases file for friendly names to devices")
	filterDuplicates = flag.Bool("dup", true, "Allow duplicate occurrences")
	strictMatching   = flag.Bool("s", true, "Only match devices which are listed in aliases configuration")
	sleepTime        = flag.Duration("S", 5*time.Minute, "Sleep time between the scans")
	scanPeriod       = flag.Duration("d", 10*time.Second, "Scanning time duration")
	plotFlag         = flag.String("p", "", "Only perform plotting from this filename")
)

var filename = "ruuvidata.txt"

var logger *slog.Logger = logging.NewColorLogHandler()

var aliases map[string]string

func advHandler(a ble.Advertisement) {
	if !strings.HasPrefix(a.LocalName(), "Ruuvi") {
		return
	}

	if !ruuvitag.IsRAWv2(a.ManufacturerData()) {
		return
	}

	parsedData, err := ruuvitag.ParseRAWv2(a.ManufacturerData())
	if err != nil {
		logger.Error(fmt.Sprintf("Ruuvitag parsing failed: %s", err))
		return
	}

	logger.Debug(fmt.Sprintf("Got RuuviTag beacon for MAC %x", parsedData.MAC))

	alias, found := aliases[a.Addr().String()]
	if !found && *strictMatching {
		logger.Debug(fmt.Sprintf("Ignoring tag with MAC: %x", parsedData.MAC))
		return
	}

	logger.Debug("Merging tag data")
	tagData := ruuvi.Data{}.MergeRuuviRaw2AndBleAdv(parsedData, a, alias)
	jTagData, err := json.Marshal(tagData)
	if err != nil {
		logger.Error(fmt.Sprintf("JSON marshalling failed: %s", err))
		return
	}

	logger.Info(fmt.Sprintf("Got RuuviTag beacon for %s", tagData.Alias))

	logger.Debug("Writing data to file")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to open file: %s", err))
		return
	}
	defer f.Close()
	jTagData = append(jTagData, '\n')
	if _, err = f.Write(jTagData); err != nil {
		logger.Error(fmt.Sprintf("Write to file failed: %s", err))
		return
	}
	logger.Info("Wrote data to file")

	if err = plotFromFile(filename); err != nil {
		logger.Error(fmt.Sprintf("plot: %s", err))
		return
	}
}

func plotFromFile(fname string) error {
	file, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("opening file failed: %w", err)
	}
	defer file.Close()

	var historyData []ruuvi.Data
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var rd ruuvi.Data
		if err = json.Unmarshal(scanner.Bytes(), &rd); err != nil {
			logger.Error(fmt.Sprintf("JSON unmarshal failed: %s", err))
			continue
		}
		historyData = append(historyData, rd)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner: %w", err)
	}

	return fmt.Errorf("plot: %w", plot.Plot(&historyData))
}

func chkErr(err error) {
	//nolint:errorlint // AFAIK this is okay here because it's coming from the common context
	switch errors.Cause(err) {
	case nil:
	case context.DeadlineExceeded:
		logger.Info("Done")
	case context.Canceled:
		logger.Info("Canceled")
	default:
		logger.Error(fmt.Sprintf("Error: %s", err.Error()))
	}
}

func main() {
	flag.Parse()

	als, err := ruuvi.ReadAliases(*aliasesFile)
	if err != nil {
		panic(err)
	}
	aliases = als

	if plotFlag != nil && *plotFlag != "" {
		if err = plotFromFile(*plotFlag); err != nil {
			logger.Error(fmt.Sprintf("Failed to plot from file %s: %s", *plotFlag, err))
			os.Exit(1)
		}
		return
	}

	d, err := dev.NewDevice("default")
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to attach new device: %s", err))
		os.Exit(1)
	}
	ble.SetDefaultDevice(d)

	// Scan for specified durantion, or until interrupted by user.
	for {
		logger.Info(fmt.Sprintf("Scanning for %s...", *scanPeriod))
		ctx, cancel := context.WithCancel(context.Background())
		ctx = ble.WithSigHandler(ctx, cancel)
		go func() {
			time.AfterFunc(*scanPeriod/2, func() {
				cancel()
			})
		}()
		chkErr(ble.Scan(ctx, *filterDuplicates, advHandler, nil))
		logger.Info(fmt.Sprintf("Sleeping for %s", *sleepTime))
		time.Sleep(*sleepTime)
	}
}
