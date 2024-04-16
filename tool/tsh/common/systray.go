// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"github.com/getlantern/systray"
	"github.com/getlantern/systray/example/icon"
)

type systrayCommand struct {
	*kingpin.CmdClause
}

func newSystrayCommand(app *kingpin.Application) *systrayCommand {
	cmd := &systrayCommand{
		CmdClause: app.Command("systray", "Show systray icon for tsh."),
	}
	cmd.Hidden()
	return cmd
}

func (c systrayCommand) run(cf *CLIConf) error {
	signalCtx, stop := signal.NotifyContext(cf.Context, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-signalCtx.Done()
		fmt.Println("quitting")
		systray.Quit()
	}()

	fmt.Println("starting systray")

	systray.Run(systrayOnReady, func() {
		fmt.Println("systray on exit")
	})
	return nil
}

func systrayOnReady() {
	fmt.Println("systray on ready")
	systray.SetIcon(icon.Data)
	systray.SetTooltip("Pretty awesome超级棒")
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

	// Sets the icon of a menu item. Only available on Mac and Windows.
	mQuit.SetIcon(icon.Data)
}
