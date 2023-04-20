get:
	go get github.com/gotk3/gotk3
	go get github.com/gotk3/gotk3/gdk
	go get github.com/gotk3/gotk3/glib
	go get github.com/dlasky/gotk3-layershell/layershell
	go get github.com/joshuarubin/go-sway
	go get github.com/allan-simon/go-singleinstance
	go get "github.com/sirupsen/logrus"

build:
	go build -o bin/nwg-dock .

install:
	-pkill -f nwg-dock
	sleep 1
	mkdir -p /usr/share/nwg-dock
	cp -r images /usr/share/nwg-dock
	cp config/* /usr/share/nwg-dock
	cp bin/nwg-dock /usr/bin

uninstall:
	rm -r /usr/share/nwg-dock
	rm /usr/bin/nwg-dock

run:
	go run .
