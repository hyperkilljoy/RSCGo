/*
 * Copyright (c) 2019 Zachariah Knight <aeros.storkpk@gmail.com>
 *
 * Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 *
 */

package world

import (
	"os"
	"reflect"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/mattn/anko/core"
	"github.com/mattn/anko/env"
	"github.com/mattn/anko/parser"
	"github.com/spkaeros/rscgo/pkg/game/entity"
	"github.com/spkaeros/rscgo/pkg/log"
	"github.com/spkaeros/rscgo/pkg/rand"
	"github.com/spkaeros/rscgo/pkg/strutil"

	// Defines various package-related scripting utilities
	_ "github.com/mattn/anko/packages"
)

//CommandHandlers A map to assign in-game commands to the functions they should execute.
var CommandHandlers = make(map[string]func(*Player, []string))

func init() {
	env.Packages["world"] = map[string]reflect.Value{
		"getPlayer":              reflect.ValueOf(Players.FromIndex),
		"getPlayerByName":        reflect.ValueOf(Players.FromUserHash),
		"players":                reflect.ValueOf(Players),
		"getEquipmentDefinition": reflect.ValueOf(GetEquipmentDefinition),
		"replaceObject":          reflect.ValueOf(ReplaceObject),
		"addObject":              reflect.ValueOf(AddObject),
		"removeObject":           reflect.ValueOf(RemoveObject),
		"addNpc":                 reflect.ValueOf(AddNpc),
		"removeNpc":              reflect.ValueOf(RemoveNpc),
		"addItem":                reflect.ValueOf(AddItem),
		"removeItem":             reflect.ValueOf(RemoveItem),
		"getObjectAt":            reflect.ValueOf(GetObject),
		"getNpc":                 reflect.ValueOf(GetNpc),
		"checkCollisions":        reflect.ValueOf(IsTileBlocking),
		"tileData":               reflect.ValueOf(CollisionData),
		"kickPlayer": reflect.ValueOf(func(client *Player) {
			client.Destroy()
		}),
		"updateStarted": reflect.ValueOf(func() bool {
			return !UpdateTime.IsZero()
		}),
		"announce": reflect.ValueOf(func(msg string) {
			Players.Range(func(player *Player) {
				player.Message("@que@" + msg)
			})
		}),
		"walkTo": reflect.ValueOf(func(target *Player, x, y int) {
			target.WalkTo(NewLocation(x, y))
		}),
		"systemUpdate": reflect.ValueOf(func(t int) {
			UpdateTime = time.Now().Add(time.Second * time.Duration(t))
			go func() {
				time.Sleep(time.Second * time.Duration(t))
				Players.Range(func(player *Player) {
					player.Destroy()
				})
				time.Sleep(2 * time.Second)
				os.Exit(200)
			}()
			Players.Range(func(player *Player) {
				player.SendUpdateTimer()
			})
		}),
		"teleport": reflect.ValueOf(func(target *Player, x, y int, bubble bool) {
			if bubble {
				target.SendPacket(TeleBubble(0, 0))
				for _, nearbyPlayer := range target.NearbyPlayers() {
					nearbyPlayer.SendPacket(TeleBubble(target.X()-nearbyPlayer.X(), target.Y()-nearbyPlayer.Y()))
				}
			}
			plane := target.Plane()
			target.ResetPath()
			target.Teleport(x, y)
			if target.Plane() != plane {
				target.SendPacket(PlaneInfo(target))
			}
		}),
		"newShop":        reflect.ValueOf(NewShop),
		"newLocation":    reflect.ValueOf(NewLocation),
		"newGeneralShop": reflect.ValueOf(NewGeneralShop),
		"getShop":        reflect.ValueOf(Shops.Get),
		"hasShop":        reflect.ValueOf(Shops.Contains),
	}
	env.PackageTypes["world"] = map[string]reflect.Type{
		"players":    reflect.TypeOf(Players),
		"player":     reflect.TypeOf(&Player{}),
		"object":     reflect.TypeOf(&Object{}),
		"item":       reflect.TypeOf(&Item{}),
		"groundItem": reflect.TypeOf(&GroundItem{}),
		"npc":        reflect.TypeOf(&NPC{}),
		"location":   reflect.TypeOf(Location{}),
	}
	env.Packages["ids"] = map[string]reflect.Value{
		"COOKEDMEAT":               reflect.ValueOf(132),
		"BURNTMEAT":                reflect.ValueOf(134),
		"FLIER":                    reflect.ValueOf(201),
		"LEATHER_GLOVES":           reflect.ValueOf(16),
		"BOOTS":                    reflect.ValueOf(17),
		"SEAWEED":                  reflect.ValueOf(622),
		"OYSTER":                   reflect.ValueOf(793),
		"CASKET":                   reflect.ValueOf(549),
		"RAW_RAT_MEAT":             reflect.ValueOf(503),
		"RAW_SHRIMP":               reflect.ValueOf(349),
		"RAW_ANCHOVIES":            reflect.ValueOf(351),
		"RAW_TROUT":                reflect.ValueOf(358),
		"RAW_SALMON":               reflect.ValueOf(356),
		"RAW_PIKE":                 reflect.ValueOf(363),
		"RAW_SARDINE":              reflect.ValueOf(354),
		"RAW_HERRING":              reflect.ValueOf(361),
		"RAW_BASS":                 reflect.ValueOf(550),
		"RAW_MACKEREL":             reflect.ValueOf(552),
		"RAW_COD":                  reflect.ValueOf(554),
		"RAW_LOBSTER":              reflect.ValueOf(372),
		"RAW_SWORDFISH":            reflect.ValueOf(369),
		"RAW_TUNA":                 reflect.ValueOf(366),
		"HOLY_SYMBOL_OF_SARADOMIN": reflect.ValueOf(385),
		"RAW_SHARK":                reflect.ValueOf(545),
		"WOODEN_SHIELD":            reflect.ValueOf(4),
		"BRONZE_LSWORD":            reflect.ValueOf(70),
		"NET":                      reflect.ValueOf(376),
		"BIG_NET":                  reflect.ValueOf(548),
		"LOBSTER_POT":              reflect.ValueOf(375),
		"FISHING_ROD":              reflect.ValueOf(377),
		"FLYFISHING_ROD":           reflect.ValueOf(378),
		"OILY_FISHING_ROD":         reflect.ValueOf(589),
		"RAW_LAVA_EEL":             reflect.ValueOf(591),
		"HARPOON":                  reflect.ValueOf(379),
		"FISHING_BAIT":             reflect.ValueOf(380),
		"FEATHER":                  reflect.ValueOf(381),
		"BRONZE_PICKAXE":           reflect.ValueOf(156),
		"IRON_PICKAXE":             reflect.ValueOf(1258),
		"STEEL_PICKAXE":            reflect.ValueOf(1259),
		"MITHRIL_PICKAXE":          reflect.ValueOf(1260),
		"ADAM_PICKAXE":             reflect.ValueOf(1261),
		"RUNE_PICKAXE":             reflect.ValueOf(1262),
		"TIN_ORE":                  reflect.ValueOf(202),
		"SLEEPING_BAG":             reflect.ValueOf(1263),
		"NEEDLE":                   reflect.ValueOf(39),
		"THREAD":                   reflect.ValueOf(43),
		"FIRE_RUNE":                reflect.ValueOf(31),
		"WATER_RUNE":               reflect.ValueOf(32),
		"AIR_RUNE":                 reflect.ValueOf(33),
		"EARTH_RUNE":               reflect.ValueOf(34),
		"MIND_RUNE":                reflect.ValueOf(35),
		"BODY_RUNE":                reflect.ValueOf(36),
		"LIFE_RUNE":                reflect.ValueOf(37),
		"DEATH_RUNE":               reflect.ValueOf(38),
		"NATURE_RUNE":              reflect.ValueOf(40),
		"CHAOS_RUNE":               reflect.ValueOf(41),
		"LAW_RUNE":                 reflect.ValueOf(42),
		"COSMIC_RUNE":              reflect.ValueOf(46),
		"BLOOD_RUNE":               reflect.ValueOf(619),
		"AIR_STAFF":                reflect.ValueOf(101),
		"WATER_STAFF":              reflect.ValueOf(102),
		"EARTH_STAFF":              reflect.ValueOf(103),
		"FIRE_STAFF":               reflect.ValueOf(197),
		"FIRE_BATTLESTAFF":         reflect.ValueOf(615),
		"WATER_BATTLESTAFF":        reflect.ValueOf(616),
		"AIR_BATTLESTAFF":          reflect.ValueOf(617),
		"EARTH_BATTLESTAFF":        reflect.ValueOf(618),
		"E_FIRE_BATTLESTAFF":       reflect.ValueOf(682),
		"E_WATER_BATTLESTAFF":      reflect.ValueOf(683),
		"E_AIR_BATTLESTAFF":        reflect.ValueOf(684),
		"E_EARTH_BATTLESTAFF":      reflect.ValueOf(685),
		"BONES":                    reflect.ValueOf(20),
		"BANANA":                   reflect.ValueOf(249),
		"BAT_BONES":                reflect.ValueOf(604),
		"DRAGON_BONES":             reflect.ValueOf(614),
		"RUNE_2H":                  reflect.ValueOf(81),
		"RUNE_CHAIN":               reflect.ValueOf(400),
		"RUNE_PLATEBODY":           reflect.ValueOf(401),
		"RUNE_PLATETOP":            reflect.ValueOf(407),
		"BLACK_DAGGER":             reflect.ValueOf(423),
		"GOLD_RING":                reflect.ValueOf(283),
		"SILK":                     reflect.ValueOf(200),
		"IRON_ORE_CERTIFICATE":     reflect.ValueOf(517),
		"CHOCOLATE_SLICE":          reflect.ValueOf(336),
		"CHOCOLATE_BAR":            reflect.ValueOf(337),
		"SPINACH_ROLL":             reflect.ValueOf(179),
		"DRAGON_SWORD":             reflect.ValueOf(593),
		"DRAGON_AXE":               reflect.ValueOf(594),
		"DSTONE_AMULET_C":          reflect.ValueOf(597),
		"DSTONE_AMULET":            reflect.ValueOf(522),
		"DSTONE_AMULET_U":          reflect.ValueOf(522),
		"DRAGON_HELMET":            reflect.ValueOf(795),
		"DRAGON_SHIELD":            reflect.ValueOf(1278),
		"EASTER_EGG":               reflect.ValueOf(677),
		"CHRISTMAS_CRACKER":        reflect.ValueOf(575),
		"PARTYHAT_RED":             reflect.ValueOf(576),
		"PARTYHAT_YELLOW":          reflect.ValueOf(577),
		"PARTYHAT_BLUE":            reflect.ValueOf(578),
		"PARTYHAT_GREEN":           reflect.ValueOf(579),
		"PARTYHAT_PINK":            reflect.ValueOf(580),
		"PARTYHAT_WHITE":           reflect.ValueOf(581),
		"GREEN_MASK":               reflect.ValueOf(828),
		"RED_MASK":                 reflect.ValueOf(831),
		"BLUE_MASK":                reflect.ValueOf(832),
		"SANTA_HAT":                reflect.ValueOf(971),
		"PRESENT":                  reflect.ValueOf(980),
		"GNOME_BALL":               reflect.ValueOf(981),
		"BLURITE_ORE":              reflect.ValueOf(266),
		"CLAY":                     reflect.ValueOf(149),
		"COPPER_ORE":               reflect.ValueOf(150),
		"IRON_ORE":                 reflect.ValueOf(151),
		"GOLD":                     reflect.ValueOf(152),
		"SILVER":                   reflect.ValueOf(383),
		"GOLD2":                    reflect.ValueOf(690),
		"MITHRIL_ORE":              reflect.ValueOf(153),
		"ADAM_ORE":                 reflect.ValueOf(154),
		"RUNITE_ORE":               reflect.ValueOf(409),
		"COAL":                     reflect.ValueOf(155),
	}
	env.Packages["bind"] = map[string]reflect.Value{
		"onLogin": reflect.ValueOf(func(fn func(player *Player)) {
			LoginTriggers = append(LoginTriggers, fn)
		}),
		"invOnBoundary": reflect.ValueOf(func(fn func(player *Player, boundary *Object, item *Item) bool) {
			InvOnBoundaryTriggers = append(InvOnBoundaryTriggers, fn)
		}),
		"invOnPlayer": reflect.ValueOf(func(pred func(*Item) bool, fn func(player *Player, target *Player, item *Item)) {
			InvOnPlayerTriggers = append(InvOnPlayerTriggers, ItemOnPlayerTrigger{pred, fn})
		}),
		"invOnObject": reflect.ValueOf(func(fn func(player *Player, boundary *Object, item *Item) bool) {
			InvOnObjectTriggers = append(InvOnObjectTriggers, fn)
		}),
		"object": reflect.ValueOf(func(pred func(*Object, int) bool, fn func(player *Player, object *Object, click int)) {
			ObjectTriggers = append(ObjectTriggers, ObjectTrigger{pred, fn})
		}),
		"item": reflect.ValueOf(func(check func(item *Item) bool, fn func(player *Player, item *Item)) {
			ItemTriggers = append(ItemTriggers, ItemTrigger{check, fn})
		}),
		"boundary": reflect.ValueOf(func(pred func(*Object, int) bool, fn func(player *Player, object *Object, click int)) {
			BoundaryTriggers = append(BoundaryTriggers, ObjectTrigger{pred, fn})
		}),
		"npc": reflect.ValueOf(func(predicate func(npc *NPC) bool, fn func(player *Player, npc *NPC)) {
			NpcTriggers = append(NpcTriggers, NpcTrigger{predicate, fn})
		}),
		"spell": reflect.ValueOf(func(ident interface{}, fn func(player *Player, spell interface{})) {
			switch ident.(type) {
			case int64:
				SpellTriggers[int(ident.(int64))] = fn
			}
		}),
		"npcAttack": reflect.ValueOf(func(pred NpcActionPredicate, fn func(player *Player, npc *NPC)) {
			NpcAtkTriggers = append(NpcAtkTriggers, NpcBlockingTrigger{pred, fn})
		}),
		"npcKilled": reflect.ValueOf(func(pred NpcActionPredicate, fn func(player *Player, npc *NPC)) {
			NpcDeathTriggers = append(NpcDeathTriggers, NpcBlockingTrigger{pred, fn})
		}),
		"command": reflect.ValueOf(func(name string, fn func(p *Player, args []string)) {
			CommandHandlers[name] = fn
		}),
	}
	env.Packages["log"] = map[string]reflect.Value{
		"debug":  reflect.ValueOf(log.Info.Println),
		"debugf": reflect.ValueOf(log.Info.Printf),
		"warn":   reflect.ValueOf(log.Warning.Println),
		"warnf":  reflect.ValueOf(log.Warning.Printf),
		"err":    reflect.ValueOf(log.Error.Println),
		"errf":   reflect.ValueOf(log.Error.Printf),
		"cheat":  reflect.ValueOf(log.Suspicious.Println),
		"cheatf": reflect.ValueOf(log.Suspicious.Printf),
		"cmd":    reflect.ValueOf(log.Commands.Println),
		"cmdf":   reflect.ValueOf(log.Commands.Printf),
	}
}

func ScriptEnv() *env.Env {
	e := env.NewEnv()
	parser.EnableErrorVerbose()
	//parser.EnableDebug(2)
	e.Define("sleep", time.Sleep)
	e.Define("runAfter", time.AfterFunc)
	e.Define("after", time.After)
	e.Define("newProjectile", CreateProjectile)
	e.Define("tMinute", time.Second*60)
	e.Define("tHour", time.Second*60*60)
	e.Define("tSecond", time.Second)
	e.Define("tMillis", time.Millisecond)
	e.Define("ChatDelay", time.Millisecond*1800)
	e.Define("tNanos", time.Nanosecond)
	e.Define("ATTACK", entity.StatAttack)
	e.Define("DEFENSE", entity.StatDefense)
	e.Define("STRENGTH", entity.StatStrength)
	e.Define("HITPOINTS", entity.StatHits)
	e.Define("RANGED", entity.StatRanged)
	e.Define("PRAYER", entity.StatPrayer)
	e.Define("MAGIC", entity.StatMagic)
	e.Define("COOKING", entity.StatCooking)
	e.Define("WOODCUTTING", entity.StatWoodcutting)
	e.Define("FLETCHING", entity.StatFletching)
	e.Define("FISHING", entity.StatFishing)
	e.Define("FIREMAKING", entity.StatFiremaking)
	e.Define("CRAFTING", entity.StatCrafting)
	e.Define("SMITHING", entity.StatSmithing)
	e.Define("MINING", entity.StatMining)
	e.Define("HERBLAW", entity.StatHerblaw)
	e.Define("AGILITY", entity.StatAgility)
	e.Define("THIEVING", entity.StatThieving)
	e.Define("PRAYER_RAPID_RESTORE", 6)
	e.Define("PRAYER_RAPID_HEAL", 7)
	e.Define("ZeroTime", time.Time{})
	e.Define("itemDefs", ItemDefs)
	e.Define("objectDefs", ObjectDefs)
	e.Define("boundaryDefs", BoundaryDefs)
	e.Define("npcDefs", NpcDefs)
	e.Define("lvlToExp", entity.LevelToExperience)
	e.Define("expToLvl", entity.ExperienceToLevel)
	e.Define("withinWorld", WithinWorld)
	e.Define("skillIndex", entity.SkillIndex)
	e.Define("skillName", entity.SkillName)
	e.Define("newNpc", NewNpc)
	e.Define("newObject", NewObject)
	e.Define("base37", strutil.Base37.Encode)
	e.Define("rand", rand.Int31N)
	e.Define("tNow", time.Now)
	e.Define("North", North)
	e.Define("NorthEast", NorthEast)
	e.Define("NorthWest", NorthWest)
	e.Define("South", South)
	e.Define("SouthEast", SouthEast)
	e.Define("SouthWest", SouthWest)
	e.Define("East", East)
	e.Define("West", West)
	e.Define("parseDirection", ParseDirection)
	e.Define("contains", func(s []int64, elem int64) bool {
		for _, v := range s {
			if v == elem {
				return true
			}
		}
		return false
	})
	e.Define("gatheringSuccess", func(req, cur int) bool {
		if cur < req {
			return false
		}
		return float64(rand.Int31N(1, 128)) <= (float64(cur)+40)-(float64(req)*1.5)
	})
	e.Define("roll", Chance)
	e.Define("boundedRoll", BoundedChance)
	e.Define("weightedChance", WeightedChoice)

	e.Define("npcPredicate", func(ids ...interface{}) func(*NPC) bool {
		return func(npc *NPC) bool {
			for _, id := range ids {
				if cmd, ok := id.(string); ok {
					if npc.Name() == cmd {
						return true
					}
				} else if id, ok := id.(int64); ok {
					if npc.ID == int(id) {
						return true
					}
				}
			}
			return false
		}
	})
	e.Define("npcBlockingPredicate", func(ids ...interface{}) func(*Player, *NPC) bool {
		return func(player *Player, npc *NPC) bool {
			for _, id := range ids {
				if cmd, ok := id.(string); ok {
					if npc.Name() == cmd {
						return true
					}
				} else if id, ok := id.(int64); ok {
					if npc.ID == int(id) {
						return true
					}
				}
			}
			return false
		}
	})

	e.Define("fuzzyFindItem", func(input string) []map[string]interface{} {
		var itemList []map[string]interface{}
		for id, item := range ItemDefs {
			if fuzzy.MatchFold(input, item.Name) {
				itemList = append(itemList, map[string]interface{}{"name": item.Name, "id": id})
			}
		}
		return itemList
	})

	e.Define("itemPredicate", func(ids ...interface{}) func(*Item) bool {
		return func(item *Item) bool {
			for _, id := range ids {
				if cmd, ok := id.(string); ok {
					if item.Command() == cmd {
						return true
					}
				} else if id, ok := id.(int64); ok {
					if item.ID == int(id) {
						return true
					}
				}
			}
			return false
		}
	})
	e.Define("objectPredicate", func(ids ...interface{}) func(*Object, int) bool {
		return func(object *Object, click int) bool {
			for _, id := range ids {
				if cmd, ok := id.(string); ok {
					if ObjectDefs[object.ID].Commands[click] == cmd {
						return true
					}
				} else if id, ok := id.(int64); ok {
					if object.ID == int(id) {
						return true
					}
				}
			}
			return false
		}
	})
	e.Define("asPlayer", func(m entity.MobileEntity) *Player {
		if m.IsPlayer() {
			return m.(*Player)
		}
		
		return nil
	})
	e.Define("asNpc", func(m entity.MobileEntity) *NPC {
		if m.IsNpc() {
			return m.(*NPC)
		}
		
		return nil
	})
	e = core.Import(e)
	return e
}
