package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	button := gtk.NewButton()
	box.PackStart(button, false, false, 0)

	image, err := createImage(ID, imgSizeScaled)
	if err != nil || image == nil {
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/icon-missing.svg"),
			imgSizeScaled, imgSizeScaled)
		if err == nil {
			image = gtk.NewImageFromPixbuf(pixbuf)
		} else {
			image = gtk.NewImage()
		}
	}

	button.SetImage(image)
	button.SetImagePosition(gtk.PosTop)
	button.SetAlwaysShowImage(true)
	button.SetTooltipText(getName(ID))
	pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/task-empty.svg"),
		imgSizeScaled, imgSizeScaled/8)
	var img *gtk.Image
	if err == nil {
		img = gtk.NewImageFromPixbuf(pixbuf)
		box.PackStart(img, false, false, 0)
	}

	button.Connect("clicked", func() {
		launch(ID)
	})

	button.Connect("button-release-event", func(btn *gtk.Button, e *gdk.Event) bool {
		btnEvent := e.AsButton()
		if btnEvent.Button() == 1 || btnEvent.Button() == 2 {
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
	menu := gtk.NewMenu()
	menuItem := gtk.NewMenuItemWithLabel("Unpin")
	menuItem.Connect("activate", func() {
		unpinTask(taskID)
	})
	menu.Append(menuItem)

	menu.ShowAll()
	return *menu
}

func launcherButton() *gtk.Button {
	if !*noLauncher && *launcherCmd != "" {
		button := gtk.NewButton()
		var pixbuf *gdkpixbuf.Pixbuf
		var e error
		if *ico == "" {
			pixbuf, e = gdkpixbuf.NewPixbufFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/grid.svg"), imgSizeScaled, imgSizeScaled)
		} else {
			pixbuf, e = createPixbuf(*ico, imgSizeScaled)
		}
		if e == nil {
			image := gtk.NewImageFromPixbuf(pixbuf)
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
		}
		return button
	}
	return nil
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
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	button := gtk.NewButton()
	box.PackStart(button, false, false, 0)

	image, _ := createImage(t.Class, imgSizeScaled)
	if image == nil {
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/icon-missing.svg"),
			imgSizeScaled, imgSizeScaled)
		if err == nil {
			image = gtk.NewImageFromPixbuf(pixbuf)
		}
	}

	if image != nil {
		button.SetImage(image)
		button.SetImagePosition(gtk.PosBottom)
		button.SetAlwaysShowImage(true)
	}
	button.SetTooltipText(getName(t.Class))

	var img *gtk.Image
	if len(instances) < 2 {
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/task-single.svg"),
			imgSizeScaled, imgSizeScaled/8)
		if err == nil {
			img = gtk.NewImageFromPixbuf(pixbuf)
		}
	} else {
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(filepath.Join(dataHome, "nwg-dock-hyprland/images/task-multiple.svg"),
			imgSizeScaled, imgSizeScaled/8)
		if err == nil {
			img = gtk.NewImageFromPixbuf(pixbuf)
		}
	}
	if img != nil {
		box.PackStart(img, false, false, 0)
	}
	button.Connect("enter-notify-event", cancelClose)

	if len(instances) == 1 {
		button.Connect("event", func(btn *gtk.Button, e *gdk.Event) bool {
			btnEvent := e.AsButton()
			if btnEvent.Type() == gdk.ButtonReleaseType || btnEvent.Type() == gdk.TouchEndType {
				if btnEvent.Button() == 1 || btnEvent.Type() == gdk.TouchEndType {
					cmd := fmt.Sprintf("dispatch focuswindow address:%s", t.Address)
					if strings.HasPrefix(t.Workspace.Name, "special") {
						_, specialName, _ := strings.Cut(t.Workspace.Name, "special:")
						cmd = fmt.Sprintf("dispatch togglespecialworkspace %s", specialName)
					}
					reply, _ := hyprctl(cmd)
					log.Debugf("%s -> %s", cmd, reply)

					// fix #14
					cmd = "dispatch bringactivetotop"
					reply, _ = hyprctl(cmd)
					log.Debugf("%s -> %s", cmd, reply)

					return true
				} else if btnEvent.Button() == 2 {
					launch(t.Class)
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
			btnEvent := e.AsButton()
			if btnEvent.Button() == 1 {
				menu := clientMenu(t.Class, instances)
				menu.PopupAtWidget(button, widgetAnchor, menuAnchor, nil)
				return true
			} else if btnEvent.Button() == 2 {
				launch(t.Class)
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
	menu := gtk.NewMenu()

	iconName, err := getIcon(class)
	if err != nil {
		log.Warn(err)
	}
	for _, instance := range instances {
		menuItem := gtk.NewMenuItem()
		hbox := gtk.NewBox(gtk.OrientationHorizontal, 6)
		image := gtk.NewImageFromIconName(iconName, int(gtk.IconSizeMenu))
		hbox.PackStart(image, false, false, 0)
		title := instance.Title
		if len(title) > 25 {
			title = title[:25]
		}
		wsName := instance.Workspace.Name
		var label *gtk.Label
		label = gtk.NewLabel(fmt.Sprintf("%s (%v)", title, instance.Workspace.Name))
		hbox.PackStart(label, false, false, 0)
		menuItem.Add(hbox)
		menu.Append(menuItem)
		a := instance.Address
		menuItem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch focuswindow address:%s", a)
			if strings.HasPrefix(wsName, "special") {
				_, specialName, _ := strings.Cut(wsName, "special:")
				cmd = fmt.Sprintf("dispatch togglespecialworkspace %s", specialName)
			}
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)

			cmd = "dispatch bringactivetotop"
			reply, _ = hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

	}
	menu.ShowAll()
	return *menu
}

func clientMenuContext(class string, instances []client) gtk.Menu {
	menu := gtk.NewMenu()

	iconName, err := getIcon(class)
	if err != nil {
		log.Warnf("%s %s", err, class)
	}
	for _, instance := range instances {
		menuItem := gtk.NewMenuItem()
		hbox := gtk.NewBox(gtk.OrientationHorizontal, 6)
		image := gtk.NewImageFromIconName(iconName, int(gtk.IconSizeMenu))
		hbox.PackStart(image, false, false, 0)
		title := instance.Title
		if len(title) > 25 {
			title = title[:25]
		}
		// Clean non-ASCII chars
		//title = strings.Map(func(r rune) rune {
		//	if r > unicode.MaxASCII {
		//		return -1
		//	}
		//	return r
		//}, title)
		label := gtk.NewLabel(fmt.Sprintf("%s (%v)", title, instance.Workspace.Name))
		hbox.PackStart(label, false, false, 0)
		menuItem.Add(hbox)
		menu.Append(menuItem)
		submenu := gtk.NewMenu()

		a := instance.Address

		subitem := gtk.NewMenuItemWithLabel("closewindow")
		submenu.Append(subitem)
		subitem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch closewindow address:%s", a)
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

		subitem = gtk.NewMenuItemWithLabel("togglefloating")
		submenu.Append(subitem)
		subitem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch togglefloating address:%s", a)
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

		subitem = gtk.NewMenuItemWithLabel("fullscreen")
		submenu.Append(subitem)
		subitem.Connect("activate", func() {
			cmd := fmt.Sprintf("dispatch fullscreen address:%s", a)
			reply, _ := hyprctl(cmd)
			log.Debugf("%s -> %s", cmd, reply)
		})

		s := gtk.NewSeparatorMenuItem()
		submenu.Append(&s.MenuItem)

		for i := 1; i < int(*numWS)+1; i++ {
			subItem := gtk.NewMenuItemWithLabel(fmt.Sprintf("-> WS %v", i))
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
	separator := gtk.NewSeparatorMenuItem()
	menu.Append(&separator.MenuItem)

	item := gtk.NewMenuItemWithLabel("New window")
	item.Connect("activate", func() {
		launch(class)
	})
	menu.Append(item)

	closeAllWindows := gtk.NewMenuItem()
	closeAllWindows.SetLabel("Close all windows")
	closeAllWindows.Connect("activate", func() {
		for _, instance := range instances {
			address := instance.Address
			cmd := fmt.Sprintf("dispatch closewindow address:%s", address)
			reply, _ := hyprctl(cmd)
			log.Infof("%s -> %s", cmd, reply)
		}
	})
	menu.Append(closeAllWindows)

	pinItem := gtk.NewMenuItem()
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
	pixbuf, e := createPixbuf(name, size)
	if e != nil {
		return nil, err
	}
	image := gtk.NewImageFromPixbuf(pixbuf)

	return image, nil
}

func createPixbuf(icon string, size int) (*gdkpixbuf.Pixbuf, error) {
	if strings.HasPrefix(icon, "/") {
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(icon, size, size)
		if err != nil {
			log.Errorf("%s", err)
			return nil, err
		}
		return pixbuf, nil
	}

	iconTheme := gtk.IconThemeGetDefault()
	//if err != nil {
	//	log.Fatal("Couldn't get default theme: ", err)
	//}
	pixbuf, err := iconTheme.LoadIcon(icon, size, gtk.IconLookupForceSize)
	if err != nil {
		ico, err := getIcon(icon)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(ico, "/") {
			pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(ico, size, size)
			if err != nil {
				return nil, err
			}
			return pixbuf, nil
		}

		pixbuf, err := iconTheme.LoadIcon(ico, size, gtk.IconLookupForceSize)
		if err != nil {
			return nil, err
		}
		return pixbuf, nil
	}
	return pixbuf, nil
}

func cacheDir() string {
	if os.Getenv("XDG_CACHE_HOME") != "" {
		return os.Getenv("XDG_CACHE_HOME")
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

func getDataHome() (string, error) {
	var dirs []string
	home := os.Getenv("HOME")
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		dirs = append(dirs, xdgDataHome)
	} else if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local/share"))
	}

	var xdgDataDirs []string
	if os.Getenv("XDG_DATA_DIRS") != "" {
		xdgDataDirs = strings.Split(os.Getenv("XDG_DATA_DIRS"), ":")
	} else {
		xdgDataDirs = []string{"/usr/local/share/", "/usr/share/"}
	}
	dirs = append(dirs, xdgDataDirs...)

	for _, d := range dirs {
		if pathExists(filepath.Join(d, "nwg-dock-hyprland")) {
			return d, nil
		}
	}
	return "", errors.New("no data directory found for nwg-dock-hyprland")
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
				}
			}
		}
	}
	// exceptions like "class": "VirtualBox Manager" & virtualbox.desktop
	b4Separator = strings.Split(badAppID, " ")[0]
	for _, d := range appDirs {
		items, _ := os.ReadDir(d)

		// first look for exact 'class.desktop' file, see #31
		for _, item := range items {
			if strings.ToUpper(item.Name()) == strings.ToUpper(fmt.Sprintf("%s.desktop", badAppID)) {
				return filepath.Join(d, item.Name())
			}
		}

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

	display := gdk.DisplayGetDefault()
	if err != nil {
		log.Fatalf("Error finding default GDK display: %v", err)
	}

	num := display.NMonitors()
	for i := 0; i < num; i++ {
		mon := display.Monitor(i)
		result[monitors[i].Name] = mon
	}
	return result, nil
}

func listGdkMonitors() ([]gdk.Monitor, error) {
	var monitors []gdk.Monitor
	display := gdk.DisplayGetDefault()

	num := display.NMonitors()
	for i := 0; i < num; i++ {
		monitor := display.Monitor(i)
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

func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
