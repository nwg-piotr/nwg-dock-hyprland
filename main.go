package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/allan-simon/go-singleinstance"
	"github.com/dlasky/gotk3-layershell/layershell"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const version = "0.1.7"

type WindowState int

const (
	WindowShow WindowState = iota
	WindowHide
)

var (
	appDirs                            []string
	dataHome                           string
	configDirectory                    string
	pinnedFile                         string
	pinned                             []string
	oldClients                         []client
	mainBox                            *gtk.Box
	src                                glib.SourceHandle
	outerOrientation, innerOrientation gtk.Orientation
	widgetAnchor, menuAnchor           gdk.Gravity
	imgSizeScaled                      int
	win                                *gtk.Window
	windowStateChannel                 chan WindowState = make(chan WindowState, 1)
	detectorEnteredAt                  int64
	his                                string // $HYPRLAND_INSTANCE_SIGNATURE
	monitors                           []monitor
	clients                            []client
	activeClient                       *client
	lastWinAddr                        string
)

// Flags
var cssFileName = flag.String("s", "style.css", "Styling: css file name")
var targetOutput = flag.String("o", "", "name of Output to display the dock on")
var displayVersion = flag.Bool("v", false, "display Version information")
var autohide = flag.Bool("d", false, "auto-hiDe: show dock when hotspot hovered, close when left or a button clicked")
var full = flag.Bool("f", false, "take Full screen width/height")
var numWS = flag.Int64("w", 10, "number of Workspaces you use")
var position = flag.String("p", "bottom", "Position: \"bottom\", \"top\" or \"left\"")
var exclusive = flag.Bool("x", false, "set eXclusive zone: move other windows aside; overrides the \"-l\" argument")
var imgSize = flag.Int("i", 48, "Icon size")
var ico = flag.String("ico", "", "alternative name or path for the launcher ICOn")
var layer = flag.String("l", "overlay", "Layer \"overlay\", \"top\" or \"bottom\"")
var launcherCmd = flag.String("c", "", "Command assigned to the launcher button")
var alignment = flag.String("a", "center", "Alignment in full width/height: \"start\", \"center\" or \"end\"")
var marginTop = flag.Int("mt", 0, "Margin Top")
var marginLeft = flag.Int("ml", 0, "Margin Left")
var marginRight = flag.Int("mr", 0, "Margin Right")
var marginBottom = flag.Int("mb", 0, "Margin Bottom")
var hotspotDelay = flag.Int64("hd", 20, "Hotspot Delay [ms]; the smaller, the faster mouse pointer needs to enter hotspot for the dock to appear; set 0 to disable")
var noLauncher = flag.Bool("nolauncher", false, "don't show the launcher button")
var resident = flag.Bool("r", false, "Leave the program resident, but w/o hotspot")
var debug = flag.Bool("debug", false, "turn on debug messages")

func buildMainBox(vbox *gtk.Box) {
	if mainBox != nil {
		mainBox.Destroy()
	}
	mainBox, _ = gtk.BoxNew(innerOrientation, 0)

	if *alignment == "start" {
		vbox.PackStart(mainBox, false, true, 0)
	} else if *alignment == "end" {
		vbox.PackEnd(mainBox, false, true, 0)
	} else {
		vbox.PackStart(mainBox, true, false, 0)
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

	var alreadyAdded []string
	for _, pin := range pinned {
		if !inTasks(pin) {
			button := pinnedButton(pin)
			mainBox.PackStart(button, false, false, 0)
		} else {
			instances := taskInstances(pin)
			c := instances[0]
			if len(instances) == 1 {
				button := taskButton(c, instances)
				mainBox.PackStart(button, false, false, 0)
				if c.Class == activeClient.Class && !*autohide {
					button.SetProperty("name", "active")
				} else {
					button.SetProperty("name", "")
				}
			} else if !isIn(alreadyAdded, c.Class) {
				button := taskButton(c, instances)
				mainBox.PackStart(button, false, false, 0)
				if c.Class == activeClient.Class && !*autohide {
					button.SetProperty("name", "active")
				} else {
					button.SetProperty("name", "")
				}
				alreadyAdded = append(alreadyAdded, c.Class)
				clientMenu(c.Class, instances)
			} else {
				continue
			}
		}
	}

	alreadyAdded = nil
	for _, t := range clients {
		// For some time after killing a client, it's still being returned by 'j/clients', however w/o the Class value.
		// Let's filter the ghosts out.
		if !inPinned(t.Class) && t.Class != "" {
			instances := taskInstances(t.Class)
			if len(instances) == 1 {
				button := taskButton(t, instances)
				mainBox.PackStart(button, false, false, 0)
				if t.Class == activeClient.Class && !*autohide {
					button.SetProperty("name", "active")
				} else {
					button.SetProperty("name", "")
				}
			} else if !isIn(alreadyAdded, t.Class) {
				button := taskButton(t, instances)
				mainBox.PackStart(button, false, false, 0)
				if t.Class == activeClient.Class && !*autohide {
					button.SetProperty("name", "active")
				} else {
					button.SetProperty("name", "")
				}
				alreadyAdded = append(alreadyAdded, t.Class)
				clientMenu(t.Class, instances)
			} else {
				continue
			}
		}
	}

	if !*noLauncher && *launcherCmd != "" {
		button, _ := gtk.ButtonNew()
		var pixbuf *gdk.Pixbuf
		var e error
		if *ico == "" {
			pixbuf, e = gdk.PixbufNewFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/grid.svg"), imgSizeScaled, imgSizeScaled)
		} else {
			pixbuf, e = createPixbuf(*ico, imgSizeScaled)
		}
		if e == nil {
			image, _ := gtk.ImageNewFromPixbuf(pixbuf)
			button.SetImage(image)
			button.SetAlwaysShowImage(true)

			button.Connect("clicked", func() {
				elements := strings.Split(*launcherCmd, " ")
				cmd := exec.Command(elements[0], elements[1:]...)

				go func() {
					err := cmd.Run()
					if err != nil {
						log.Warnf("Unable to start program: %s", err.Error())
					}
				}()

				if *autohide {
					win.Hide()
				}
			})
			button.Connect("enter-notify-event", cancelClose)
		} else {
			log.Errorf("Unable to show grid button: %s", err.Error())
		}

		mainBox.PackStart(button, false, false, 0)
	}

	mainBox.ShowAll()
}

func setupHotSpot(monitor gdk.Monitor, dockWindow *gtk.Window) gtk.Window {
	w, h := dockWindow.GetSize()
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)

	layershell.InitForWindow(win)
	layershell.SetMonitor(win, &monitor)

	var box *gtk.Box
	if *position == "bottom" || *position == "top" {
		box, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	} else {
		box, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	}
	win.Add(box)

	detectorBox, _ := gtk.EventBoxNew()
	_ = detectorBox.SetProperty("name", "detector-box")

	if *position == "bottom" {
		box.PackStart(detectorBox, false, false, 0)
	} else {
		box.PackEnd(detectorBox, false, false, 0)
	}

	detectorBox.Connect("enter-notify-event", func() {
		detectorEnteredAt = time.Now().UnixNano() / 1000000
	})

	hotspotBox, _ := gtk.EventBoxNew()
	_ = hotspotBox.SetProperty("name", "hotspot-box")

	if *position == "bottom" {
		box.PackStart(hotspotBox, false, false, 0)
	} else {
		box.PackEnd(hotspotBox, false, false, 0)
	}

	hotspotBox.Connect("enter-notify-event", func() {
		hotspotEnteredAt := time.Now().UnixNano() / 1000000
		delay := hotspotEnteredAt - detectorEnteredAt
		layershell.SetMonitor(dockWindow, &monitor)
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
			layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_BOTTOM, true)
		} else {
			layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_TOP, true)
		}

		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_LEFT, *full)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_RIGHT, *full)
	}

	if *position == "left" {
		detectorBox.SetSizeRequest(w/3, h)
		hotspotBox.SetSizeRequest(2, h)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_LEFT, true)

		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_TOP, *full)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_BOTTOM, *full)
	}

	layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_OVERLAY)

	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_TOP, *marginTop)
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_LEFT, *marginLeft)
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_RIGHT, *marginRight)
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_BOTTOM, *marginBottom)

	layershell.SetExclusiveZone(win, -1)

	return *win
}

func main() {
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

	if *autohide {
		log.Info("Starting in autohiDe mode")
	}
	if *resident {
		log.Info("Starting in resident mode")
	}

	// Gentle SIGTERM handler thanks to reiki4040 https://gist.github.com/reiki4040/be3705f307d3cd136e85
	// v0.2: we also need to support SIGUSR from now on.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		for {
			s := <-signalChan
			switch s {
			case syscall.SIGTERM:
				log.Info("SIGTERM received, bye bye!")
				gtk.MainQuit()
			case syscall.SIGUSR1:
				if *resident || *autohide {
					// As win.Show() called from inside a goroutine randomly crashes GTK,
					// let's just set e helper variable here. We'll be checking it with glib.TimeoutAdd.
					if !win.IsVisible() {
						log.Debug("SIGUSR1 received, showing the window")
						windowStateChannel <- WindowShow
					} else {
						log.Debug("SIGUSR1 received, hiding the window")
						windowStateChannel <- WindowHide
					}
				} else {
					log.Info("SIGUSR1 received, and I'm not resident, bye bye!")
					gtk.MainQuit()
				}
			default:
				log.Warn("Unknown signal")
			}
		}
	}()

	// Unless we are in autohide/resident mode, we probably want the same key/mouse binding to turn the dock off.
	// Since v0.2 we can't just send SIGKILL if running instance found. We'll send SIGUSR1 instead.
	// If it's running with `-r` or `-d` flag, it'll show the window. If not - it will die.
	lockFilePath := fmt.Sprintf("%s/nwg-dock.lock", tempDir())
	lockFile, err := singleinstance.CreateLockFile(lockFilePath)
	if err != nil {
		pid, err := readTextFile(lockFilePath)
		if err == nil {
			i, err := strconv.Atoi(pid)
			if err == nil {
				if *autohide || *resident {
					log.Info("Running instance found, terminating...")
				} else {
					_ = syscall.Kill(i, syscall.SIGUSR1)
					log.Info("Sending SIGUSR1 to running instance and bye, bye!")
				}
			}
		}
		os.Exit(0)
	}
	defer lockFile.Close()

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

	dataHome = getDataHome()
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

	appDirs = getAppDirs()

	gtk.Init(nil)

	cssProvider, _ := gtk.CssProviderNew()

	err = cssProvider.LoadFromPath(cssFile)
	if err != nil {
		log.Warnf("%s file not found, using GTK styling\n", cssFile)
	} else {
		log.Printf("Using style: %s\n", cssFile)
		screen, _ := gdk.ScreenGetDefault()
		gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	win, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	layershell.InitForWindow(win)

	var output2mon map[string]*gdk.Monitor
	if *targetOutput != "" {
		// We want to assign layershell to a monitor, but we only know the output name!
		output2mon, err = mapOutputs()
		if err == nil {
			layershell.SetMonitor(win, output2mon[*targetOutput])
		} else {
			log.Warn(fmt.Sprintf("Couldn't assign layershell to monitor: %s", err))
		}
	}

	if *exclusive {
		layershell.AutoExclusiveZoneEnable(win)
		*layer = "top"
	}

	if *position == "bottom" || *position == "top" {
		if *position == "bottom" {
			layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_BOTTOM, true)

			widgetAnchor = gdk.GDK_GRAVITY_NORTH
			menuAnchor = gdk.GDK_GRAVITY_SOUTH
		} else {
			layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_TOP, true)

			widgetAnchor = gdk.GDK_GRAVITY_SOUTH
			menuAnchor = gdk.GDK_GRAVITY_NORTH
		}

		outerOrientation = gtk.ORIENTATION_VERTICAL
		innerOrientation = gtk.ORIENTATION_HORIZONTAL

		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_LEFT, *full)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_RIGHT, *full)
	}

	if *position == "left" {
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_LEFT, true)

		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_TOP, *full)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_BOTTOM, *full)

		outerOrientation = gtk.ORIENTATION_HORIZONTAL
		innerOrientation = gtk.ORIENTATION_VERTICAL

		widgetAnchor = gdk.GDK_GRAVITY_EAST
		menuAnchor = gdk.GDK_GRAVITY_WEST
	}

	if *layer == "top" {
		layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_TOP)
	} else if *layer == "bottom" {
		layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_BOTTOM)
	} else {
		layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_OVERLAY)
		layershell.SetExclusiveZone(win, -1)
	}

	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_TOP, *marginTop)
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_LEFT, *marginLeft)
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_RIGHT, *marginRight)
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_BOTTOM, *marginBottom)

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

	outerBox, _ := gtk.BoxNew(outerOrientation, 0)
	_ = outerBox.SetProperty("name", "box")
	win.Add(outerBox)

	alignmentBox, _ := gtk.BoxNew(innerOrientation, 0)
	outerBox.PackStart(alignmentBox, true, true, 0)

	mainBox, _ = gtk.BoxNew(innerOrientation, 0)
	// We'll pack mainBox later, in buildMainBox

	oldClients = clients
	refreshMainBox := func(forceRefresh bool) {
		if forceRefresh || (len(clients) != len(oldClients)) {
			glib.TimeoutAdd(0, func() bool {
				buildMainBox(alignmentBox)
				oldClients = clients
				return false
			})
		}
	}

	err = listClients()
	if err != nil {
		log.Fatalf("Couldn't list clients: %s", err)
	}
	buildMainBox(alignmentBox)

	win.ShowAll()

	if *autohide {
		glib.TimeoutAdd(uint(500), win.Hide)

		mRefProvider, _ := gtk.CssProviderNew()
		css := "window { background-color: rgba (0, 0, 0, 0); border: none}"
		err := mRefProvider.LoadFromData(css)
		if err != nil {
			log.Warn(err)
		}

		if *targetOutput == "" {
			// hot spots on all displays
			monitors, _ := listGdkMonitors()
			for _, monitor := range monitors {
				win := setupHotSpot(monitor, win)

				ctx, _ := win.GetStyleContext()
				ctx.AddProvider(mRefProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

				win.ShowAll()
			}
		} else {
			// hot spot on the selected display only
			monitor := output2mon[*targetOutput]
			win := setupHotSpot(*monitor, win)

			ctx, _ := win.GetStyleContext()
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
		Name: fmt.Sprintf("/tmp/hypr/%s/.socket2.sock", his),
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
