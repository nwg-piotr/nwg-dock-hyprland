package main

/*
#include <signal.h>
*/
import "C"

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/allan-simon/go-singleinstance"
	log "github.com/sirupsen/logrus"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtklayershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

const version = "0.4.6"

type WindowState int

const (
	WindowShow WindowState = iota
	WindowHide
)

var (
	activeClient                       *client
	appDirs                            []string
	clients                            []client
	configDirectory                    string
	dataHome                           string
	detectorEnteredAt                  int64
	his                                string // $HYPRLAND_INSTANCE_SIGNATURE
	hyprDir                            string // $XDG_RUNTIME_DIR/hypr since hyprland>0.39.1, earlier /tmp/hypr
	ignoredWorkspaces                  []string
	imgSizeScaled                      int
	lastWinAddr                        string
	mainBox                            *gtk.Box
	monitors                           []monitor
	oldClients                         []client
	outerOrientation, innerOrientation gtk.Orientation
	pinned                             []string
	pinnedFile                         string
	src                                glib.SourceHandle
	widgetAnchor, menuAnchor           gdk.Gravity
	win                                *gtk.Window
	windowStateChannel                 chan WindowState = make(chan WindowState, 1)
	classesToIgnore                    []string
)

// Flags
var alignment = flag.String("a", "center", "Alignment in full width/height: \"start\", \"center\" or \"end\"")
var autohide = flag.Bool("d", false, "auto-hiDe: show dock when hotspot hovered, close when left or a button clicked")
var cssFileName = flag.String("s", "style.css", "Styling: css file name")
var debug = flag.Bool("debug", false, "turn on debug messages")
var displayVersion = flag.Bool("v", false, "display Version information")
var exclusive = flag.Bool("x", false, "set eXclusive zone: move other windows aside; overrides the \"-l\" argument")
var full = flag.Bool("f", false, "take Full screen width/height")
var ignoreClasses = flag.String("g", "", "quote-delimited, space-separated class list to iGnore in the dock")
var hotspotDelay = flag.Int64("hd", 20, "Hotspot Delay [ms]; the smaller, the faster mouse pointer needs to enter hotspot for the dock to appear; set 0 to disable")
var hotspotLayer = flag.String("hl", "overlay", "Hotspot Layer \"overlay\" or \"top\"")
var ico = flag.String("ico", "", "alternative name or path for the launcher ICOn")
var ignoreWorkspaces = flag.String("iw", "", "Ignore the running applications on these Workspaces based on the workspace's name or id, e.g. \"special,10\"")
var imgSize = flag.Int("i", 48, "Icon size")
var launcherCmd = flag.String("c", "nwg-drawer", "Command assigned to the launcher button")
var launcherPos = flag.String("lp", "end", "Launcher button position, 'start' or 'end'")
var layer = flag.String("l", "overlay", "Layer \"overlay\", \"top\" or \"bottom\"")
var marginBottom = flag.Int("mb", 0, "Margin Bottom")
var marginLeft = flag.Int("ml", 0, "Margin Left")
var marginRight = flag.Int("mr", 0, "Margin Right")
var marginTop = flag.Int("mt", 0, "Margin Top")
var noLauncher = flag.Bool("nolauncher", false, "don't show the launcher button")
var numWS = flag.Int64("w", 10, "number of Workspaces you use")
var position = flag.String("p", "bottom", "Position: \"bottom\", \"top\" \"left\" or \"right\"")
var resident = flag.Bool("r", false, "Leave the program resident, but w/o hotspot")
var targetOutput = flag.String("o", "", "name of Output to display the dock on")
var allowMultipleInstances = flag.Bool("m", false, "allow Multiple instances of the dock (skip lock file check)")

var vertical bool
var alignmentBox *gtk.Box

func buildMainBox() {
	if mainBox != nil {
		mainBox.Destroy()
	}
	mainBox = gtk.NewBox(innerOrientation, 0)

	if *alignment == "start" {
		alignmentBox.PackStart(mainBox, false, true, 0)
	} else if *alignment == "end" {
		alignmentBox.PackEnd(mainBox, false, true, 0)
	} else {
		alignmentBox.PackStart(mainBox, true, false, 0)
	}

	var err error
	pinned, err = loadTextFile(pinnedFile)
	if err != nil {
		pinned = nil
	}

	var allItems []string
	for _, cntPin := range pinned {
		if !isIn(allItems, cntPin) {
			allItems = append(allItems, cntPin)
		}
	}

	// actually unnecessary in recent Hyprland versions, but just in case, see #44.
	sort.Slice(clients, func(i, j int) bool {
		if clients[i].Workspace.Id != clients[j].Workspace.Id {
			return clients[i].Workspace.Id < clients[j].Workspace.Id
		} else {
			return clients[i].Class < clients[j].Class
		}
	})

	// delete the clients that are on ignored workspaces
	clients = slices.DeleteFunc(clients, func(cl client) bool {
		// only use the part in front of ":" if something like "special:scratch_term" is being used
		clWorkspace, _, _ := strings.Cut(cl.Workspace.Name, ":")
		return isIn(ignoredWorkspaces, strconv.Itoa(cl.Workspace.Id)) || isIn(ignoredWorkspaces, clWorkspace)
	})

	for _, cntTask := range clients {
		if !isIn(allItems, cntTask.Class) && !strings.Contains(*launcherCmd, cntTask.Class) && cntTask.Class != "" {
			allItems = append(allItems, cntTask.Class)
		}
	}

	divider := 1
	if len(allItems) > 0 {
		divider = len(allItems)
	}

	// scale icons down when their number increases
	if *imgSize*6/(divider) < *imgSize {
		overflow := (len(allItems) - 6) / 3
		imgSizeScaled = *imgSize * 6 / (6 + overflow)
	} else {
		imgSizeScaled = *imgSize
	}

	if *launcherPos == "start" {
		button := launcherButton(position)
		if button != nil {
			mainBox.PackStart(button, false, false, 0)
		}
	}

	var alreadyAdded []string
	for _, pin := range pinned {
		if !inTasks(pin) {
			if !isIn(classesToIgnore, pin) {
				button := pinnedButton(pin, position)
				mainBox.PackStart(button, false, false, 0)
			} else {
				log.Debugf("Ignoring pin '%s'", pin)
			}
		} else {
			instances := taskInstances(pin)
			c := instances[0]
			if !isIn(classesToIgnore, c.Class) {
				if len(instances) == 1 {
					button := taskButton(c, instances, position)
					mainBox.PackStart(button, false, false, 0)
					if c.Class == activeClient.Class && !*autohide {
						button.SetObjectProperty("name", "active")
					} else {
						button.SetObjectProperty("name", "")
					}
				} else if !isIn(alreadyAdded, c.Class) {
					button := taskButton(c, instances, position)
					mainBox.PackStart(button, false, false, 0)
					if c.Class == activeClient.Class && !*autohide {
						button.SetObjectProperty("name", "active")
					} else {
						button.SetObjectProperty("name", "")
					}
					alreadyAdded = append(alreadyAdded, c.Class)
					clientMenu(c.Class, instances)
				} else {
					continue
				}
			} else {
				log.Debugf("Ignoring instance '%s'", c.Class)
			}
		}
	}

	alreadyAdded = nil
	for _, t := range clients {
		// For some time after killing a client, it's still being returned by 'j/clients', however w/o the Class value.
		// Let's filter the ghosts out.
		if !inPinned(t.Class) && t.Class != "" {
			instances := taskInstances(t.Class)
			if !isIn(classesToIgnore, t.Class) {
				if len(instances) == 1 {
					button := taskButton(t, instances, position)
					mainBox.PackStart(button, false, false, 0)
					if t.Class == activeClient.Class && !*autohide {
						button.SetObjectProperty("name", "active")
					} else {
						button.SetObjectProperty("name", "")
					}
				} else if !isIn(alreadyAdded, t.Class) {
					button := taskButton(t, instances, position)
					mainBox.PackStart(button, false, false, 0)
					if t.Class == activeClient.Class && !*autohide {
						button.SetObjectProperty("name", "active")
					} else {
						button.SetObjectProperty("name", "")
					}
					alreadyAdded = append(alreadyAdded, t.Class)
					clientMenu(t.Class, instances)
				} else {
					continue
				}
			} else {
				log.Debugf("Ignoring '%s'", t.Class)
			}
		}
	}

	if *launcherPos == "end" {
		button := launcherButton(position)
		if button != nil {
			mainBox.PackStart(button, false, false, 0)
		}
	}

	mainBox.ShowAll()
}

func setupHotSpot(monitor gdk.Monitor, dockWindow *gtk.Window) gtk.Window {
	w, h := dockWindow.Size()
	win := gtk.NewWindow(gtk.WindowToplevel)

	gtklayershell.InitForWindow(win)
	gtklayershell.SetMonitor(win, &monitor)
	gtklayershell.SetNamespace(win, "hotspot")

	var box *gtk.Box
	if *position == "bottom" || *position == "top" {
		box = gtk.NewBox(gtk.OrientationVertical, 0)
	} else {
		box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	}
	win.Add(box)

	detectorBox := gtk.NewEventBox()
	detectorBox.SetObjectProperty("name", "detector-box")

	if *position == "bottom" || *position == "right" {
		box.PackStart(detectorBox, false, false, 0)
	} else {
		box.PackEnd(detectorBox, false, false, 0)
	}

	detectorBox.Connect("enter-notify-event", func() {
		detectorEnteredAt = time.Now().UnixNano() / 1000000
	})

	hotspotBox := gtk.NewEventBox()
	hotspotBox.SetObjectProperty("name", "hotspot-box")

	if *position == "bottom" {
		box.PackStart(hotspotBox, false, false, 0)
	} else {
		box.PackEnd(hotspotBox, false, false, 0)
	}

	hotspotBox.Connect("enter-notify-event", func() {
		hotspotEnteredAt := time.Now().UnixNano() / 1000000
		delay := hotspotEnteredAt - detectorEnteredAt
		gtklayershell.SetMonitor(dockWindow, &monitor)
		if delay <= *hotspotDelay || *hotspotDelay == 0 {
			log.Debugf("Delay %v < %v ms, let's show the window!", delay, *hotspotDelay)
			dockWindow.Hide()
			dockWindow.Show()
		} else {
			log.Debugf("Delay %v > %v ms, don't show the window :/", delay, *hotspotDelay)
		}
	})

	if *position == "bottom" || *position == "top" {
		detectorBox.SetSizeRequest(w, h/3)
		hotspotBox.SetSizeRequest(w, 2)
		if *position == "bottom" {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeBottom, true)
		} else {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeTop, true)
		}

		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeLeft, *full)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeRight, *full)
	}

	if *position == "left" || *position == "right" {
		detectorBox.SetSizeRequest(w/3, h)
		hotspotBox.SetSizeRequest(2, h)
		if *position == "left" {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeLeft, true)
		} else {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeRight, true)
		}

		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeTop, *full)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeBottom, *full)
	}

	if *hotspotLayer == "top" {
		gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerTop)
	} else {
		gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerOverlay)
	}

	// resolve #65
	// gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeTop, *marginTop)
	// gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, *marginLeft)
	// gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, *marginRight)
	// gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeBottom, *marginBottom)

	gtklayershell.SetExclusiveZone(win, -1)

	return *win
}

func main() {
	sigRtmin := syscall.Signal(C.SIGRTMIN)
	sigToggle := sigRtmin + 1
	sigShow := sigRtmin + 2
	sigHide := sigRtmin + 3

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", flag.CommandLine.Name())
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nUsage of signals:\n")
		fmt.Fprintf(flag.CommandLine.Output(), " SIGRTMIN+1 (%s): toggle dock visibility (USR1 has been deprecated)\n", sigToggle)
		fmt.Fprintf(flag.CommandLine.Output(), " SIGRTMIN+2 (%s): show the dock\n", sigShow)
		fmt.Fprintf(flag.CommandLine.Output(), " SIGRTMIN+3 (%s): hide the dock\n", sigHide)
	}

	flag.Parse()
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	if *autohide && *resident {
		log.Warn("autohiDe and Resident arguments are mutually exclusive, ignoring -d!")
		*autohide = false
	}

	if *displayVersion {
		fmt.Printf("nwg-dock-hyprland version %s\n", version)
		os.Exit(0)
	}

	his = os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if his == "" {
		log.Fatal("HYPRLAND_INSTANCE_SIGNATURE not found, terminating.")
		os.Exit(1)
	}
	log.Debugf("HYPRLAND_INSTANCE_SIGNATURE: '%s'", his)

	if os.Getenv("XDG_RUNTIME_DIR") != "" && pathExists(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "hypr")) {
		hyprDir = filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "hypr")
	} else {
		hyprDir = "/tmp/hypr"
	}
	log.Debugf("hyprDir: '%s'", hyprDir)

	if *autohide {
		log.Info("Starting in autohiDe mode")
	}
	if *resident {
		log.Info("Starting in resident mode")
	}
	if *ignoreClasses != "" {
		log.Infof("Ignoring classes: '%s'", *ignoreClasses)
		classesToIgnore = strings.Split(*ignoreClasses, " ")
	}

	// Gentle SIGTERM handler thanks to reiki4040 https://gist.github.com/reiki4040/be3705f307d3cd136e85
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGUSR1, sigToggle, sigShow, sigHide)

	go func() {
		for {
			s := <-signalChan
			switch s {
			case syscall.SIGTERM:
				log.Info("SIGTERM received, bye bye!")
				gtk.MainQuit()
			case syscall.SIGUSR1:
				log.Warn("SIGUSR1 for toggling visibility is deprecated, use SIGRTMIN+1")
				if *resident || *autohide {
					if !win.IsVisible() {
						log.Debug("SIGUSR1 received, showing the window")
						windowStateChannel <- WindowShow
					} else {
						log.Debug("SIGUSR1 received, hiding the window")
						windowStateChannel <- WindowHide
					}
				} else {
					log.Debugf("SIGUSR1 received, but I'm not resident, ignoring")
				}
			case sigToggle:
				if *resident || *autohide {
					if !win.IsVisible() {
						log.Debug("sigToggle received, showing the window")
						windowStateChannel <- WindowShow
					} else {
						log.Debug("sigToggle received, hiding the window")
						windowStateChannel <- WindowHide
					}
				} else {
					log.Debug("sigToggle received, but I'm not resident, ignoring")
				}
			case sigShow:
				if *resident || *autohide {
					if !win.IsVisible() {
						log.Debug("sigShow received, showing the window")
						windowStateChannel <- WindowShow
					} else {
						log.Debug("sigShow received, but window already visible, ignoring")
					}
				} else {
					log.Debug("sigToggle received, but I'm not resident, ignoring")
				}
			case sigHide:
				if *resident || *autohide {
					if !win.IsVisible() {
						log.Debug("sigHide received, but window already hidden, ignoring")
					} else {
						log.Debug("sigHide received, hiding the window")
						windowStateChannel <- WindowHide
					}
				} else {
					log.Debug("sigHide received, but I'm not resident, ignoring")
				}
			default:
				log.Warn("Unknown signal")
			}
		}
	}()

	var err error
	if !*allowMultipleInstances {
		log.Debug("Allowing only one instance of nwg-dock-hyprland")
		// If running instance found, send sigToggle to it.
		// If it's running with `-r` or `-d` flag, it'll show/hide the window.
		// Otherwise, it'll ignore the signal.

		// Use md5-hashed $USER name to create unique lock files for multiple users
		lockFilePath := fmt.Sprintf("%s/nwg-dock-%s.lock", tempDir(), md5Hash(os.Getenv("USER")))
		lockFile, e := singleinstance.CreateLockFile(lockFilePath)
		if e != nil {
			pid, err := readTextFile(lockFilePath)
			if err == nil {
				i, err := strconv.Atoi(pid)
				if err == nil {
					if *autohide || *resident {
						log.Info("Running instance found, terminating...")
					} else {
						_ = syscall.Kill(i, sigToggle)
						log.Info("Sending sigToggle to running instance and bye, bye!")
					}
				}
			} else {
				log.Warnf("Error reading lock file: %s at %s", err, lockFilePath)
			}
			os.Exit(0)
		}
		defer lockFile.Close()
	}

	if !*noLauncher && *launcherCmd == "" {
		if isCommand("nwg-drawer") {
			*launcherCmd = "nwg-drawer"
		} else if isCommand("nwggrid") {
			*launcherCmd = "nwggrid -p"
		}

		if *launcherCmd != "" {
			log.Infof("Using auto-detected launcher command: '%s'", *launcherCmd)
		} else {
			log.Info("Neither 'nwg-drawer' nor 'nwggrid' command found, and no other launcher specified; hiding the launcher button.")
		}
	}

	dataHome, err = getDataHome()
	if err != nil {
		log.Fatal("Error getting data directory:", err)
	}
	configDirectory = configDir()
	// if it doesn't exist:
	createDir(configDirectory)

	if !pathExists(fmt.Sprintf("%s/style.css", configDirectory)) {
		err := copyFile(filepath.Join(dataHome, "nwg-dock-hyprland/style.css"), fmt.Sprintf("%s/style.css", configDirectory))
		if err != nil {
			log.Warnf("Error copying file: %s", err)
		}
	}

	cacheDirectory := cacheDir()
	if cacheDirectory == "" {
		log.Panic("Couldn't determine cache directory location")
	}
	pinnedFile = filepath.Join(cacheDirectory, "nwg-dock-pinned")
	cssFile := filepath.Join(configDirectory, *cssFileName)
	ignoredWorkspaces = strings.Split(*ignoreWorkspaces, ",")
	if *ignoreWorkspaces != "" {
		log.Infof("Ignored workspaces: %s\n", strings.Join(ignoredWorkspaces, ","))
	}

	appDirs = getAppDirs()

	gtk.Init()

	cssProvider := gtk.NewCSSProvider()

	err = cssProvider.LoadFromPath(cssFile)
	if err != nil {
		log.Warnf("%s file not found, using GTK styling\n", cssFile)
	} else {
		log.Printf("Using style: %s\n", cssFile)
		screen := gdk.ScreenGetDefault()
		gtk.StyleContextAddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	win = gtk.NewWindow(gtk.WindowToplevel)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	gtklayershell.InitForWindow(win)
	gtklayershell.SetNamespace(win, "nwg-dock")

	var output2mon map[string]*gdk.Monitor
	output2mon, err = mapOutputs()
	_, targetOutputExists := output2mon[*targetOutput]
	if !targetOutputExists {
		if *targetOutput != "" {
			log.Warnf("Target output '%s' not found, ignoring", *targetOutput)
		} else {
			log.Debug("No target output specified, using the focused one")
		}
	} else {
		log.Debugf("Creating widow on specified output: %s", *targetOutput)
	}

	if *targetOutput != "" {
		// We want to assign gtklayershell to a monitor, but we only know the output name!
		if err == nil {
			if targetOutputExists {
				gtklayershell.SetMonitor(win, output2mon[*targetOutput])
			}
		} else {
			log.Warn(fmt.Sprintf("Couldn't assign gtklayershell to monitor: %s", err))
		}
	}

	if *exclusive {
		gtklayershell.AutoExclusiveZoneEnable(win)
		*layer = "top"
	}

	if *position == "bottom" || *position == "top" {
		if *position == "bottom" {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeBottom, true)

			widgetAnchor = gdk.GravityNorth
			menuAnchor = gdk.GravitySouth
		} else {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeTop, true)

			widgetAnchor = gdk.GravitySouth
			menuAnchor = gdk.GravityNorth
		}

		outerOrientation = gtk.OrientationVertical
		innerOrientation = gtk.OrientationHorizontal

		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeLeft, *full)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeRight, *full)
	}

	if *position == "left" || *position == "right" {
		if *position == "left" {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeLeft, true)
		} else {
			gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeRight, true)
		}

		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeTop, *full)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeBottom, *full)

		outerOrientation = gtk.OrientationHorizontal
		innerOrientation = gtk.OrientationVertical

		widgetAnchor = gdk.GravityEast
		menuAnchor = gdk.GravityWest
	}

	if *layer == "top" {
		gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerTop)
	} else if *layer == "bottom" {
		gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerBottom)
	} else {
		gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerOverlay)
		gtklayershell.SetExclusiveZone(win, -1)
	}

	gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeTop, *marginTop)
	gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, *marginLeft)
	gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, *marginRight)
	gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeBottom, *marginBottom)

	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Close the window on leave, but not immediately, to avoid accidental closes
	win.Connect("leave-notify-event", func() {
		if *autohide {
			src = glib.TimeoutAdd(uint(1000), func() bool {
				win.Hide()
				src = 0
				return false
			})
		}
	})

	win.Connect("enter-notify-event", func() {
		cancelClose()
	})

	outerBox := gtk.NewBox(outerOrientation, 0)
	outerBox.SetObjectProperty("name", "box")
	win.Add(outerBox)

	alignmentBox = gtk.NewBox(innerOrientation, 0)
	outerBox.PackStart(alignmentBox, true, true, 0)

	mainBox = gtk.NewBox(innerOrientation, 0)
	// We'll pack mainBox later, in buildMainBox

	oldClients = clients
	refreshMainBox := func(forceRefresh bool) {
		if forceRefresh || (len(clients) != len(oldClients)) {
			glib.TimeoutAdd(0, func() bool {
				buildMainBox()
				oldClients = clients
				return false
			})
		}
	}

	err = listClients()
	if err != nil {
		log.Fatalf("Couldn't list clients: %s", err)
	}
	buildMainBox()

	win.ShowAll()

	if *autohide {
		glib.TimeoutAdd(uint(500), win.Hide)

		mRefProvider := gtk.NewCSSProvider()
		css := "window { background-color: rgba (0, 0, 0, 0); border: none}"
		hotspotCssFile := filepath.Join(configDirectory, "hotspot.css")
		if !pathExists(hotspotCssFile) {
			_ = mRefProvider.LoadFromData(css)
			log.Infof("Optional '%s' file not found, using internal definition", hotspotCssFile)
		} else {
			err := mRefProvider.LoadFromPath(hotspotCssFile)
			if err == nil {
				log.Infof("Hotspot css loaded from %s", hotspotCssFile)
			} else {
				log.Warnf("Error loading hotspot css from %s", hotspotCssFile)
			}
		}

		if err != nil {
			log.Warn(err)
		}

		if *targetOutput == "" || !targetOutputExists {
			// hot spots on all displays
			monitors, _ := listGdkMonitors()
			for _, monitor := range monitors {
				win := setupHotSpot(monitor, win)

				ctx := win.StyleContext()
				ctx.AddProvider(mRefProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

				win.ShowAll()
			}
		} else {
			// hot spot on the selected display only
			monitor := output2mon[*targetOutput]
			win := setupHotSpot(*monitor, win)
			ctx := win.StyleContext()
			ctx.AddProvider(mRefProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

			win.ShowAll()
		}
	}

	go func() {
		for {
			windowState := <-windowStateChannel

			glib.TimeoutAdd(0, func() bool {
				if windowState == WindowShow && win != nil && !win.IsVisible() {
					win.ShowAll()
				}
				if windowState == WindowHide && win != nil && win.IsVisible() {
					win.Hide()
				}

				return false
			})
		}
	}()

	addr := &net.UnixAddr{
		Name: fmt.Sprintf("%s/%s/.socket2.sock", hyprDir, his),
		Net:  "unix",
	}

	go func() {
		conn, err := net.DialUnix("unix", nil, addr)
		if err != nil {
			fmt.Println("Error connecting to the socket:", err)
			os.Exit(1)
		}
		defer conn.Close()

		for {
			buf := make([]byte, 10240)
			n, err := conn.Read(buf)
			if err != nil {
				fmt.Println("Error reading from socket2:", err)
			}

			s := string(buf[:n])
			if strings.Contains(s, "activewindowv2") {
				winAddr := strings.TrimSpace(strings.Split(s, "activewindowv2>>")[1])
				if winAddr != lastWinAddr && !strings.Contains(winAddr, ">>") {
					err = listClients()
					if err != nil {
						log.Fatalf("Couldn't list clients: %s", err)
					} else {
						refreshMainBox(true)
					}
					lastWinAddr = winAddr
				}
			}
		}
	}()

	gtk.Main()
}
