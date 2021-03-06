/*
 * Copyright (c) 2020 Zachariah Knight <aeros.storkpk@gmail.com>
 *
 * Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 *
 */

package engine

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"reflect"
	`strconv`
	"time"
	
	"github.com/gobwas/ws"
	
	"github.com/spkaeros/rscgo/pkg/config"
	"github.com/spkaeros/rscgo/pkg/engine/tasks"
	"github.com/spkaeros/rscgo/pkg/game/world"
	"github.com/spkaeros/rscgo/pkg/log"
	_ "github.com/spkaeros/rscgo/pkg/game/net/handshake"
)

var (
	//Kill a signaling channel for killing the main server
	Kill = make(chan struct{})
)

//Bind binds to the TCP port at port, and the websocket port at port+1.
func Bind(port int) {
	listener, err := net.Listen("tcp", ":" + strconv.Itoa(port))
	if err != nil {
		log.Error.Printf("Can't bind to specified port: %d\n", port)
		log.Error.Println(err)
		os.Exit(1)
	}
	go func() {
		var wsUpgrader = ws.Upgrader{
			Protocol: func(protocol []byte) bool {
				// Chrome is picky, won't work without explicit protocol acceptance
				return true
			},
			ReadBufferSize:  5000,
			WriteBufferSize: 5000,
		}


		defer func() {
			err := listener.Close()
			if err != nil {
				log.Error.Println("Could not close game socket listener:", err)
			}
		}()

		certChain, certErr := tls.LoadX509KeyPair("./data/ssl/fullchain.pem", "./data/ssl/privkey.pem")

		for {
			socket, err := listener.Accept()
			if err != nil {
				if config.Verbosity > 0 {
					log.Error.Println("Error occurred attempting to accept a client:", err)
				}
				continue
			}
			if port == config.WSPort() {
				if certErr == nil {
					// set up socket to use TLS if we have certs that we can load
					socket = tls.Server(socket, &tls.Config{Certificates: []tls.Certificate{certChain}, InsecureSkipVerify: true})
				}
				if _, err := wsUpgrader.Upgrade(socket); err != nil {
					log.Error.Println("Encountered a problem upgrading the websocket:", err)
					continue
				}
			}
			newClient(socket, port == config.WSPort())
		}
	}()
}

//StartConnectionService Binds and listens for new clients on the configured ports
func StartConnectionService() {
	Bind(config.Port())   // UNIX sockets
	Bind(config.WSPort()) // websockets
}

func runTickables(p *world.Player) {
	var toRemove []int
	for i, fn := range p.Tickables {
		if realFn, ok := fn.(func(context.Context) (reflect.Value, reflect.Value)); ok {
			_, err := realFn(context.Background())
			if !err.IsNil() {
				toRemove = append(toRemove, i)
				log.Warning.Println("Error in tickable:", err)
				continue
			}
		}
		if realFn, ok := fn.(func(context.Context, reflect.Value) (reflect.Value, reflect.Value)); ok {
			_, err := realFn(context.Background(), reflect.ValueOf(p))
			if !err.IsNil() {
				toRemove = append(toRemove, i)
				log.Warning.Println("Error in tickable:", err)
				continue
			}
		}
		if realFn, ok := fn.(func()); ok {
			realFn()
		}
		if realFn, ok := fn.(func() bool); ok {
			if realFn() {
				toRemove = append(toRemove, i)
			}
		}
		if realFn, ok := fn.(func(*world.Player)); ok {
			realFn(p)
		}
		if realFn, ok := fn.(func(*world.Player) bool); ok {
			if realFn(p) {
				toRemove = append(toRemove, i)
			}
		}
	}
	for _, idx := range toRemove {
		p.Tickables[idx] = nil
		p.Tickables = p.Tickables[:idx]
		if idx < len(p.Tickables)-1 {
			p.Tickables = append(p.Tickables[idx+1:])
		}
	}
}

//Tick One game engine 'tick'.  This is to handle movement, to synchronize client, to update movement-related state variables... Runs once per 640ms.
func Tick() {
	tasks.Tickers.RunSynchronous()

	world.Players.Range(func(p *world.Player) {
		p.UpdateWG.Lock()
		runTickables(p)
		if fn := p.DistancedAction; fn != nil {
			if fn() {
				p.ResetDistancedAction()
			}
		}
		p.TraversePath()
	})

	world.UpdateNPCPositions()

	world.Players.Range(func(p *world.Player) {
		// Everything is updated relative to our player's position, so player position net comes first
		if positions := world.PlayerPositions(p); positions != nil {
			p.SendPacket(positions)
		}
		if appearances := world.PlayerAppearances(p); appearances != nil {
			p.SendPacket(appearances)
		}
		if npcUpdates := world.NPCPositions(p); npcUpdates != nil {
			p.SendPacket(npcUpdates)
		}
		if objectUpdates := world.ObjectLocations(p); objectUpdates != nil {
			p.SendPacket(objectUpdates)
		}
		if boundaryUpdates := world.BoundaryLocations(p); boundaryUpdates != nil {
			p.SendPacket(boundaryUpdates)
		}
		if itemUpdates := world.ItemLocations(p); itemUpdates != nil {
			p.SendPacket(itemUpdates)
		}
		if clearDistantChunks := world.ClearDistantChunks(p); clearDistantChunks != nil {
			p.SendPacket(clearDistantChunks)
		}
	})

	//	world.Players.Range(func(p *world.Player) {
	//		for _, fn := range p.ResetTickables {
	//			fn()
	//		}
	//		p.ResetTickables = p.ResetTickables[:0]
	//	})
	world.Players.Range(func(p *world.Player) {
		p.ResetRegionRemoved()
		p.ResetRegionMoved()
		p.ResetSpriteUpdated()
		p.ResetAppearanceChanged()
		p.UpdateWG.Unlock()
	})
	world.ResetNpcUpdateFlags()
}

//StartGameEngine Launches a goroutine to handle updating the state of the game every 640ms in a synchronized fashion.  This is known as a single game engine 'pulse'.
func StartGameEngine() {
	go func() {
		ticker := time.NewTicker(640 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			Tick()
		}
	}()
}

//Stop This will stop the game instance, if it is running.
func Stop() {
	log.Info.Println("Stopping ..")
	Kill <- struct{}{}
}
