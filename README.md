# nwg-dock-hyprland

This application is a part of the [nwg-shell](https://nwg-piotr.github.io/nwg-shell) project.

**Contributing:** please read the [general contributing rules for the nwg-shell project](https://nwg-piotr.github.io/nwg-shell/contribution).

Configurable (w/ command line arguments and css) dock, written in Go, aimed exclusively at the [Hyprland](https://github.com/hyprwm/Hyprland) 
Wayland compositor. It features pinned buttons, client buttons and the launcher button. The latter by default starts 
[nwg-drawer](https://github.com/nwg-piotr/nwg-drawer).

![2023-04-22-021230_hypr_screenshot](https://user-images.githubusercontent.com/20579136/233751336-b5c6abdd-72f7-43c7-b34d-e2f64248eb86.png)

![image](https://user-images.githubusercontent.com/20579136/233751391-97f8f685-55ae-4078-badf-b8c3d7c41ab4.png)

## Differences from nwg-dock for sway:

- instead of swayipc, we use Hyprland IP C, via socket & socket2, to execute hyprctl commands and listen to events;
- removed the workspace switcher button; AFAIK it's not widely used even on sway. On Hyprland I don't know of a way to check the currently focused workspace, and it would limit the functionality of the button;
- added highlighting of the button that represents the focused client (permanent docks only);
- added 2 entries to the context (right click) menu: `togglefloating` and `fullscreen`;
- fixed searching .desktop files of the names starting from `org.` and the like.

[![Packaging status](https://repology.org/badge/vertical-allrepos/nwg-dock-hyprland.svg)](https://repology.org/project/nwg-dock-hyprland/versions)

## Installation

### Requirements

- `go`>=1.20 (just to build)
- `gtk3`
- `gtk-layer-shell`
- [nwg-drawer](https://github.com/nwg-piotr/nwg-drawer): optionally. You may use another launcher (see help),
or none at all. The launcher button won't show up, if so.

### Steps

1. Clone the repository, cd into it.
2. Install golang libraries with `make get`. First time it may take ages, be patient.
3. `make build`
4. `sudo make install`

## Running

Either start the dock permanently in `hyprland.conf`:

```text
exec_once = nwg-dock [arguments]
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

Move the mouse pointer to expected dock location for the dock to show up. It will be hidden a second after you leave the
window. Invisible hot spots will be created on all your outputs, unless you specify one with the `-o` argument.

### `-r` for just Resident

No hotspot will be created. To show/hide the dock, bind the `exec nwg-dock` command to some key or button.
How about the `Menu` key, which is usually useless?

Re-execution of the same command hides the dock. If a resident instance found, the `nwg-dock` command w/o
arguments sends `SIGUSR1` to it. Actually `pkill -USR1 nwg-dock` could be used instead. This also works in autohiDe
mode.

Re-execution of the command with the `-d` or `-r` argument won't kill the running instance. If the dock is
running residently, another instance will just exit with 0 code. In case you'd like to terminate it anyway, you need 
to `pkill -f nwg-dock`.

*NOTE: you need to kill the running instance before reloading Hyprland, if you've just changed the arguments you
auto-start the dock with.*

```txt
$ nwg-dock-hyprland -h
Usage of nwg-dock-hyprland:
  -a string
    	Alignment in full width/height: "start", "center" or "end" (default "center")
  -c string
    	Command assigned to the launcher button
  -d	auto-hiDe: show dock when hotspot hovered, close when left or a button clicked
  -debug
    	turn on debug messages
  -f	take Full screen width/height
  -hd int
    	Hotspot Delay [ms]; the smaller, the faster mouse pointer needs to enter hotspot for the dock to appear; set 0 to disable (default 20)
  -i int
    	Icon size (default 48)
  -ico string
    	alternative name or path for the launcher ICOn
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
  -o string
    	name of Output to display the dock on
  -p string
    	Position: "bottom", "top" or "left" (default "bottom")
  -r	Leave the program resident, but w/o hotspot
  -s string
    	Styling: css file name (default "style.css")
  -v	display Version information
  -w int
    	number of Workspaces you use (default 10)
  -x	set eXclusive zone: move other windows aside; overrides the "-l" argument
```

![screenshot-2.png](https://raw.githubusercontent.com/nwg-piotr/nwg-shell-resources/master/images/nwg-dock/dock-2.png)

## Styling

Edit `~/.config/nwg-dock-hyprland/style.css` to your taste.

## Troubleshooting

### An application icon is not displayed

The only thing the dock knows about the app is it's class name.

```text
$ hyprctl clients
(...)
Window 55a62254b8c0 -> piotr@msi:~:
	mapped: 1
	hidden: 0
	at: 1204,270
	size: 2552,1402
	workspace: 6 (6)
	floating: 0
	monitor: 2
	class: foot
	title: piotr@msi:~
	initialClass: foot
	initialTitle: foot
	pid: 58348
	xwayland: 0
	pinned: 0
	fullscreen: 0
	fullscreenmode: 0
	fakefullscreen: 0
	grouped: 0
	swallowing: 0
```

Now it'll look for an icon named 'foot'. If that fails, it'll look for a .desktop file named foot.desktop, which should contain the icon name or path. If this fails as well, no icon will be displayed. I've added workarounds for some most common exceptions, but it's impossible to predict every single application misbehaviour. This is either programmers fault (improper class name), or bad packaging (.desktop file name different from the application class name).

If some app has no icon in the dock:

1. check the app class name (`hyprctl clients`);
2. find the app's .desktop file;
3. copy it to ~/.local/share/applications/` and rename to <class_name>.desktop.

If the .desktop file contains proper icon definition (`Icon=`), it should work now.

## Credits

This program uses some great libraries:

- [gotk3](https://github.com/gotk3/gotk3) Copyright (c) 2013-2014 Conformal Systems LLC,
Copyright (c) 2015-2018 gotk3 contributors
- [gotk3-layershell](https://github.com/dlasky/gotk3-layershell) by [@dlasky](https://github.com/dlasky/gotk3-layershell/commits?author=dlasky) - many thanks for writing this software, and for patience with my requests!
- [go-singleinstance](github.com/allan-simon/go-singleinstance) Copyright (c) 2015 Allan Simon
