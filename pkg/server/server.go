package server

import (
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"bitbucket.org/zlacki/rscgo/pkg/entity"
	"bitbucket.org/zlacki/rscgo/pkg/list"
	"bitbucket.org/zlacki/rscgo/pkg/server/packets"
)

var (
	listener net.Listener
	//LogWarning Log interface for warnings.
	LogWarning = log.New(os.Stdout, "[WARNING] ", log.Ltime|log.Lshortfile)
	//LogInfo Log interface for debug information.
	LogInfo = log.New(os.Stdout, "[INFO] ", log.Ltime|log.Lshortfile)
	//LogError Log interface for errors.
	LogError   = log.New(os.Stderr, "[ERROR] ", log.Ltime|log.Lshortfile)
	syncTicker = time.NewTicker(time.Millisecond * 650)
	kill       = make(chan struct{})
	//ClientList List of active clients.
	ClientList = list.New(2048)
	//Clients A map of base37 encoded username hashes to client references.  This is a common lookup and I consider this an optimization.
	Clients = make(map[uint64]*Client)
)

//TomlConfig A data structure representing the RSCGo TOML configuration file.
var TomlConfig struct {
	DataDir    string `toml:"data_directory"`
	Version    int    `toml:"version"`
	Port       int    `toml:"port"`
	MaxPlayers int    `toml:"max_players"`
	Database   struct {
		playerDB string `toml:"player_db"`
		worldDB  string `toml:"world_db"`
	} `toml:"database"`
	Crypto struct {
		RsaKeyFile string `toml:"rsa_key"`
		HashSalt   string `toml:"hash_salt"`
	} `toml:"crypto"`
}

//Flags This is used to interface with the go-flags package from some guy on github.
var Flags struct {
	Verbose   []bool `short:"v" long:"verbose" description:"Display more verbose output"`
	Port      int    `short:"p" long:"port" description:"The port for the server to listen on," default:"43591"`
	Config    string `short:"c" long:"config" description:"Specify the configuration file to load server settings from" default:"config.toml"`
	UseCipher bool   `short:"e" long:"encryption" description:"Enable command opcode encryption using ISAAC to encrypt packet opcodes."`
}

func bind(port int) {
	var err error
	portS := strconv.Itoa(port)
	listener, err = net.Listen("tcp", ":"+portS)
	if err != nil {
		LogError.Printf("Can't bind to specified port: %d\n", port)
		LogError.Println(err)
		os.Exit(1)
	}
}

func startConnectionService() {
	if listener == nil {
		if len(Flags.Verbose) > 0 {
			LogWarning.Println("Attempted to start connection service without a listener!  This shouldn't happen.")
			LogWarning.Println("Starting listener on default port...")
		}
		bind(43591)
	}

	go func() {
		defer listener.Close()
		// TODO: Can this ticker be made smaller safely?
		connTicker := time.NewTicker(time.Millisecond * 50)
		for range connTicker.C {
			socket, err := listener.Accept()
			if err != nil {
				if len(Flags.Verbose) > 0 {
					LogError.Println("Could not accept connection via server listener.")
					LogError.Println(err)
				}
				return
			}

			client := NewClient(socket)
			client.index = ClientList.Add(client)
			if client.index == -1 {
				LogWarning.Printf("Problem adding client to client list.  Current size:%d, max size:2048\n", ClientList.Size())
			}
		}
	}()

}

//Start Listens for and processes new clients connecting to the server.
// This method blocks while the server is running.
func Start() {
	LogInfo.Println("RSCGo starting up...")
	count := LoadObjects()
	if len(Flags.Verbose) > 0 {
		if count > 0 {
			LogInfo.Printf("Loaded %d game objects.\n", count)
		}
		LogInfo.Print("Attempting to bind to network...")
	}
	bind(Flags.Port)
	if len(Flags.Verbose) > 0 {
		LogInfo.Println("done")
		LogInfo.Print("Attempting to start connection service...")
	}
	startConnectionService()
	if len(Flags.Verbose) > 0 {
		LogInfo.Println("done")
		LogInfo.Print("Attempting to start synchronized task service...")
	}
	startSynchronizedTaskService()
	LogInfo.Printf("done\n\n")
	LogInfo.Println("RSCGo is now running.")
	LogInfo.Printf("Listening on port %d...\n", Flags.Port)
	// TODO: Probably need to handle certain signals, for usability sake.
	// TODO: Implement a data store for dynamic game data, e.g player information, and so on.
	select {
	case <-kill:
		os.Exit(0)
	}
}

//startSynchronizedTaskService Launches a goroutine to handle updating the state of the server every 650ms in a
// synchronized fashion.  This is known as a single game engine 'pulse'.  All mobile entities must have their position
// updated during this pulse to be compatible with Jagex RSClassic Client software.
// TODO: Can movement be handled concurrently per-player safely on the Jagex Client? Mob movement might not look right.
func startSynchronizedTaskService() {
	go func() {
		for range syncTicker.C {
			// Loop once to actually move the mobs
			var wg sync.WaitGroup
			wg.Add(ClientList.Size())
			for _, c := range ClientList.Values {
				if c, ok := c.(*Client); ok {
					go func() {
						defer wg.Done()
						c.player.TraversePath()
					}()
				}
			}
			wg.Wait()
			// Loop again to update the clients about what the mobs have been up to in the prior loop.
			wg.Add(ClientList.Size())
			for _, c := range ClientList.Values {
				if c, ok := c.(*Client); ok {
					go func() {
						defer wg.Done()
						if c.player.X() == 0 && c.player.Y() == 0 {
							return
						}
						localRegions := entity.SurroundingRegions(c.player.X(), c.player.Y())
						var localPlayers []*entity.Player
						var localAppearances []*entity.Player
						var removingPlayers []*entity.Player
						var localObjects []*entity.Object
						var removingObjects []*entity.Object
						for _, r := range localRegions {
							for _, p := range r.Players {
								if p.Index != c.index {
									if c.player.Location().LongestDelta(p.Location()) <= 15 {
										if !c.player.LocalPlayers.ContainsPlayer(p) {
											localPlayers = append(localPlayers, p)
										}
									} else {
										if c.player.LocalPlayers.ContainsPlayer(p) {
											removingPlayers = append(removingPlayers, p)
										}
									}
								}
							}
							for _, o := range r.Objects {
								if c.player.Location().LongestDelta(o.Location()) <= 20 {
									if !c.player.LocalObjects.ContainsObject(o) {
										localObjects = append(localObjects, o)
									}
								} else {
									if c.player.LocalObjects.ContainsObject(o) {
										removingObjects = append(removingObjects, o)
									}
								}
							}
						}
						// TODO: Clean up appearance list code.
						for _, index := range c.player.Appearances {
							v := ClientList.Get(index)
							if v, ok := v.(*Client); ok {
								localAppearances = append(localAppearances, v.player)
							}
						}
						localAppearances = append(localAppearances, localPlayers...)
						c.player.Appearances = c.player.Appearances[:0]
						// POSITIONS BEFORE EVERYTHING ELSE.
						if positions := packets.PlayerPositions(c.player, localPlayers, removingPlayers); positions != nil {
							// Only send when needed
							c.outgoingPackets <- positions
						}
						if appearances := packets.PlayerAppearances(c.player, localAppearances); appearances != nil {
							c.outgoingPackets <- appearances
						}
						c.outgoingPackets <- packets.ObjectLocations(c.player, localObjects, removingObjects)
						// TODO: Update movement, update client-side collections
					}()
				}
			}
			wg.Wait()
			wg.Add(ClientList.Size())
			for _, c := range ClientList.Values {
				if c, ok := c.(*Client); ok {
					go func() {
						defer wg.Done()
						c.player.Removing = false
						c.player.HasMoved = false
						c.player.AppearanceChanged = false
						c.player.HasSelf = true
					}()
				}
			}
			wg.Wait()
		}
	}()
}

//Stop This will stop the server instance, if it is running.
func Stop() {
	LogInfo.Printf("Clearing client list...")
	ClientList.Clear()
	LogInfo.Println("done")
	LogInfo.Println("Stopping server...")
	kill <- struct{}{}
}
