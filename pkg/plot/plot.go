package plot

import (
	"fmt"
	"io"
	"os"
	"time"

	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

const outHTMLFilename = "sensor_data.html"

func getTemperatures(data []*ruuvipb.RuuviStreamDataRequest) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range data {
		items = append(items, opts.LineData{
			Name:  d.Device,
			Value: fmt.Sprintf("%.2f", d.Temperature),
		})
	}
	return items
}

func getHumidity(data []*ruuvipb.RuuviStreamDataRequest) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range data {
		items = append(items, opts.LineData{
			Name:  d.Device,
			Value: fmt.Sprintf("%.2f", d.Humidity),
		})
	}
	return items
}

func getPressure(data []*ruuvipb.RuuviStreamDataRequest) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range data {
		var val float64
		if d.Pressure > 1000 { // TODO Remove from the final version
			val = float64(d.Pressure) / 10.0
		}
		items = append(items, opts.LineData{
			Name:  d.Device,
			Value: fmt.Sprintf("%.2f", val),
		})
	}
	return items
}

func dateRange(data []*ruuvipb.RuuviStreamDataRequest) []string {
	items := []string{}
	for _, d := range data {
		items = append(items, d.Timestamp.AsTime().String())
	}
	return items
}

//nolint:dupl // Okay for now
func plotTemperature(data []*ruuvipb.RuuviStreamDataRequest) *charts.Line {
	plotGraph := charts.NewLine()
	plotGraph.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Min: 19.0,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Temperature",
			Subtitle: time.Now().Format(time.RFC3339),
		}),
		charts.WithAnimation(*opts.Bool(true)),
	)

	m := map[string][]*ruuvipb.RuuviStreamDataRequest{}
	for _, event := range data {
		if _, found := m[event.Device]; !found {
			m[event.Device] = []*ruuvipb.RuuviStreamDataRequest{}
		}
		tmp := m[event.Device]
		tmp = append(tmp, event)
		m[event.Device] = tmp
	}

	for Device, values := range m {
		plotGraph.SetXAxis(dateRange(data)).
			AddSeries(Device, getTemperatures(values)).
			SetSeriesOptions(
				charts.WithLineChartOpts(
					opts.LineChart{
						Smooth:     opts.Bool(false),
						ShowSymbol: opts.Bool(true),
					},
				),
			)
	}

	return plotGraph
}

func plotHumidity(data []*ruuvipb.RuuviStreamDataRequest) *charts.Line {
	plotGraph := charts.NewLine()
	plotGraph.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		// charts.WithYAxisOpts(opts.YAxis{
		// 	Min: 19.0,
		// }),
		charts.WithTitleOpts(opts.Title{
			Title:    "Humidity",
			Subtitle: time.Now().Format(time.RFC3339),
		}),
		charts.WithAnimation(*opts.Bool(true)),
	)

	m := map[string][]*ruuvipb.RuuviStreamDataRequest{}
	for _, event := range data {
		if _, found := m[event.Device]; !found {
			m[event.Device] = []*ruuvipb.RuuviStreamDataRequest{}
		}
		tmp := m[event.Device]
		tmp = append(tmp, event)
		m[event.Device] = tmp
	}

	for Device, values := range m {
		plotGraph.SetXAxis(dateRange(data)).
			AddSeries(Device, getHumidity(values)).
			SetSeriesOptions(
				charts.WithLineChartOpts(
					opts.LineChart{
						Smooth:     opts.Bool(false),
						ShowSymbol: opts.Bool(true),
					},
				),
			)
	}

	return plotGraph
}

//nolint:dupl // Okay for now
func plotPressure(data []*ruuvipb.RuuviStreamDataRequest) *charts.Line {
	plotGraph := charts.NewLine()
	plotGraph.SetGlobalOptions(
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Min: 900.0,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Air pressure",
			Subtitle: time.Now().Format(time.RFC3339),
		}),
		charts.WithAnimation(true),
	)

	m := map[string][]*ruuvipb.RuuviStreamDataRequest{}
	for _, event := range data {
		if _, found := m[event.Device]; !found {
			m[event.Device] = []*ruuvipb.RuuviStreamDataRequest{}
		}
		tmp := m[event.Device]
		tmp = append(tmp, event)
		m[event.Device] = tmp
	}

	for Device, values := range m {
		plotGraph.SetXAxis(dateRange(data)).
			AddSeries(Device, getPressure(values)).
			SetSeriesOptions(
				charts.WithLineChartOpts(
					opts.LineChart{
						Smooth:     opts.Bool(false),
						ShowSymbol: opts.Bool(true),
					},
				),
			)
	}

	return plotGraph
}

func Plot(data []*ruuvipb.RuuviStreamDataRequest) error {
	page := components.NewPage()
	page.AddCharts(
		plotTemperature(data),
		plotHumidity(data),
		plotPressure(data),
	)

	f, err := os.Create(outHTMLFilename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	if err = page.Render(io.MultiWriter(f)); err != nil {
		return fmt.Errorf("page render: %w", err)
	}

	return nil
}
