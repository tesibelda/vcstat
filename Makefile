.DEFAULT_GOAL := build

build:
	go build -ldflags "-w -s -X main.Version=0.0.1" -o bin/vcstat cmd/main.go

buildwin:
	go build -ldflags "-w -s -X main.Version=0.0.1" -o bin\vcstat.exe cmd/main.go

run:
	./bin/vcstat -config ./etc/vcstat.conf

runwin:
	bin\vcstat -config etc\vcstat.conf
