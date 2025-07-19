package table

import (
	"bytes"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

func RenderTable(data [][]string) string {
	colorCfg := renderer.ColorizedConfig{
		Header: renderer.Tint{
			FG: renderer.Colors{color.FgGreen, color.Bold},
			BG: renderer.Colors{color.BgHiWhite},
		},
		Column: renderer.Tint{
			FG: renderer.Colors{color.FgCyan},
			Columns: []renderer.Tint{
				{FG: renderer.Colors{color.FgMagenta}},
				{},
				{FG: renderer.Colors{color.FgHiRed}},
			},
		},
		Footer: renderer.Tint{
			FG: renderer.Colors{color.FgYellow, color.Bold},
			Columns: []renderer.Tint{
				{},
				{FG: renderer.Colors{color.FgHiYellow}},
				{},
			},
		},
		Border:    renderer.Tint{FG: renderer.Colors{color.FgWhite}},
		Separator: renderer.Tint{FG: renderer.Colors{color.FgWhite}},
	}

	var buf bytes.Buffer

	tbl := tablewriter.NewTable(&buf,
		tablewriter.WithRenderer(renderer.NewColorized(colorCfg)),
		tablewriter.WithConfig(tablewriter.Config{
			Row: tw.CellConfig{
				Formatting:   tw.CellFormatting{AutoWrap: tw.WrapNormal},
				Alignment:    tw.CellAlignment{Global: tw.AlignLeft},
				ColMaxWidths: tw.CellWidth{Global: 25},
			},
			Footer: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignRight},
			},
		}),
	)

	tbl.Header([]string{"metrics", "value"})
	tbl.Bulk(data)
	tbl.Render()

	return buf.String()
}
