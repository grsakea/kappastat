package backend

import (
	"errors"
	"github.com/mrshankly/go-twitch/twitch"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
	"os"
)

func (c *Controller) Loop() {
	log.Print("Start Loop")

	go loopViewers(c.twitchAPI, c.cViewer, c.infosViewer)
	go loopChat(c.cChat, c.infosChat)
	go loopStat(c.cStat, c.storage.db)

	for {
		select {
		case temp, ok := <-c.infosViewer:
			if !ok {
				log.Println("InfosViewer failed")
				return
			}
			storeViewerCount(c.storage.views, temp)

		case temp, ok := <-c.infosChat:
			if !ok {
				log.Println("InfosChat failed")
				return
			}
			storeChatEntry(c.storage.chat, temp)
		}
	}
	log.Println("Loop failed")
}

func SetupController(dbName string) (contr *Controller) {
	store := StorageController{
		db: setupStorage(dbName),
	}
	store.views = store.db.C("viewer_count")
	store.chat = store.db.C("chat_entries")
	store.follow = store.db.C("follow")

	contr = &Controller{
		config:      LoadConfig("config.json"),
		infosChat:   make(chan ChatEntry),
		infosViewer: make(chan ViewerCount),
		cViewer:     make(chan Message),
		cChat:       make(chan Message),
		cStat:       make(chan Message),
		tracked:     make(map[string]bool),
		storage:     store,
		twitchAPI:   twitch.NewClient(&http.Client{}),
	}

	contr.loadFollowed()

	os.Setenv("GO-TWITCH_CLIENTID", contr.config.ClientID)
	return
}

func (c *Controller) AddStream(name string) error {
	_, present := c.tracked[name]
	if present {
		log.Println("Already Following")
		return errors.New("Already Following")
	}
	log.Println("Adding", name)
	user, _ := c.twitchAPI.Users.User(name)
	c.storage.follow.Insert(user)

	c.tracked[name] = true
	c.cChat <- Message{AddStream, name}
	c.cViewer <- Message{AddStream, name}
	c.cStat <- Message{AddStream, name}
	log.Println("Finished adding", name)
	return nil
}

func (c *Controller) RemoveStream(name string) {
	_, present := c.tracked[name]
	if !present {
		log.Println("Not Following")
		return
	}
	log.Println("Removing ", name)
	c.storage.follow.Remove(bson.M{"name": name})
	c.cChat <- Message{RemoveStream, name}
	c.cViewer <- Message{RemoveStream, name}
	c.cStat <- Message{RemoveStream, name}
	delete(c.tracked, name)
}

func (c *Controller) ListStreams() []string {
	keys := make([]string, 0, len(c.tracked))
	for k := range c.tracked {
		keys = append(keys, k)
	}
	return keys
}

func (c *Controller) loadFollowed() {
	var f []twitch.UserS
	c.storage.follow.Find(nil).All(&f)

	for _, v := range f {
		c.tracked[v.Name] = true
		go func(name string) {
			c.cChat <- Message{AddStream, name}
			c.cViewer <- Message{AddStream, name}
			c.cStat <- Message{AddStream, name}
		}(v.Name)
	}
}
