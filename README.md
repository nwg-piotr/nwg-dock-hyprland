# nwg-dock

This application is a part of the [nwg-shell](https://nwg-piotr.github.io/nwg-shell) project.

**Contributing:** please read the [general contributing rules for the nwg-shell project](https://nwg-piotr.github.io/nwg-shell/contribution).

Fully configurable (w/ command line arguments and css) dock, written in Go, aimed exclusively at [sway](https://github.com/swaywm/sway) Wayland compositor. It features pinned buttons, task buttons, the workspace switcher and the launcher button. The latter by default starts [nwg-drawer](https://github.com/nwg-piotr/nwg-drawer) or `nwggrid` (application grid) - if found. In the picture(s) below the dock has been shown together with [nwg-panel](https://github.com/nwg-piotr/nwg-panel).

![screenshot-1.png](https://raw.githubusercontent.com/nwg-piotr/nwg-shell-resources/master/images/nwg-dock/dock-1.png)

[![Packaging status](https://repology.org/badge/vertical-allrepos/nwg-dock.svg)](https://repology.org/project/nwg-dock/versions)

## Installation

### Requirements

- `go`>=1.16 (just to build)
- `gtk3`
- `gtk-layer-shell`
- [nwg-drawer](https://github.com/nwg-piotr/nwg-drawer) or
[nwg-launchers](https://github.com/nwg-piotr/nwg-launchers): optionally. You may use another launcher (see help),
or none at all. The launcher button won't show up, if so.

### Steps

1. Clone the repository, cd into it.
2. Install golang libraries with `make get`. First time it may take ages, be patient.
3. `make build`
4. `sudo make install`

## Running

Either start the dock permanently in the sway config file,

```text
exec nwg-dock [arguments]
```

or assign the command to some key binding. Running the command again kills the existing program instance, so that
you could use the same key to open and close the dock.

## Running the dock residently

If you run the program with the `-d` or `-r` argument (preferably in autostart), it will be running residently.

```text
exec_always nwg-dock -d
```

or

```text
exec_always nwg-dock -r
```

### `-d` for autohiDe

Move the mouse pointer to expected dock location for the dock to show up. It will be hidden a second after you leave the window. Invisible hot spots will be created on all your outputs, unless you specify one with the `-o` argument.

### `-r` for just Resident

No hotspot will be created. To show/hide the dock, bind the `exec nwg-dock` command to some key or button.
How about the `Menu` key, which is usually useless?

Re-execution of the same command hides the dock. If a resident instance found, the `nwg-dock` command w/o
arguments sends `SIGUSR1` to it. Actually `pkill -USR1 nwg-dock` could be used instead. This also works in autohiDe
mode.

Re-execution of the command with the `-d` or `-r` argument won't kill the running instance. If the dock is
running residently, another instance will just exit with 0 code. In case you'd like to terminate it anyway, you need to `pkill -f nwg-dock`.

*NOTE: you need to kill the running instance before reloading sway, if you've just changed the arguments you
auto-start the dock with.*

```txt
nwg-dock -h
Usage of nwg-dock:
  -a string
    	Alignment in full width/height: "start", "center" or "end" (default "center")
  -c string
    	Command assigned to the launcher button
  -d	auto-hiDe: show dock when hotspot hovered, close when left or a button clicked
  -debug
    	turn on debug messages
  -f	take Full screen width/height
  -i int
    	Icon size (default 48)
  -l string
    	Layer "overlay", "top" or "bottom" (default "overlay")
  -mb int
    	Margin Bottom
  -ml int
    	Margin Left
  -mr int
    	Margin Right
  -mt int
    	Margin Top
  -nolauncher
    	don't show the launcher button
  -nows
    	don't show the workspace switcher
  -o string
    	name of Output to display the dock on
  -p string
    	Position: "bottom", "top" or "left" (default "bottom")
  -r	Leave the program resident, but w/o hotspot
  -s string
    	Styling: css file name (default "style.css")
  -v	display Version information
  -w int
    	number of Workspaces you use (default 8)
  -x	set eXclusive zone: move other windows aside; overrides the "-l" argument
```

![screenshot-2.png](https://raw.githubusercontent.com/nwg-piotr/nwg-shell-resources/master/images/nwg-dock/dock-2.png)

## Styling

Edit `~/.config/nwg-dock/style.css` to your taste.

## Credits

This program uses some great libraries:

- [gotk3](https://github.com/gotk3/gotk3) Copyright (c) 2013-2014 Conformal Systems LLC,
Copyright (c) 2015-2018 gotk3 contributors
- [gotk3-layershell](https://github.com/dlasky/gotk3-layershell) by [@dlasky](https://github.com/dlasky/gotk3-layershell/commits?author=dlasky) - many thanks for writing this software, and for patience with my requests!
- [go-sway](https://github.com/joshuarubin/go-sway) Copyright (c) 2019 Joshua Rubin
- [go-singleinstance](github.com/allan-simon/go-singleinstance) Copyright (c) 2015 Allan Simon
