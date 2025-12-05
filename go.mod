module github.com/ringsq/netboxvmsync

go 1.24.3

require (
	github.com/ringsq/vcenterapi v0.0.0-20240320174002-fd0df8347ac2
	github.com/rsapc/netbox v0.0.0-20251205151015-16d375370672
	github.com/srerun/go-proxmox-pdm v0.0.0-00010101000000-000000000000
)

require (
	github.com/buger/goterm v1.0.4 // indirect
	github.com/diskfs/go-diskfs v1.2.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/jinzhu/copier v0.3.4 // indirect
	github.com/magefile/mage v1.14.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	gopkg.in/djherbis/times.v1 v1.2.0 // indirect
)

require (
	github.com/go-resty/resty/v2 v2.11.0 // indirect
	github.com/joho/godotenv v1.5.1
	github.com/luthermonson/go-proxmox v0.0.0-beta6
	github.com/rsapc/hookcmd v0.0.0-20240228165245-7a165828a6f1 // indirect
	golang.org/x/exp v0.0.0-20240222234643-814bf88cf225 // indirect
	golang.org/x/net v0.21.0 // indirect
)

replace github.com/srerun/go-proxmox-pdm => ../../srerun/go-proxmox-pdm
