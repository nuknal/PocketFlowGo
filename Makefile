.PHONY: ui server build

build: ui server

# build the ui
ui:
	cd ui && npm run build

#build the server
server:
	cd cmd/scheduler && go build -o pocketflowgo && mv pocketflowgo ../../
