package main

import (
	"encoding/json"
	"fmt"
	"net"
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
	Fullscreen     bool          `json:"fullscreen"`
	FullscreenMode int           `json:"fullscreenMode"`
	FakeFullscreen bool          `json:"fakeFullscreen"`
	Grouped        []interface{} `json:"grouped"`
	Swallowing     interface{}   `json:"swallowing"`
}

func hyprctl(cmd string) ([]byte, error) {
	socketFile := fmt.Sprintf("/tmp/hypr/%s/.socket.sock", his)
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
	activeClientAddr = getActiveWindow()
	return err
}

func getActiveWindow() string {
	var activeWindow client
	reply, err := hyprctl("j/activewindow")
	err = json.Unmarshal([]byte(reply), &activeWindow)
	if err == nil {
		return activeWindow.Address
	}
	return ""
}
