get:
	go get github.com/diamondburned/gotk4-layer-shell/pkg/gtklayershell
	go get github.com/diamondburned/gotk4/pkg
	go get github.com/joshuarubin/go-sway
	go get github.com/allan-simon/go-singleinstance
	go get "github.com/sirupsen/logrus"

build:
	go build -v -o bin/nwg-dock-hyprland .

install:
	-pkill -f nwg-dock-hyprland
	sleep 1
	mkdir -p /usr/share/nwg-dock-hyprland
	cp -r images /usr/share/nwg-dock-hyprland
	cp config/* /usr/share/nwg-dock-hyprland
	cp bin/nwg-dock-hyprland /usr/bin

uninstall:
	rm -r /usr/share/nwg-dock-hyprland
	rm /usr/bin/nwg-dock-hyprland

run:
	go run .
