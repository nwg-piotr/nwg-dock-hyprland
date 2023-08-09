package main

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

func taskInstances(ID string) []client {
	var found []client
	for _, c := range clients {
		if strings.Contains(strings.ToUpper(c.Class), strings.ToUpper(ID)) {
			found = append(found, c)
		}
	}
	return found
}

func pinnedButton(ID string) *gtk.Box {
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	button, _ := gtk.ButtonNew()
	box.PackStart(button, false, false, 0)

	image, err := createImage(ID, imgSizeScaled)
	if err != nil {
		pixbuf, err := gdk.PixbufNewFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/icon-missing.svg"),
			imgSizeScaled, imgSizeScaled)
		if err == nil {
			image, _ = gtk.ImageNewFromPixbuf(pixbuf)
		} else {
			image, _ = gtk.ImageNew()
		}
	}

	button.SetImage(image)
	button.SetImagePosition(gtk.POS_TOP)
	button.SetAlwaysShowImage(true)
	button.SetTooltipText(getName(ID))
	pixbuf, err := gdk.PixbufNewFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/task-empty.svg"),
		imgSizeScaled, imgSizeScaled/8)
	var img *gtk.Image
	if err == nil {
		img, err = gtk.ImageNewFromPixbuf(pixbuf)
		if err == nil {
			box.PackStart(img, false, false, 0)
		}
	}

	button.Connect("clicked", func() {
		launch(ID)
	})

	button.Connect("button-release-event", func(btn *gtk.Button, e *gdk.Event) bool {
		btnEvent := gdk.EventButtonNewFromEvent(e)
		if btnEvent.Button() == 1 {
			launch(ID)
			return true
		} else if btnEvent.Button() == 3 {
			contextMenu := pinnedMenuContext(ID)
			contextMenu.PopupAtWidget(button, widgetAnchor, menuAnchor, nil)
			return true
		}
		return false
	})

	button.Connect("enter-notify-event", cancelClose)
	return box
}

func pinnedMenuContext(taskID string) gtk.Menu {
	menu, _ := gtk.MenuNew()
	menuItem, _ := gtk.MenuItemNewWithLabel("Unpin")
	menuItem.Connect("activate", func() {
		unpinTask(taskID)
	})
	menu.Append(menuItem)

	menu.ShowAll()
	return *menu
}

/*
Window on-leave-notify event hides the dock with glib Timeout 1000 ms.
We might have left the window by accident, so let's clear the timeout if window re-entered.
Furthermore - hovering a button triggers window on-leave-notify event, and the timeout
needs to be cleared as well.
*/
func cancelClose() {
	if src > 0 {
		glib.SourceRemove(src)
		src = 0
	}
}

func taskButton(t client, instances []client) *gtk.Box {
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	button, _ := gtk.ButtonNew()
	box.PackStart(button, false, false, 0)

	image, err := createImage(t.Class, imgSizeScaled)
	if err != nil {
		pixbuf, err := gdk.PixbufNewFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/icon-missing.svg"),
			imgSizeScaled, imgSizeScaled)
		if err == nil {
			image, _ = gtk.ImageNewFromPixbuf(pixbuf)
		} else {
			image, _ = gtk.ImageNew()
		}
	}

	button.SetImage(image)
	button.SetImagePosition(gtk.POS_TOP)
	button.SetAlwaysShowImage(true)
	button.SetTooltipText(getName(t.Class))
	var img *gtk.Image
	if len(instances) < 2 {
		pixbuf, err := gdk.PixbufNewFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/task-single.svg"),
			imgSizeScaled, imgSizeScaled/8)
		if err == nil {
			img, _ = gtk.ImageNewFromPixbuf(pixbuf)
		}
	} else {
		pixbuf, err := gdk.PixbufNewFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/task-multiple.svg"),
			imgSizeScaled, imgSizeScaled/8)
		if err == nil {
			img, _ = gtk.ImageNewFromPixbuf(pixbuf)
		}
	}
	if img != nil {
		box.PackStart(img, false, false, 0)
	}
	button.Connect("enter-notify-event", cancelClose)

	if len(instances) == 1 {
		button.Connect("event", func(btn *gtk.Button, e *gdk.Event) bool {
			btnEvent := gdk.EventButtonNewFromEvent(e)
			/* EVENT_BUTTON_PRESS would be more obvious, but it causes the misbehaviour:
			   if con is located on an external display, after pressing the button, the conID value
			   "freezes", and stays the same for all taskButtons, until the right mouse click.
			   A gotk3 bug or WTF? */
			if btnEvent.Type() == gdk.EVENT_BUTTON_RELEASE || btnEvent.Type() == gdk.EVENT_TOUCH_END {
				if btnEvent.Button() == 1 || btnEvent.Type() == gdk.EVENT_TOUCH_END {
					cmd := fmt.Sprintf("dispatch focuswindow address:%s", t.Address)
					reply, _ := hyprctl(cmd)
					log.Debugf("%s -> %s", cmd, reply)
					return true
				} else if btnEvent.Button() == 3 {
					contextMenu := clientMenuContext(t.Class, instances)
					contextMenu.PopupAtWidget(button, widgetAnchor, menuAnchor, nil)
					return true
				}
			}
			return false
		})
	} else {
		button.Connect("button-release-event", func(btn *gtk.Button, e *gdk.Event) bool {
			btnEvent := gdk.EventButtonNewFromEvent(e)
			if btnEvent.Button() == 1 {
				menu := clientMenu(t.Class, instances)
				menu.PopupAtWidget(button, widgetAnchor, menuAnchor, nil)
				return true
			} else if btnEvent.Button() == 3 {
				contextMenu := clientMenuContext(t.Class, instances)
				contextMenu.PopupAtWidget(button, widgetAnchor, menuAnchor, nil)
				return true
			}
			return false
		})
	}

	return box
}

func clientMenu(class string, instances []client) gtk.Menu {
	menu, _ := gtk.MenuNew()

	iconName, err := getIcon(class)
	if err != nil {
		log.Warn(err)
	}
	for _, instance := range instances {
		menuItem, _ := gtk.MenuItemNew()
		hbox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
		image, _ := gtk.ImageNewFromIconName(iconName, gtk.ICON_SIZE_MENU)
		hbox.PackStart(image, false, false, 0)
		title := instance.Title
		if len(title) > 20 {
			title = title[:20]
		}
		label, _ := gtk.LabelNew(fmt.Sprintf("%s (%v)", title, instance.Workspace.Id))
		hbox.PackStart(label, false, false, 0)
		menuItem.Add(hbox)
		menu.Append(menuItem)
		a := instance.Address
		menuItem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch focuswindow address:%s", a)
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

	}
	menu.ShowAll()
	return *menu
}

func clientMenuContext(class string, instances []client) gtk.Menu {
	menu, _ := gtk.MenuNew()

	iconName, err := getIcon(class)
	if err != nil {
		log.Warnf("%s %s", err, class)
	}
	for _, instance := range instances {
		menuItem, _ := gtk.MenuItemNew()
		hbox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
		image, _ := gtk.ImageNewFromIconName(iconName, gtk.ICON_SIZE_MENU)
		hbox.PackStart(image, false, false, 0)
		title := instance.Title
		if len(title) > 20 {
			title = title[:20]
		}
		// Clean non-ASCII chars
		title = strings.Map(func(r rune) rune {
			if r > unicode.MaxASCII {
				return -1
			}
			return r
		}, title)
		label, _ := gtk.LabelNew(fmt.Sprintf("%s (%v)", title, instance.Workspace.Id))
		hbox.PackStart(label, false, false, 0)
		menuItem.Add(hbox)
		menu.Append(menuItem)
		submenu, _ := gtk.MenuNew()

		a := instance.Address

		subitem, _ := gtk.MenuItemNewWithLabel("closewindow")
		submenu.Append(subitem)
		subitem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch closewindow address:%s", a)
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

		subitem, _ = gtk.MenuItemNewWithLabel("togglefloating")
		submenu.Append(subitem)
		subitem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch togglefloating address:%s", a)
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

		subitem, _ = gtk.MenuItemNewWithLabel("fullscreen")
		submenu.Append(subitem)
		subitem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch fullscreen address:%s", a)
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

		s, _ := gtk.SeparatorMenuItemNew()
		submenu.Append(s)

		for i := 1; i < int(*numWS)+1; i++ {
			subItem, _ := gtk.MenuItemNewWithLabel(fmt.Sprintf("-> WS %v", i))
			target := i
			subItem.Connect("activate", func() {
				cmd := fmt.Sprintf("dispatch movetoworkspace %v,address:%v", target, a)
				reply, _ := hyprctl(cmd)
				log.Debugf("%s -> %s", cmd, reply)
			})
			submenu.Append(subItem)
		}

		menuItem.SetSubmenu(submenu)
	}
	separator, _ := gtk.SeparatorMenuItemNew()
	menu.Append(separator)

	item, _ := gtk.MenuItemNewWithLabel("New window")
	item.Connect("activate", func() {
		launch(class)
	})
	menu.Append(item)

	pinItem, _ := gtk.MenuItemNew()
	if !inPinned(class) {
		pinItem.SetLabel("Pin")
		pinItem.Connect("activate", func() {
			log.Infof("pin %s", class)
			pinTask(class)
		})
	} else {
		pinItem.SetLabel("Unpin")
		pinItem.Connect("activate", func() {
			log.Infof("unpin %s", class)
			unpinTask(class)
		})
	}
	menu.Append(pinItem)

	menu.ShowAll()
	return *menu
}

func inPinned(taskID string) bool {
	for _, id := range pinned {
		if strings.TrimSpace(taskID) == strings.TrimSpace(id) {
			return true
		}
	}
	return false
}

func inTasks(pinID string) bool {
	for _, task := range clients {
		if strings.TrimSpace(task.Class) == strings.TrimSpace(pinID) {
			return true
		}
	}
	return false
}

func createImage(appID string, size int) (*gtk.Image, error) {
	name, err := getIcon(appID)
	if err != nil {
		name = appID
	}
	pixbuf, err := createPixbuf(name, size)
	if err != nil {
		return nil, err
	}
	image, _ := gtk.ImageNewFromPixbuf(pixbuf)

	return image, nil
}

func createPixbuf(icon string, size int) (*gdk.Pixbuf, error) {
	if strings.HasPrefix(icon, "/") {
		pixbuf, err := gdk.PixbufNewFromFileAtSize(icon, size, size)
		if err != nil {
			log.Errorf("%s", err)
			return nil, err
		}
		return pixbuf, nil
	}

	iconTheme, err := gtk.IconThemeGetDefault()
	if err != nil {
		log.Fatal("Couldn't get default theme: ", err)
	}
	pixbuf, err := iconTheme.LoadIcon(icon, size, gtk.ICON_LOOKUP_FORCE_SIZE)
	if err != nil {
		ico, err := getIcon(icon)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(ico, "/") {
			pixbuf, err := gdk.PixbufNewFromFileAtSize(ico, size, size)
			if err != nil {
				return nil, err
			}
			return pixbuf, nil
		}

		pixbuf, err := iconTheme.LoadIcon(ico, size, gtk.ICON_LOOKUP_FORCE_SIZE)
		if err != nil {
			return nil, err
		}
		return pixbuf, nil
	}
	return pixbuf, nil
}

func cacheDir() string {
	if os.Getenv("XDG_CACHE_HOME") != "" {
		return os.Getenv("XDG_CONFIG_HOME")
	}
	if os.Getenv("HOME") != "" && pathExists(filepath.Join(os.Getenv("HOME"), ".cache")) {
		p := filepath.Join(os.Getenv("HOME"), ".cache")
		return p
	}
	return ""
}

func tempDir() string {
	if os.Getenv("TMPDIR") != "" {
		return os.Getenv("TMPDIR")
	} else if os.Getenv("TEMP") != "" {
		return os.Getenv("TEMP")
	} else if os.Getenv("TMP") != "" {
		return os.Getenv("TMP")
	}
	return "/tmp"
}

func readTextFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func configDir() string {
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		return fmt.Sprintf("%s/nwg-dock-hyprland", os.Getenv("XDG_CONFIG_HOME"))
	}
	return fmt.Sprintf("%s/.config/nwg-dock-hyprland", os.Getenv("HOME"))
}

func createDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, os.ModePerm)
		if err == nil {
			log.Infof("Creating dir: %s", dir)
		}
	}
}

func copyFile(src, dst string) error {
	log.Infof("Copying file: %s", dst)

	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func getDataHome() string {
	if os.Getenv("XDG_DATA_HOME") != "" {
		return os.Getenv("XDG_DATA_HOME")
	}
	return "/usr/share/"
}

func getAppDirs() []string {
	var dirs []string
	xdgDataDirs := ""

	home := os.Getenv("HOME")
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if os.Getenv("XDG_DATA_DIRS") != "" {
		xdgDataDirs = os.Getenv("XDG_DATA_DIRS")
	} else {
		xdgDataDirs = "/usr/local/share/:/usr/share/"
	}
	if xdgDataHome != "" {
		dirs = append(dirs, filepath.Join(xdgDataHome, "applications"))
	} else if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local/share/applications"))
	}
	for _, d := range strings.Split(xdgDataDirs, ":") {
		dirs = append(dirs, filepath.Join(d, "applications"))
	}
	flatpakDirs := []string{filepath.Join(home, ".local/share/flatpak/exports/share/applications"),
		"/var/lib/flatpak/exports/share/applications"}

	for _, d := range flatpakDirs {
		if !isIn(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

func isIn(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func getIcon(appName string) (string, error) {
	appName = strings.Split(appName, " ")[0]
	if strings.HasPrefix(strings.ToUpper(appName), "GIMP") {
		return "gimp", nil
	}
	p := ""
	for _, d := range appDirs {
		path := filepath.Join(d, fmt.Sprintf("%s.desktop", appName))
		if pathExists(path) {
			p = path
			break
		} else if pathExists(strings.ToLower(path)) {
			p = strings.ToLower(path)
			break
		}
	}
	/* Some apps' class varies from their .desktop file name, e.g. 'gimp-2.9.9' or 'pamac-manager'.
	   Let's try to find a matching .desktop file name */
	if !strings.HasPrefix(appName, "/") && p == "" { // skip icon paths given instead of names
		p = searchDesktopDirs(appName)
	}

	if p != "" {
		lines, err := loadTextFile(p)
		if err != nil {
			return "", err
		}
		for _, line := range lines {
			if strings.HasPrefix(strings.ToUpper(line), "ICON") {
				return strings.Split(line, "=")[1], nil
			}
		}
	}
	return "", errors.New("couldn't find the icon")
}

func searchDesktopDirs(badAppID string) string {
	b4Separator := strings.Split(badAppID, "-")[0]
	for _, d := range appDirs {
		items, _ := os.ReadDir(d)
		for _, item := range items {
			if strings.Contains(item.Name(), b4Separator) {
				//Let's check items starting from 'org.' first
				if strings.Count(item.Name(), ".") > 1 && strings.HasSuffix(item.Name(),
					fmt.Sprintf("%s.desktop", badAppID)) {
					return filepath.Join(d, item.Name())
				} else {
					return ""
				}
			}
		}
	}
	// exceptions like "class": "VirtualBox Manager" & virtualbox.desktop
	b4Separator = strings.Split(badAppID, " ")[0]
	for _, d := range appDirs {
		items, _ := os.ReadDir(d)
		for _, item := range items {
			if strings.Contains(strings.ToUpper(item.Name()), strings.ToUpper(b4Separator)) {
				return filepath.Join(d, item.Name())
			}
		}
	}
	return ""
}

func getExec(appName string) (string, error) {
	cmd := appName
	if strings.HasPrefix(strings.ToUpper(appName), "GIMP") {
		cmd = "gimp"
	}
	for _, d := range appDirs {
		files, _ := os.ReadDir(d)
		path := ""
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".desktop") {
				if f.Name() == fmt.Sprintf("%s.desktop", appName) ||
					f.Name() == fmt.Sprintf("%s.desktop", strings.ToLower(appName)) {
					path = filepath.Join(d, f.Name())
					break
				}
			}
		}

		// as above in getIcon - for tasks w/ improper app_id
		if path == "" {
			path = searchDesktopDirs(appName)
		}

		if path != "" {
			lines, err := loadTextFile(path)
			if err != nil {
				return "", err
			}
			for _, line := range lines {
				if strings.HasPrefix(strings.ToUpper(line), "EXEC") {
					l := line[5:]
					cutAt := strings.Index(l, "%")
					if cutAt != -1 {
						l = l[:cutAt-1]
					}
					cmd = l
					break
				}
			}
			return cmd, nil
		}
	}
	return cmd, nil
}

func getName(appName string) string {
	name := appName
	for _, d := range appDirs {
		files, _ := os.ReadDir(d)
		path := ""
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".desktop") {
				if f.Name() == fmt.Sprintf("%s.desktop", appName) ||
					f.Name() == fmt.Sprintf("%s.desktop", strings.ToLower(appName)) {
					path = filepath.Join(d, f.Name())
					break
				}
			}
		}

		if path != "" {
			lines, err := loadTextFile(path)
			if err != nil {
				return name
			}
			for _, line := range lines {
				if strings.HasPrefix(strings.ToUpper(line), "NAME") {
					name = line[5:]
					break
				}
			}
		}
	}
	return name
}

func pathExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func loadTextFile(path string) ([]string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(bytes), "\n")
	var output []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			output = append(output, line)
		}

	}
	return output, nil
}

func pinTask(itemID string) {
	for _, item := range pinned {
		if item == itemID {
			println(item, "already pinned")
			return
		}
	}
	pinned = append(pinned, itemID)
	savePinned()
}

func unpinTask(itemID string) {
	pinned = remove(pinned, itemID)
	savePinned()
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func savePinned() {
	f, err := os.OpenFile(pinnedFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	for _, line := range pinned {
		if line != "" {
			_, err := f.WriteString(line + "\n")

			if err != nil {
				log.Errorf("Error saving pinned %s", err)
			}
		}
	}
}

func launch(ID string) {
	command, err := getExec(ID)
	if err != nil {
		log.Errorf("%s", err)
	}
	// remove quotation marks if any
	if strings.Contains(command, "\"") {
		command = strings.ReplaceAll(command, "\"", "")
	}

	elements := strings.Split(command, " ")

	// find prepended env variables, if any
	envVarsNum := strings.Count(command, "=")
	var envVars []string

	cmdIdx := -1

	if envVarsNum > 0 {
		for idx, item := range elements {
			if strings.Contains(item, "=") {
				envVars = append(envVars, item)
			} else if !strings.HasPrefix(item, "-") && cmdIdx == -1 {
				cmdIdx = idx
			}
		}
	}
	if cmdIdx == -1 {
		cmdIdx = 0
	}
	var args []string
	for _, arg := range elements[1+cmdIdx:] {
		if !strings.Contains(arg, "=") {
			args = append(args, arg)
		}
	}

	cmd := exec.Command(elements[cmdIdx], elements[1+cmdIdx:]...)

	// set env variables
	if len(envVars) > 0 {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, envVars...)
	}

	msg := fmt.Sprintf("env vars: %s; command: '%s'; args: %s\n", envVars, elements[cmdIdx], args)
	log.Info(msg)

	if err := cmd.Start(); err != nil {
		log.Error("Unable to launch command!", err.Error())
	}

	if *autohide {
		win.Hide()
	}
}

// Returns map output name -> gdk.Monitor
func mapOutputs() (map[string]*gdk.Monitor, error) {
	result := make(map[string]*gdk.Monitor)

	err := listMonitors()
	if err != nil {
		log.Fatalf("Error listing monitors: %v", err)
	}

	display, err := gdk.DisplayGetDefault()
	if err != nil {
		log.Fatalf("Error finding default GDK display: %v", err)
	}

	num := display.GetNMonitors()
	for i := 0; i < num; i++ {
		mon, _ := display.GetMonitor(i)
		geometry := mon.GetGeometry()
		// assign output to monitor on the basis of the same x, y coordinates
		for _, m := range monitors {
			if m.X == geometry.GetX() && m.Y == geometry.GetY() {
				result[m.Name] = mon
			}
		}
	}
	return result, nil
}

func listGdkMonitors() ([]gdk.Monitor, error) {
	var monitors []gdk.Monitor
	display, err := gdk.DisplayGetDefault()
	if err != nil {
		return nil, err
	}

	num := display.GetNMonitors()
	for i := 0; i < num; i++ {
		monitor, _ := display.GetMonitor(i)
		monitors = append(monitors, *monitor)
	}
	return monitors, nil
}

// Returns output of a CLI command with optional arguments
func getCommandOutput(command string) string {
	out, err := exec.Command("sh", "-c", command).Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

func isCommand(command string) bool {
	cmd := strings.Fields(command)[0]
	return getCommandOutput(fmt.Sprintf("command -v %s ", cmd)) != ""
}
