module lutexplorer

go 1.23

require (
	github.com/fsnotify/fsnotify v1.8.0
	github.com/klauspost/compress v1.17.11
	github.com/rs/cors v1.11.1
	stakergs v0.0.0
)

require github.com/gorilla/websocket v1.5.3

require golang.org/x/sys v0.13.0 // indirect

replace stakergs => ../stakergs
