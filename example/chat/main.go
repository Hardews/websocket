package main

import (
	"gin"
	"log"
)

func main() {
	engine := gin.New()

	room := engine.NewGroup("")
	{
		room.GET("/new", NewRoom)
		room.GET("/join", JoinRoom)
		room.GET("/show", ShowRoom)
	}

	err := engine.Run()
	if err != nil {
		log.Fatalln(err)
	}
}
