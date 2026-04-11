package pool

import (
	"fmt"
	"log"
	"sync"

	"github.com/deware-pk/go-mcbots/pkg/bot"
	"github.com/deware-pk/go-mcbots/pkg/protocol"
)

type BotPool struct {
	mu   sync.RWMutex
	bots map[string]*bot.Bot
}

func New() *BotPool {
	return &BotPool{
		bots: make(map[string]*bot.Bot),
	}
}

func (p *BotPool) Add(botID string, b *bot.Bot) {
	p.mu.Lock()
	defer p.mu.Unlock()

	b.SetOnClose(func() {
		p.remove(botID)
		log.Printf("[Pool] Bot %q auto-evicted (disconnected)", botID)
	})

	p.bots[botID] = b
}

func (p *BotPool) Get(botID string) (*bot.Bot, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	b, ok := p.bots[botID]
	return b, ok
}

// remove is the internal unlocked delete (used by auto-eviction).
func (p *BotPool) remove(botID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.bots, botID)
}

func (p *BotPool) Remove(botID string) {
	p.mu.Lock()
	b, ok := p.bots[botID]
	if ok {
		delete(p.bots, botID)
	}
	p.mu.Unlock()

	if ok {
		b.SetOnClose(nil) // prevent auto-eviction callback
		b.Close()
	}
}

func (p *BotPool) GetAllIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]string, 0, len(p.bots))
	for id := range p.bots {
		ids = append(ids, id)
	}
	return ids
}

func (p *BotPool) Execute(botID string, action func(b *bot.Bot) error) error {
	p.mu.RLock()
	b, ok := p.bots[botID]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("bot %q not found", botID)
	}
	return action(b)
}

func (p *BotPool) Launch(botID, name string, ver protocol.VersionInfo, addr string, setup func(b *bot.Bot)) error {
	if _, exists := p.Get(botID); exists {
		return fmt.Errorf("bot %q already exists", botID)
	}

	b := bot.New(name, ver)

	if setup != nil {
		setup(b)
	}

	p.Add(botID, b)

	go func() {
		if err := b.Connect(addr); err != nil {
			log.Printf("[Pool] Bot %q connection error: %v", botID, err)
		}
	}()

	return nil
}

func (p *BotPool) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, b := range p.bots {
		b.SetOnClose(nil) // prevent auto-eviction during shutdown
		b.Close()
		log.Printf("[Pool] Bot %q shut down", id)
	}

	p.bots = make(map[string]*bot.Bot)
}
