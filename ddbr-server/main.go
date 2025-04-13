package main

import (
	"log"
	sever "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
)

func main() {
	svr := sever.NewServer(new(ServerImpl))

	err := svr.Run()

	if err != nil {
		log.Println(err.Error())
	}
}
