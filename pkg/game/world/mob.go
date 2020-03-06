package world

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/spkaeros/rscgo/pkg/game/entity"
	isaac "github.com/spkaeros/rscgo/pkg/rand"
	"github.com/spkaeros/rscgo/pkg/log"
)

type MobState = int

//MSIdle The default MobState, means doing nothing.
const (
	//MSIdle The default MobState, means doing nothing.
	MSIdle MobState = iota
	//MSBanking The mob is banking.
	MSBanking
	//MSChatting The mob is chatting with a NPC
	MSChatting
	//MSOptionMenu The mob is in an option menu.  The option menu handling routines will remove this state as soon
	// as they end, so if this is activated, there is an option menu waiting for a reply.
	MSOptionMenu
	//MSTrading The mob is negotiating a trade.
	MSTrading
	//MSDueling The mob is negotiating a duel.
	MSDueling
	//MSFighting The mob is fighting.
	MSFighting
	//MSBatching The mob is performing a skill that repeats itself an arbitrary number of times.
	MSBatching
	//MSSleeping The mob is using a bed or sleeping bag, and trying to solve a CAPTCHA
	MSSleeping
	//MSBusy Generic busy state
	MSBusy
	//MSChangingAppearance Indicates that the mob in this state is in the player aooearance changing screen
	MSChangingAppearance
	//MSShopping Indicates that the mob in this state is using a shop interface
	MSShopping
	//MSItemAction Indicates that the mob in this state is doing an inventory action
	MSItemAction
)

const (
	SyncSprite     = 1
	SyncMoved      = 1 << 1
	SyncRemoved    = 1 << 2
	SyncAppearance = 1 << 3

	SyncNeedsPosition = SyncRemoved | SyncMoved | SyncSprite
)

// mobSet a collection of entity.MobileEntitys
type mobSet []entity.MobileEntity

//MobList a container type for holding entity.MobileEntitys
type MobList struct {
	mobSet
	sync.RWMutex
}

//NewMobList returns a pointer to a newly pre-allocated MobList, with an initial capacity
// of 255.
func NewMobList() *MobList {
	return &MobList{mobSet: make(mobSet, 0, 255)}
}

//Add Adds a entity.MobileEntity to this MobList
func (l *MobList) Add(m entity.MobileEntity) int {
	l.Lock()
	defer l.Unlock()
	l.mobSet = append(l.mobSet, m)
	return len(l.mobSet)
}

//Range Runs action(entity.MobileEntity) for each entity.MobileEntity in the lists collection, until
// either running out of entries, or action returns true.
func (l *MobList) Range(action func(entity.MobileEntity) bool) int {
	l.RLock()
	defer l.RUnlock()
	for i, v := range l.mobSet {
		if action(v) {
			return i
		}
	}
	return -1
}

//Size returns the number of mobile entitys entered into this list.
func (l *MobList) Size() int {
	l.RLock()
	defer l.RUnlock()
	return len(l.mobSet)
}

//Size returns the number of mobile entitys entered into this list.
func (l *MobList) Contains(mob entity.MobileEntity) bool {
	return l.Range(func(m entity.MobileEntity) bool {
		return m == mob
	}) > -1
}

//Remove removes a entity.MobileEntity from this list and reslices the collection.
func (l *MobList) Remove(m entity.MobileEntity) bool {
	l.Lock()
	defer l.Unlock()
	for i, v := range l.mobSet {
		if v == m {
			if len(l.mobSet) >= i+1 {
				l.mobSet = append(l.mobSet[:i], l.mobSet[i+1:]...)
			} else {
				l.mobSet = l.mobSet[:i]
			}
			return true
		}
	}
	return false
}

func (l *MobList) RangePlayers(action func(*Player) bool) int {
	return l.Range(func(m entity.MobileEntity) bool {
		if p, ok := m.(*Player); ok {
			return action(p)
		}
		return false
	})
}

func (l *MobList) RangeNpcs(action func(*NPC) bool) int {
	return l.Range(func(m entity.MobileEntity) bool {
		if n, ok := m.(*NPC); ok {
			return action(n)
		}
		return false
	})
}

//Mob Represents a mobile entity within the game world.
type Mob struct {
	*Entity
	TransAttrs     *entity.AttributeList
	SyncMask       int
	ResetTickables []func()
	sync.RWMutex
}

func (p *Player) IsPlayer() bool {
	return true
}

func (p *Player) Type() entity.EntityType {
	return -1
}

func (p *Player) ServerIndex() int {
	return p.Index
}

func (n *NPC) ServerIndex() int {
	return n.Index
}

func (p *Player) IsNpc() bool {
	return false
}

func (n *NPC) IsPlayer() bool {
	return false
}

func (n *NPC) IsNpc() bool {
	return true
}

func (m *Mob) Transients() *entity.AttributeList {
	return m.TransAttrs
}

//Busy Returns true if this mobs state is anything other than idle. otherwise returns false.
func (m *Mob) Busy() bool {
	return m.State() != MSIdle && m.State() != MSItemAction
}

func (m *Mob) IsFighting() bool {
	return m.HasState(MSFighting)
}

func (m *Mob) FightTarget() entity.MobileEntity {
	return m.TransAttrs.VarMob("fightTarget")
}

func (m *Mob) SetFightTarget(m2 entity.MobileEntity) {
	m.TransAttrs.SetVar("fightTarget", m2)
}

func (m *Mob) FightRound() int {
	return m.TransAttrs.VarInt("fightRound", 0)
}

func (m *Mob) SetFightRound(i int) {
	m.TransAttrs.SetVar("fightRound", i)
}

func (m *Mob) LastRetreat() time.Time {
	return m.TransAttrs.VarTime("lastRetreat")
}

func (m *Mob) LastFight() time.Time {
	return m.TransAttrs.VarTime("lastFight")
}

func (m *Mob) UpdateLastRetreat() {
	m.TransAttrs.SetVar("lastRetreat", time.Now())
}

func (m *Mob) UpdateLastFight() {
	m.TransAttrs.SetVar("lastFight", time.Now())
}

//Direction Returns the mobs direction.
func (m *Mob) Direction() int {
	return m.TransAttrs.VarInt("direction", North)
}

//SetDirection Sets the mobs direction.
func (m *Mob) SetDirection(direction int) {
	m.TransAttrs.SetVar("direction", direction)
	m.SetSpriteUpdated()
}

//SetSpriteUpdated Sets the synchronization flag for whether this mob changed directions to true.
func (m *Mob) SetSpriteUpdated() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask |= SyncSprite
}

//SetRegionRemoved Sets the synchronization flag for whether this mob needs to be removed to true.
func (m *Mob) SetRegionRemoved() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask |= SyncRemoved
}

//UpdateSelf Sets the synchronization flag for whether this mob has moved to true.
func (m *Mob) SetRegionMoved() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask |= SyncMoved
}

func (m *Mob) SetAppearanceChanged() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask |= SyncAppearance
}

func (m *Mob) ResetRegionMoved() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask &= ^SyncMoved
}

func (m *Mob) ResetRegionRemoved() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask &= ^SyncRemoved
}

func (m *Mob) ResetAppearanceChanged() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask &= ^SyncAppearance
}

func (m *Mob) ResetSpriteUpdated() {
	m.Lock()
	defer m.Unlock()
	m.SyncMask &= ^SyncSprite
}

//SetPath Sets the mob's current pathway to path.  If path is nil, effectively resets the mobs path.
func (m *Mob) SetPath(path *Pathway) {
	m.TransAttrs.SetVar("path", path)
}

func (m *Mob) WalkTo(end Location) {
	path := MakePath(m.Location, end)
	m.SetPath(path)
}

//Path returns the path that this mob is trying to traverse.
func (m *Mob) Path() *Pathway {
	v, ok := m.TransAttrs.Var("path")
	if ok {
		return v.(*Pathway)
	}
	return nil
}

//ResetPath Sets the mobs path to nil, to stop the traversal of the path instantly
func (m *Mob) ResetPath() {
	m.TransAttrs.UnsetVar("path")
	m.TransAttrs.UnsetVar("pathLength")
}

//FinishedPath Returns true if the mobs path is nil, the paths current waypoint exceeds the number of waypoints available, or the next tile in the path is not a valid location, implying that we have reached our destination.
func (m *Mob) FinishedPath() bool {
	path := m.Path()
	if path == nil {
		return m.TransAttrs.VarInt("pathLength", 0) <= 0
	}
	return path.CurrentWaypoint >= path.countWaypoints() || !path.nextTileFrom(m.Location).IsValid()
}

//SetLocation Sets the mobs location.
func (m *Mob) SetLocation(location Location, teleport bool) {
	m.SetCoords(location.X(), location.Y(), teleport)
}

func (p *Player) SetLocation(l Location, teleport bool) {
	p.UpdateRegion(l.X(), l.Y())
	p.Mob.SetLocation(l, teleport)
}

func (n *NPC) SetLocation(l Location, teleport bool) {
	n.UpdateRegion(l.X(), l.Y())
	n.Mob.SetLocation(l, teleport)
}

//SetCoords Sets the mobs locations coordinates.
func (m *Mob) SetCoords(x, y int, teleport bool) {
	if !teleport {
		m.SetDirection(m.DirectionTo(x, y))
		m.SetRegionMoved()
	} else {
		m.SetRegionRemoved()
	}
	m.SetX(x)
	m.SetY(y)
}

func (p *Player) SetCoords(x, y int, teleport bool) {
	p.UpdateRegion(x, y)
	p.Mob.SetCoords(x, y, teleport)
}

func (n *NPC) SetCoords(x, y int, teleport bool) {
	n.UpdateRegion(x, y)
	n.Mob.SetCoords(x, y, teleport)
}

func (p *Player) Teleport(x, y int) {
	p.SetCoords(x, y, true)
}

func (n *NPC) Teleport(x, y int) {
	n.SetCoords(x, y, true)
}

func (m *Mob) State() int {
	return m.TransAttrs.VarInt("state", MSIdle)
}

//HasState Returns true if the mob has any of these states
func (m *Mob) HasState(state ...int) bool {
	return m.Transients().HasMasks("state", state...)
}

func (m *Mob) AddState(state int) {
	if state == MSIdle {
		m.Transients().SetVar("state", MSIdle)
		return
	}
	if m.HasState(state) {
		log.Warning.Println("Attempted to add a Mobstate that we already have:", state)
		return
	}
	m.Transients().StoreMask("state", state)
}

func (m *Mob) RemoveState(state int) {
	if state == MSIdle {
		return
	}
	if !m.HasState(state) {
		log.Warning.Println("Attempted to remove a Mobstate that we did not add:", state)
		return
	}
	m.Transients().RemoveMask("state", state)
}

//ResetFighting Resets melee fight related variables
func (m *Mob) ResetFighting() {
	target := m.TransAttrs.VarMob("fightTarget")
	if target != nil && target.IsFighting() {
		target.Transients().UnsetVar("fightTarget")
		target.Transients().UnsetVar("fightRound")
		target.SetDirection(North)
		target.RemoveState(MSFighting)
		if target.HasState(MSDueling) {
			target.RemoveState(MSDueling)
		}
		target.UpdateLastFight()
	}
	if m.IsFighting() {
		m.TransAttrs.UnsetVar("fightTarget")
		m.TransAttrs.UnsetVar("fightRound")
		m.SetDirection(North)
		m.RemoveState(MSFighting)
		if m.HasState(MSDueling) {
			m.RemoveState(MSDueling)
		}
		m.UpdateLastFight()
	}
}

//FightMode Returns the players current fight mode.
func (m *Mob) FightMode() int {
	return m.TransAttrs.VarInt("fight_mode", 0)
}

//SetFightMode Sets the players fightmode to i.  0=all,1=attack,2=defense,3=strength
func (m *Mob) SetFightMode(i int) {
	m.TransAttrs.SetVar("fight_mode", i)
}

//ArmourPoints Returns the players armour points.
func (m *Mob) ArmourPoints() int {
	return m.TransAttrs.VarInt("armour_points", 1)
}

//SetArmourPoints Sets the players armour points to i.
func (m *Mob) SetArmourPoints(i int) {
	m.TransAttrs.SetVar("armour_points", i)
}

//PowerPoints Returns the players power points.
func (m *Mob) PowerPoints() int {
	return m.TransAttrs.VarInt("power_points", 1)
}

//SetPowerPoints Sets the players power points to i
func (m *Mob) SetPowerPoints(i int) {
	m.TransAttrs.SetVar("power_points", i)
}

//AimPoints Returns the players aim points
func (m *Mob) AimPoints() int {
	return m.TransAttrs.VarInt("aim_points", 1)
}

//SetAimPoints Sets the players aim points to i.
func (m *Mob) SetAimPoints(i int) {
	m.TransAttrs.SetVar("aim_points", i)
}

//MagicPoints Returns the players magic points
func (m *Mob) MagicPoints() int {
	return m.TransAttrs.VarInt("magic_points", 1)
}

//SetMagicPoints Sets the players magic points to i
func (m *Mob) SetMagicPoints(i int) {
	m.TransAttrs.SetVar("magic_points", i)
}

//PrayerPoints Returns the players prayer points
func (m *Mob) PrayerPoints() int {
	return m.TransAttrs.VarInt("prayer_points", 1)
}

//SetPrayerPoints Sets the players prayer points to i
func (m *Mob) SetPrayerPoints(i int) {
	m.TransAttrs.SetVar("prayer_points", i)
}

//RangedPoints Returns the players ranged points.
func (m *Mob) RangedPoints() int {
	return m.TransAttrs.VarInt("ranged_points", 1)
}

//SetRangedPoints Sets the players ranged points tp i.
func (m *Mob) SetRangedPoints(i int) {
	m.TransAttrs.SetVar("ranged_points", i)
}

func (m *Mob) Skills() *entity.SkillTable {
	return m.TransAttrs.VarSkills("skills")
}

func (m *Mob) PrayerModifiers() [3]float64 {
	var modifiers = [...]float64{1.0, 1.0, 1.0}

	if m.TransAttrs.VarBool("prayer0", false) {
		modifiers[1] += .05
	}
	if m.TransAttrs.VarBool("prayer1", false) {
		modifiers[2] += .05
	}
	if m.TransAttrs.VarBool("prayer2", false) {
		modifiers[0] += .05
	}
	if m.TransAttrs.VarBool("prayer3", false) {
		modifiers[1] += .1
	}
	if m.TransAttrs.VarBool("prayer4", false) {
		modifiers[2] += .1
	}
	if m.TransAttrs.VarBool("prayer5", false) {
		modifiers[0] += .1
	}
	if m.TransAttrs.VarBool("prayer9", false) {
		modifiers[1] += .15
	}
	if m.TransAttrs.VarBool("prayer10", false) {
		modifiers[2] += .15
	}
	if m.TransAttrs.VarBool("prayer11", false) {
		modifiers[0] += .15
	}

	return modifiers
}

func (m *Mob) StyleBonus(stat int) int {
	mode := m.FightMode()
	if mode == 0 {
		return 1
	}
	if (mode == 1 && stat == entity.StatStrength) || (mode == 2 && stat == entity.StatAttack) || (mode == 3 && stat == entity.StatDefense) {
		return 3
	}
	return 0
}

//MaxMeleeDamage Calculates and returns the current max hit for this mob, based on many variables.
func (m *Mob) MaxMeleeDamage() float64 {
	return float64(((float64(m.Skills().Current(entity.StatStrength))*m.PrayerModifiers()[entity.StatStrength])+float64(m.StyleBonus(entity.StatStrength)))*((float64(m.PowerPoints())*0.00175)+0.1)) + 1.05
}

//AttackPoints Calculates and returns the accuracy capability of this mob, based on many variables, as a single variable.
func (m *Mob) AttackPoints() float64 {
	return float64(((float64(m.Skills().Current(entity.StatAttack))*m.PrayerModifiers()[entity.StatAttack])+float64(m.StyleBonus(entity.StatAttack)))*((float64(m.AimPoints())*0.00175)+0.1)) + 1.05
	//	return (float64(m.Skills().Current(StatAttack)) * m.PrayerModifiers()[StatAttack]) + float64(m.StyleBonus(StatAttack)+m.AimPoints())
}

//DefensePoints Calculates and returns the defensive capability of this mob, based on many variables, as a single variable.
func (m *Mob) DefensePoints() float64 {
	return float64(((float64(m.Skills().Current(entity.StatDefense))*m.PrayerModifiers()[entity.StatDefense])+float64(m.StyleBonus(entity.StatDefense)))*((float64(m.ArmourPoints())*0.00175)+0.1)) + 1.05
	//	return (float64(m.Skills().Current(StatDefense)) * m.PrayerModifiers()[StatDefense]) + float64(m.StyleBonus(StatDefense)+m.ArmourPoints())
}

//MeleeDamage Calculates and returns a melee damage from the receiver mob onto the target mob.
func (m *Mob) MeleeDamage(target entity.MobileEntity) int {
	//log.Info.Println((m.AttackPoints() / (target.DefensePoints() * 4)) * 100)
	if BoundedChance(m.AttackPoints()/(target.DefensePoints()*4)*100, 0.0, 82.0) {
		maxDamage := m.MaxMeleeDamage()
		var damage float64
		for damage > maxDamage || damage < 1 {
			//damage = rand.ExpFloat64()/(1/(maxDamage))
			damage = math.Floor((meleeRand.NormFloat64() * (maxDamage / 3)) + (maxDamage / 2))
		}
		return int(damage)
	}
	return 0
}
var meleeRand = rand.New(isaac.IsaacRng)
