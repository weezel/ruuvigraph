package ruuvi

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-ble/ble"
	"github.com/peterhellberg/ruuvitag"
)

type Data struct {
	Datetime     string  `json:"datetime"`
	Name         string  `json:"name,omitempty"`
	Alias        string  `json:"alias"`
	Address      string  `json:"address"`
	Temperature  float64 `json:"temperature,omitempty"`
	Humidity     float64 `json:"humidity,omitempty"`
	AirPressure  float64 `json:"air_pressure,omitempty"`
	BatteryVolts float64 `json:"battery_volts,omitempty"`
	SequenceNro  uint16  `json:"sequence_nro,omitempty"`
	Dbm          int     `json:"dbm,omitempty"`
}

func (d Data) MergeRuuviRaw2AndBleAdv(
	ruuviData ruuvitag.RAWv2,
	bleAdv ble.Advertisement,
	alias string,
) Data {
	return Data{
		Datetime:     time.Now().Local().Format("2006-01-02T15:04"),
		Name:         bleAdv.LocalName(),
		Alias:        alias,
		Address:      bleAdv.Addr().String(),
		Dbm:          bleAdv.RSSI(),
		Temperature:  ruuviData.Temperature,
		Humidity:     ruuviData.Humidity,
		AirPressure:  float64(ruuviData.Pressure) / 10.0,
		BatteryVolts: float64(ruuviData.Battery) / 1000.0,
		SequenceNro:  ruuviData.Sequence,
	}
}

func (d *Data) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Name: %s\n", d.Name)
	fmt.Fprintf(&sb, "Alias: %s\n", d.Alias)
	fmt.Fprintf(&sb, "Address: %s\n", d.Address)
	fmt.Fprintf(&sb, "dBm: %3d\n", d.Dbm)
	fmt.Fprintf(&sb, "Temperature: %.1f °C\n", d.Temperature)
	fmt.Fprintf(&sb, "Humidity: %.2f\n", d.Humidity)
	fmt.Fprintf(&sb, "Air pressure: %.2f Pa\n", d.AirPressure)
	fmt.Fprintf(&sb, "Battery: %.1f V\n", d.BatteryVolts)
	fmt.Fprintf(&sb, "Sequence: %d\n", d.SequenceNro)
	return sb.String()
}

// ReadAliases reads Ruuvitag aliases into memory for human friendly name mapping
func ReadAliases(filename string) (map[string]string, error) {
	file, err := os.OpenFile(filepath.Clean(filename), os.O_RDONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("file open: %w", err)
	}
	defer file.Close()

	macNameMapping := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		splt := strings.Split(strings.TrimRight(scanner.Text(), "\r\t\n"), "|")
		if len(splt) != 2 {
			fmt.Printf("malformed line: %q\n", scanner.Text())
			continue
		}
		macNameMapping[splt[0]] = splt[1]
	}

	return macNameMapping, nil
}
