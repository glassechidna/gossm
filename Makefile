all:
	go test ./...
	yarn
	yarn run webpack
	packr
	gox -arch="amd64" -os="windows linux darwin" ./...
	packr clean
