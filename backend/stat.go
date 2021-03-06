package main

import (
	"github.com/grsakea/kappastat/common"
	"github.com/robfig/cron"
	"gopkg.in/mgo.v2"
	//"gopkg.in/mgo.v2/bson"
	"log"
	"strings"
	"time"
)

type statData struct {
	itC  *mgo.Iter
	lenC int
	itV  *mgo.Iter
	lenV int
}

func loopStat(ch chan Message, cBroad chan Message, db *mgo.Database) {
	followed := []string{}
	loop := true
	liveBroadcast := make(map[string]time.Time)
	c := cron.New()

	c.AddFunc("0 * * * * *", func() { computeStat(db, followed, 01*time.Minute) })
	c.AddFunc("0 */5 * * * *", func() { computeStat(db, followed, 05*time.Minute) })
	c.AddFunc("0 */15 * * * *", func() { computeStat(db, followed, 15*time.Minute) })
	c.AddFunc("@hourly", func() { computeStat(db, followed, time.Hour) })
	c.AddFunc("0 0 */12 * * *", func() { computeStat(db, followed, 12*time.Hour) })
	c.AddFunc("@daily", func() { computeStat(db, followed, 24*time.Hour) })

	c.Start()
	for loop {
		select {
		case msg := <-ch:
			followed, loop = followedHandler(followed, msg)
		case msg := <-cBroad:
			if msg.s == StartBroadcast {
				addBroadcast(liveBroadcast, msg.v)
			} else if msg.s == EndBroadcast {
				processBroadcast(db, liveBroadcast, msg.v)
			}
		}
	}
}

func computeStat(db *mgo.Database, channels []string, duration time.Duration) {
	to := time.Now()
	from := to.Add(-duration)

	for _, channel := range channels {
		data, err := fetchStatData(db, channel, from, to)
		if err == nil {
			se := processStatData(from, to, duration, channel, data)
			storeStatEntry(db.C("stat_entries"), se)
		}
	}
}

func addBroadcast(m map[string]time.Time, channel string) {
	log.Print(channel, " Started Broadcast")
	m[channel] = time.Now().Add(-time.Minute)
}

func processBroadcast(db *mgo.Database, m map[string]time.Time, channel string) {
	var v []kappastat.ViewerCount
	log.Print(channel, " Ended Broadcast duration : ", time.Now().Sub(m[channel]))

	ret := kappastat.Broadcast{
		Start: m[channel],
		End:   time.Now(),

		Channel: channel,
	}

	//b := bson.M{
	//"Channel":  channel,
	//"Duration": 1 * time.Minute,
	//"Start":    bson.M{"gt": m[channel]}}
	db.C("stat_entries").Find(nil).All(&v)
	log.Print("broadcast lasted ", len(v), " minutes ", m[channel])

	max := 0
	min := v[0].Viewer
	for _, i := range v {
		view := i.Viewer

		if view > max {
			max = view
		}
		if view < min {
			min = view
		}
		ret.AverageViewership += view
	}

	ret.AverageViewership /= len(v)
	ret.MinViewership = min
	ret.MaxViewership = max

	db.C("broadcasts").Insert(ret)

	log.Print(v)
}

func processStatData(from time.Time, to time.Time, duration time.Duration, channel string, data statData) (ret kappastat.StatEntry) {
	ret.Channel = channel
	ret.Duration = duration
	ret.Start = from
	ret.End = to

	var resultC kappastat.ChatEntry
	uniqueChatter := make(map[string]bool)
	termUsed := make(map[string]int)

	for data.itC.Next(&resultC) {
		if resultC.Sender == "twitchnotify" {
			if strings.Contains(resultC.Text, "just") {
				ret.Newsub += 1
			} else if strings.Contains(resultC.Text, "months") {
				ret.Resub += 1
			}
		} else {
			ret.Messages += 1

			for _, i := range strings.Split(resultC.Text, " ") {
				termUsed[i] += 1
			}

			_, present := uniqueChatter[resultC.Sender]
			if !present {
				uniqueChatter[resultC.Sender] = true
			}
		}
	}
	ret.UniqueChat = len(uniqueChatter)

	var result kappastat.ViewerCount
	ret.Viewer = 0
	nbZero := 0
	for data.itV.Next(&result) {
		ret.Viewer += result.Viewer
		if result.Viewer == 0 {
			nbZero--
		}
	}
	ret.Viewer /= data.lenV
	if nbZero > data.lenV {
		ret.NonZeroViewer /= (data.lenV - nbZero)
	} else {
		ret.NonZeroViewer = 0
	}
	return
}
