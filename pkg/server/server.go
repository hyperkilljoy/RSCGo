package server

import (
	"bitbucket.org/zlacki/rscgo/pkg/server/clients"
	"bitbucket.org/zlacki/rscgo/pkg/server/script"
	"fmt"
	"github.com/gobwas/ws"
	"net"
	"os"
	"time"

	"bitbucket.org/zlacki/rscgo/pkg/server/config"
	"bitbucket.org/zlacki/rscgo/pkg/server/log"
	"bitbucket.org/zlacki/rscgo/pkg/server/world"
)

var (
	Kill = make(chan struct{})
)

func StartConnectionService() {
	bind := func(offset int) {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port()+offset))
		if err != nil {
			log.Error.Printf("Can't bind to specified port: %d\n", config.Port()+offset)
			log.Error.Println(err)
			os.Exit(1)
		}

		go func() {
			var wsUpgrader = ws.Upgrader{
				Protocol: func(protocol []byte) bool {
					// Chrome is picky, won't work without explicit protocol acceptance
					return true
				},
			}

			defer func() {
				err := listener.Close()
				if err != nil {
					log.Error.Println("Could not close server socket listener:", err)
					return
				}
			}()

			for {
				socket, err := listener.Accept()
				if err != nil {
					if config.Verbosity > 0 {
						log.Error.Println("Error occurred attempting to accept a client:", err)
					}
					continue
				}
				if offset != 0 {
					if _, err := wsUpgrader.Upgrade(socket); err != nil {
						log.Info.Println("Error upgrading websocket connection:", err)
						continue
					}
				}
				if clients.Size() >= config.MaxPlayers() {
					if n, err := socket.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 14}); err != nil || n != 9 {
						if config.Verbosity > 0 {
							log.Error.Println("Could not send world is full response to rejected client:", err)
						}
					}
					continue
				}

				c := NewClient(socket)
				if offset != 0 {
					c.Player().Websocket = true
				}
			}
		}()
	}

	bind(0) // UNIX sockets
	bind(1) // websockets
}

//Tick One game engine 'tick'.  This is to handle movement, to synchronize client, to update movement-related state variables... Runs once per 600ms.
func Tick() {
	clients.Range(func(c clients.Client) {
		if fn := c.Player().DistancedAction; fn != nil {
			if fn() {
				c.Player().ResetDistancedAction()
			}
		}
		c.Player().TraversePath()
	})
	world.UpdateNPCPositions()
	clients.Range(func(c clients.Client) {
		c.UpdatePositions()
	})
	clients.Range(func(c clients.Client) {
		c.ResetUpdateFlags()
	})
	world.ResetNpcUpdateFlags()
	fns := script.ActiveTriggers
	script.ActiveTriggers = script.ActiveTriggers[:0]
	for _, fn := range fns {
		go fn()
	}

}

//StartGameEngine Launches a goroutine to handle updating the state of the server every 600ms in a synchronized fashion.  This is known as a single game engine 'pulse'.
func StartGameEngine() {
	go func() {
		ticker := time.NewTicker(600 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			Tick()
		}
	}()
}

//Stop This will stop the server instance, if it is running.
func Stop() {
	log.Info.Println("Stopping server...")
	Kill <- struct{}{}
}
