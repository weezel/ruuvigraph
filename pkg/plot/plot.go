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
			Value: []any{
				d.Timestamp.AsTime().Local().Format(time.RFC3339), // X axis
				d.GetTemperature(), // Y axis
			},
		})
	}
	return items
}

func getHumidity(data []*ruuvipb.RuuviStreamDataRequest) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range data {
		items = append(items, opts.LineData{
			Value: []any{
				d.Timestamp.AsTime().Local().Format(time.RFC3339), // X axis
				d.GetHumidity(), // Y axis
			},
		})
	}
	return items
}

func getPressure(data []*ruuvipb.RuuviStreamDataRequest) []opts.LineData {
	items := []opts.LineData{}
	for _, d := range data {
		items = append(items, opts.LineData{
			// Name: d.Device,
			Value: []any{
				d.Timestamp.AsTime().Local().Format(time.RFC3339), // X axis
				d.GetPressure(), // Y axis
			},
		})
	}
	return items
}

//nolint:dupl // Okay for now
func plotTemperature(data []*ruuvipb.RuuviStreamDataRequest) *charts.Line {
	plotGraph := charts.NewLine()
	plotGraph.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle: "Temperature",
			Width:     "100%",
			Height:    "500px",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Type: "time",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Min: 19.0,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Temperature",
			Subtitle: time.Now().Local().Format(time.DateTime),
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
		plotGraph.
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
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle: "Humidity",
			Width:     "100%",
			Height:    "500px",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Type: "time",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Humidity",
			Subtitle: time.Now().Local().Format(time.DateTime),
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
		plotGraph.
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
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle: "Pressure",
			Width:     "100%",
			Height:    "500px",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Type: "time",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Min: 900.0,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Air pressure",
			Subtitle: time.Now().Local().Format(time.DateTime),
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
		plotGraph.
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
