package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type workspace struct {
	Id              int    `json:"id"`
	Name            string `json:"name"`
	Monitor         string `json:"monitor"`
	Windows         int    `json:"windows"`
	Hasfullscreen   bool   `json:"hasfullscreen"`
	Lastwindow      string `json:"lastwindow"`
	Lastwindowtitle string `json:"lastwindowtitle"`
}

type hyprPlugin struct {
	Name string `json:"name"`
}

type hyprAnimation struct {
	Name    string  `json:"name"`
	Enabled bool    `json:"enabled"`
	Speed   float64 `json:"speed"`
}

func layerCloseAnimationDuration() time.Duration {
	reply, err := hyprctl("j/animations")
	if err != nil {
		return 0
	}

	var response [][]hyprAnimation
	if json.Unmarshal(reply, &response) != nil || len(response) == 0 {
		return 0
	}

	var duration time.Duration
	for _, animation := range response[0] {
		if animation.Enabled && (animation.Name == "layersOut" || animation.Name == "fadeLayersOut") {
			animationDuration := time.Duration(animation.Speed * float64(100*time.Millisecond))
			if animationDuration > duration {
				duration = animationDuration
			}
		}
	}
	return duration
}

type vDesk struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Workspaces []int  `json:"workspaces"`
}

func isPluginLoaded(plugin string) bool {
	reply, err := hyprctl("j/plugin list")
	if err == nil {
		var plugins []hyprPlugin
		err = json.Unmarshal([]byte(reply), &plugins)
		if err == nil {
			for _, p := range plugins {
				if p.Name == plugin {
					return true
				}
			}
		}
	}
	return false
}

type monitor struct {
	Id              int     `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Make            string  `json:"make"`
	Model           string  `json:"model"`
	Serial          string  `json:"serial"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	RefreshRate     float64 `json:"refreshRate"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	ActiveWorkspace struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
	Reserved   []int   `json:"reserved"`
	Scale      float64 `json:"scale"`
	Transform  int     `json:"transform"`
	Focused    bool    `json:"focused"`
	DpmsStatus bool    `json:"dpmsStatus"`
	Vrr        bool    `json:"vrr"`
}

type client struct {
	Address   string `json:"address"`
	Mapped    bool   `json:"mapped"`
	Hidden    bool   `json:"hidden"`
	At        []int  `json:"at"`
	Size      []int  `json:"size"`
	Workspace struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	Floating       bool          `json:"floating"`
	Monitor        int           `json:"monitor"`
	Class          string        `json:"class"`
	Title          string        `json:"title"`
	InitialClass   string        `json:"initialClass"`
	InitialTitle   string        `json:"initialTitle"`
	Pid            int           `json:"pid"`
	Xwayland       bool          `json:"xwayland"`
	Pinned         bool          `json:"pinned"`
	Fullscreen     int           `json:"fullscreen"`
	FullscreenMode int           `json:"fullscreenMode"`
	FakeFullscreen bool          `json:"fakeFullscreen"`
	Grouped        []interface{} `json:"grouped"`
	Swallowing     interface{}   `json:"swallowing"`
}

func hyprctl(cmd string) ([]byte, error) {
	socketFile := fmt.Sprintf("%s/%s/.socket.sock", hyprDir, his)
	conn, err := net.Dial("unix", socketFile)
	if err != nil {
		return nil, err
	}

	message := []byte(cmd)
	_, err = conn.Write(message)
	if err != nil {
		return nil, err
	}

	reply := make([]byte, 102400)
	n, err := conn.Read(reply)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	return reply[:n], nil
}

func listMonitors() error {
	reply, err := hyprctl("j/monitors")
	if err != nil {
		return err
	} else {
		err = json.Unmarshal([]byte(reply), &monitors)
	}
	return err
}

func listClients() error {
	reply, err := hyprctl("j/clients")
	if err != nil {
		return err
	} else {
		err = json.Unmarshal([]byte(reply), &clients)
	}
	activeClient, _ = getActiveWindow()
	return err
}

func getActiveWindow() (*client, error) {
	var activeWindow client
	reply, err := hyprctl("j/activewindow")
	err = json.Unmarshal([]byte(reply), &activeWindow)
	if err == nil {
		return &activeWindow, nil
	}
	return nil, err
}

func dispatch(luaCmd string, legacyCmd string) {
	reply, err := hyprctl(luaCmd)

	// Fall back to legacy if connection error or invalid dispatcher
	if err != nil || strings.Contains(string(reply), "Invalid dispatcher") {
		reply, err = hyprctl(legacyCmd)
	}

	// If we got an error, well, log it but nothing more
	if err != nil {
		log.Debugf("%s -> %s", legacyCmd, reply)
	}
}

func focusWindow(address string) {
	if isPluginLoaded("virtual-desktops") {
		reply, err := hyprctl("j/printstate")
		if err == nil {
			var vdesks []vDesk
			json.Unmarshal([]byte(reply), &vdesks)

			var wsId = -1
			for _, c := range clients {
				if c.Address == address {
					wsId = c.Workspace.Id
					break
				}
			}
			if wsId != -1 {
				for _, vd := range vdesks {
					for _, ws := range vd.Workspaces {
						if ws == wsId {
							luaCmd := fmt.Sprintf("dispatch function() hl.plugin.virtual_desktops.vdesk('%d') end", vd.Id)
							legacyCmd := fmt.Sprintf("dispatch vdesk %d", vd.Id)
							dispatch(luaCmd, legacyCmd)
							break
						}
					}
				}
			}
		}
	}

	lua := fmt.Sprintf("dispatch hl.dsp.focus({ window = 'address:%s' })", address)
	legacy := fmt.Sprintf("dispatch focuswindow address:%s ", address)
	dispatch(lua, legacy)
}

func closeWindow(address string) {
	lua := fmt.Sprintf("dispatch hl.dsp.window.close({ window = 'address:%s' })", address)
	legacy := fmt.Sprintf("dispatch closewindow address:%s", address)
	dispatch(lua, legacy)
}

func toggleFloatingWindow(address string) {
	lua := fmt.Sprintf("dispatch hl.dsp.window.float({ window = 'address:%s', action = 'toggle' })", address)
	legacy := fmt.Sprintf("dispatch togglefloating address:%s", address)
	dispatch(lua, legacy)
}

func toggleFullscreenWindow(address string) {
	lua := fmt.Sprintf("dispatch hl.dsp.window.fullscreen({ window = 'address:%s', action = 'toggle' })", address)
	legacy := fmt.Sprintf("dispatch fullscreen address:%s", address)
	dispatch(lua, legacy)
}

func moveWindowToWorkspace(address string, workspace int) {
	lua := fmt.Sprintf("dispatch hl.dsp.window.move({ workspace = '%v', window = 'address:%v' })", workspace, address)
	legacy := fmt.Sprintf("dispatch movetoworkspace %d, address:%s", workspace, address)
	dispatch(lua, legacy)
}

func moveWindowToDesk(address string, desk int) {
	lua := fmt.Sprintf("dispatch function() hl.plugin.virtual_desktops.movetodesk('%d,address:%s') end", desk, address)
	legacy := fmt.Sprintf("dispatch movetodesk %d,address:%s", desk, address)
	dispatch(lua, legacy)
}
