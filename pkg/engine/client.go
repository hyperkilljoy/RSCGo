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
	"bufio"
	"fmt"
	"io"
	stdnet "net"
	"strings"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/spkaeros/rscgo/pkg/db"
	"github.com/spkaeros/rscgo/pkg/errors"
	"github.com/spkaeros/rscgo/pkg/game/net"
	"github.com/spkaeros/rscgo/pkg/game/net/handlers"
	"github.com/spkaeros/rscgo/pkg/game/world"
	"github.com/spkaeros/rscgo/pkg/log"
)

//client Represents a single connecting client.
type client struct {
	player     *world.Player
	socket     stdnet.Conn
	destroyer  sync.Once
	readWriter *bufio.ReadWriter
	websocket  bool
	reader   io.Reader
	readSize  int
	readLimit  int
	frameFin  bool
}

//startNetworking Starts up 3 new goroutines; one for reading incoming data from the socket, one for writing outgoing data to the socket, and one for client state updates and parsing plus handling incoming world.  When the client kill signal is sent through the kill channel, the state update and net handling goroutine will wait for both the reader and writer goroutines to complete their operations before unregistering the client.
func (c *client) startNetworking() {
	incomingPackets := make(chan *net.Packet, 20)
	awaitDeath := sync.WaitGroup{}

	go func() {
		defer awaitDeath.Done()
		defer c.player.Destroy()
		awaitDeath.Add(1)
		for {
			select {
			case p := <-c.player.OutgoingPackets:
				if p == nil {
					return
				}
				c.writePacket(*p)
			case <-c.player.KillC:
				return
			}
		}
	}()
	go func() {
		defer awaitDeath.Done()
		defer c.player.Destroy()
		awaitDeath.Add(1)
		for {
			select {
			default:
				p, err := c.readPacket()
				if err != nil {
					if err, ok := err.(errors.NetError); ok && err.Fatal {
						return
					}

					log.Warning.Printf("Rejected Packet from: %s", c.player.String())
					log.Warning.Println(err)
					continue
				}
				if !c.player.Connected() && p.Opcode != 32 && p.Opcode != 0 && p.Opcode != 2 && p.Opcode != 220 {
					log.Warning.Printf("Unauthorized packet[opcode:%v,len:%v] rejected from: %v\n", p.Opcode, len(p.FrameBuffer), c)
					return
				}
				incomingPackets <- p
			case <-c.player.KillC:
				return
			}
		}
	}()
	go func() {
		defer c.destroy()
		defer close(incomingPackets)
		defer awaitDeath.Wait()
		defer c.player.Destroy()
		for {
			select {
			case p := <-incomingPackets:
				if p == nil {
					log.Warning.Println("Tried processing nil packet!")
					continue
				}
				c.handlePacket(p)
			case <-c.player.KillC:
				return
			}
		}
	}()
}

//destroy Safely tears down a client, saves it to the database, and removes it from game-wide player list.
func (c *client) destroy() {
	c.destroyer.Do(func() {
		go func() {
			c.player.UpdateWG.RLock()
			c.player.SetConnected(false)
			if err := c.socket.Close(); err != nil {
				log.Error.Println("Couldn't close socket:", err)
			}
			close(c.player.OutgoingPackets)
			if player, ok := world.Players.FromIndex(c.player.Index); c.player.Index == -1 || (ok && player != c.player) || !ok {
				log.Warning.Printf("Unregistered: Unauthenticated connection ('%v'@'%v')\n", c.player.Username(), c.player.CurrentIP())
				if ok {
					log.Suspicious.Printf("Unauthenticated player being destroyed had index %d and there is a player that is assigned that index already! (%v)\n", c.player.Index, player)
				}
				return
			}
			c.player.Attributes.SetVar("lastIP", c.player.CurrentIP())
			world.RemovePlayer(c.player)
			db.DefaultPlayerService.PlayerSave(c.player)
			log.Info.Printf("Unregistered: %v\n", c.player.String())
			c.player.UpdateWG.RUnlock()
		}()
	})
}

//handlePacket Finds the mapped handler function for the specified net, and calls it with the specified parameters.
func (c *client) handlePacket(p *net.Packet) {
	handler := handlers.Handler(p.Opcode)
	if handler == nil {
		log.Info.Printf("Unhandled Packet: {opcode:%d; length:%d};\n", p.Opcode, len(p.FrameBuffer))
		fmt.Printf("CONTENT: %v\n", p.FrameBuffer)
		return
	}

	handler(c.player, p)
}

//newClient Creates a new instance of a client, launches goroutines to handle I/O for it, and returns a reference to it.
func newClient(socket stdnet.Conn, ws2 bool) *client {
	c := &client{socket: socket}
	c.player = world.NewPlayer(-1, strings.Split(socket.RemoteAddr().String(), ":")[0])
	c.websocket = ws2
	c.readWriter = bufio.NewReadWriter(bufio.NewReader(socket), bufio.NewWriter(socket))
	c.startNetworking()
	return c
}

//Write Writes data to the client's socket from `b`.  Returns the length of the written bytes.
func (c *client) Write(src []byte) int {
	var err error
	var dataLen int
	if c.websocket {
		err = wsutil.WriteServerBinary(c.socket, src)
		dataLen = len(src)
	} else {
		dataLen, err = c.socket.Write(src)
	}
	if err != nil {
		log.Error.Println("Problem writing to websocket client:", err)
		c.player.Destroy()
		return -1
	}
	return dataLen
}

//Read Reads data off of the client's socket into 'dst'.  Returns length read into dst upon success.  Otherwise, returns -1 with a meaningful error message.
func (c *client) Read(dst []byte) (int, error) {
	// set the read deadline for the socket to 10 seconds from now.
	err := c.socket.SetReadDeadline(time.Now().Add(time.Second * 10))
	if err != nil {
		return -1, errors.NewNetworkError("Connection closed", true)
	}

	if c.websocket {
		if c.readLimit <= c.readSize  {
			// reset buffer read index and create the next reader
			header, reader, err := wsutil.NextReader(c.readWriter, ws.StateServerSide)
			c.readSize = 0
			c.readLimit = int(header.Length)
			c.frameFin = header.Fin
			c.reader = reader
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF || strings.Contains(err.Error(), "connection reset by peer") || strings.Contains(err.Error(), "use of closed") {
					return -1, errors.NewNetworkError("Connection closed", true)
				} else if e, ok := err.(stdnet.Error); ok && e.Timeout() {
					return -1, errors.NewNetworkError("Connection timeout", true)
				} else {
					log.Warning.Println("Problem creating reader for next websocket frame:", err)
				}
				return -1, err
			}
		}
		
		n, err := c.reader.Read(dst)
		c.readSize += n
		if err == io.EOF && c.frameFin || err == nil {
			return n, nil
		}
		if err == io.ErrUnexpectedEOF || strings.Contains(err.Error(), "connection reset by peer") ||
				strings.Contains(err.Error(), "use of closed") || err == io.EOF && !c.frameFin {
			return -1, errors.NewNetworkError("Connection closed", true)
		}
		if e, ok := err.(stdnet.Error); ok && e.Timeout() {
			return -1, errors.NewNetworkError("Connection timeout", true)
		}
		log.Warning.Println(err)
		return -1, err
	}

	n, err := c.readWriter.Read(dst)
	if err != nil {
		log.Info.Println(err)
		if err == io.EOF || strings.Contains(err.Error(), "connection reset by peer") || strings.Contains(err.Error(), "use of closed") {
			return -1, errors.NewNetworkError("Connection closed", true)
		} else if e, ok := err.(stdnet.Error); ok && e.Timeout() {
			return -1, errors.NewNetworkError("Connection timeout", true)
		}
		return -1, err
	}
	return n, nil
}

//readPacket Attempts to read and parse the next 3 bytes of incoming data for the 16-bit length and 8-bit opcode of the next net frame the client is sending us.
func (c *client) readPacket() (p *net.Packet, err error) {
	header := make([]byte, 2)
	l, err := c.Read(header)
	if err != nil {
		return nil, err
	}
	if l < 2 {
		return nil, errors.NewNetworkError("SHORT_DATA", false)
	}
	length := int(header[0]) - 1
	bigLength := length >= 160
	if bigLength {
		length = (length-160)<<8 + int(header[1])
	}

	if length >= 4998 || length < 0 {
		log.Suspicious.Printf("Invalid packet length from [%v]: %d\n", c, length)
		log.Warning.Printf("Packet from [%v] length out of bounds; got %d, expected between 0 and 5000\n", c, length)
		return nil, errors.NewNetworkError("Packet length out of bounds; must be between 0 and 5000.", false)
	}

	payload := make([]byte, length)

	if length > 0 {
		if l, err := c.Read(payload); err != nil {
			return nil, err
		} else if l < length {
			return nil, errors.NewNetworkError("SHORT_DATA", false)
		}
	}

	if !bigLength {
		// If the length in the header used 1 byte, the 2nd byte in the header is the final byte of frame data
		payload = append(payload, header[1])
	}

	return net.NewPacket(payload[0], payload[1:]), nil
}

//writePacket This is a method to send a net to the client.  If this is a bare net, the net payload will
// be written as-is.  If this is not a bare packet, the packet will have the first 3 bytes changed to the
// appropriate values for the client to parse the length and opcode for this net.
func (c *client) writePacket(p net.Packet) {
	if p.HeaderBuffer == nil {
		c.Write(p.FrameBuffer)
		return
	}
	frameLength := len(p.FrameBuffer)
	if frameLength >= 0xA0 {
		p.HeaderBuffer[0] = byte(frameLength>>8 + 0xA0)
		p.HeaderBuffer[1] = byte(frameLength)
	} else {
		p.HeaderBuffer[0] = byte(frameLength)
		frameLength--
		p.HeaderBuffer[1] = p.FrameBuffer[frameLength]
	}
	c.Write(append(p.HeaderBuffer, p.FrameBuffer[:frameLength]...))
	return
}
