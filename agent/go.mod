module github.com/guimc233/JustPing/agent

go 1.22

require (
	github.com/gorilla/websocket v1.5.3
	github.com/guimc233/JustPing/shared v0.0.0
	github.com/prometheus-community/pro-bing v0.4.1
)

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
)

replace github.com/guimc233/JustPing/shared => ../shared
