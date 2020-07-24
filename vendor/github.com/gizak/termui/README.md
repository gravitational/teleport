# termui

[<img src="./_assets/demo.gif" alt="demo cast under osx 10.10; Terminal.app; Menlo Regular 12pt.)" width="100%">](./_examples/demo.go)

termui is a cross-platform and fully-customizable terminal dashboard and widget library built on top of [termbox-go](https://github.com/nsf/termbox-go). It is inspired by [blessed-contrib](https://github.com/yaronn/blessed-contrib) and [tui-rs](https://github.com/fdehau/tui-rs) and written purely in Go.

## Features

- Built in widget implementations for common use cases
- Utilities to create custom widgets
- A grid layout for relative widget positioning
- Mouse support
- Event handling for keyboard, mouse and resizing events
- Colors and styling

## Installation

Installing from the master branch is recommended since termui has been completely reworked since the latest release and because the latest release doesn't support Go modules.

Do note that termui is still undergoing breaking changes so make sure to check the changelog when upgrading.

```bash
go get -u github.com/gizak/termui@master
```

## Hello World

```go
package main

import (
	"log"

	ui "github.com/gizak/termui"
	"github.com/gizak/termui/widgets"
)

func main() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	p := widgets.NewParagraph()
	p.Text = "Hello World!"
	p.SetRect(0, 0, 25, 5)

	ui.Render(p)

	for e := range ui.PollEvents() {
		if e.Type == ui.KeyboardEvent {
			break
		}
	}
}
```

## Widgets

- [BarChart](./_examples/barchart.go)
- [Canvas](./_examples/canvas.go) (for drawing braille dots)
- [Gauge](./_examples/gauge.go)
- [Image](./_examples/image.go)
- [List](./_examples/list.go)
- [Paragraph](./_examples/paragraph.go)
- [PieChart](./_examples/piechart.go)
- [Plot](./_examples/plot.go) (for scatterplots and linecharts)
- [Sparkline](./_examples/sparkline.go)
- [StackedBarChart](./_examples/stacked_barchart.go)
- [Table](./_examples/table.go)
- [Tabs](./_examples/tabs.go)

Run an example with `go run _examples/{example}.go` or run all of them consecutively with `make run-examples`.

## Documentation

- [wiki](https://github.com/gizak/termui/wiki)

## Uses

- [dockdash](https://github.com/byrnedo/dockdash)
- [expvarmon](https://github.com/divan/expvarmon)
- [go-ethereum/monitorcmd](https://github.com/ethereum/go-ethereum/blob/master/cmd/geth/monitorcmd.go)
- [go-jira-ui](https://github.com/mikepea/go-jira-ui)
- [gotop](https://github.com/cjbassi/gotop)
- [termeter](https://github.com/atsaki/termeter)

## Related Works

- [blessed-contrib](https://github.com/yaronn/blessed-contrib)
- [gocui](https://github.com/jroimartin/gocui)
- [termdash](https://github.com/mum4k/termdash)
- [tui-rs](https://github.com/fdehau/tui-rs)
- [tview](https://github.com/rivo/tview)

## License

[MIT](http://opensource.org/licenses/MIT)
