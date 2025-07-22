package plot

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"weezel/ruuvigraph/pkg/ruuvi"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

const outHTMLFilename = "sensor_data.html"

func getTemperatures(data *[]ruuvi.Data) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range *data {
		items = append(items, opts.LineData{
			Name:  d.Alias,
			Value: fmt.Sprintf("%.2f", d.Temperature),
		})
	}
	return items
}

func getHumidity(data *[]ruuvi.Data) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range *data {
		items = append(items, opts.LineData{
			Name:  d.Alias,
			Value: fmt.Sprintf("%.2f", d.Humidity),
		})
	}
	return items
}

func getAirPressure(data *[]ruuvi.Data) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range *data {
		var val float64
		if d.AirPressure > 1000 { // TODO Remove from the final version
			val = d.AirPressure / 10.0
		}
		items = append(items, opts.LineData{
			Name:  d.Alias,
			Value: fmt.Sprintf("%.2f", val),
		})
	}
	return items
}

func dateRange(data *[]ruuvi.Data) []string {
	items := []string{}
	for _, d := range *data {
		items = append(items, d.Datetime)
	}
	return items
}

//nolint:dupl // Okay for now
func plotTemperature(data *[]ruuvi.Data) *charts.Line {
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

	m := map[string]*[]ruuvi.Data{}
	for _, event := range *data {
		if _, found := m[event.Alias]; !found {
			m[event.Alias] = &[]ruuvi.Data{}
		}
		tmp := *m[event.Alias]
		tmp = append(tmp, event)
		m[event.Alias] = &tmp
	}

	for alias, values := range m {
		plotGraph.SetXAxis(dateRange(data)).
			AddSeries(alias, getTemperatures(values)).
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

func plotHumidity(data *[]ruuvi.Data) *charts.Line {
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

	m := map[string]*[]ruuvi.Data{}
	for _, event := range *data {
		if _, found := m[event.Alias]; !found {
			m[event.Alias] = &[]ruuvi.Data{}
		}
		tmp := *m[event.Alias]
		tmp = append(tmp, event)
		m[event.Alias] = &tmp
	}

	for alias, values := range m {
		plotGraph.SetXAxis(dateRange(data)).
			AddSeries(alias, getHumidity(values)).
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
func plotAirPressure(data *[]ruuvi.Data) *charts.Line {
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

	m := map[string]*[]ruuvi.Data{}
	for _, event := range *data {
		if _, found := m[event.Alias]; !found {
			m[event.Alias] = &[]ruuvi.Data{}
		}
		tmp := *m[event.Alias]
		tmp = append(tmp, event)
		m[event.Alias] = &tmp
	}

	for alias, values := range m {
		plotGraph.SetXAxis(dateRange(data)).
			AddSeries(alias, getAirPressure(values)).
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

func Plot(data *[]ruuvi.Data) error {
	page := components.NewPage()
	page.AddCharts(
		plotTemperature(data),
		plotHumidity(data),
		plotAirPressure(data),
	)

	f, err := os.Create(outHTMLFilename)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to create bar.html: %s", err))
		msg := fmt.Sprintf("create file %s", outHTMLFilename)
		return fmt.Errorf("%s: %w", msg, err)
	}

	return fmt.Errorf("page render: %w", page.Render(io.MultiWriter(f)))
}
