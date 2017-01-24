package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type dto struct {
	Names map[string]int `json:"names"`
}

type scores struct {
	names map[string]int
	lock  sync.Mutex
}

func main() {
	log.Println("Starting demo score server...")

	scores := scores{
		names: make(map[string]int),
	}
	sendUpdate := make(chan struct{}, 3)

	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	//router.LoadHTMLFiles("templates/template1.html", "templates/template2.html")
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", nil)
	})

	router.GET("/heartbeat", func(c *gin.Context) {
		c.String(http.StatusOK, time.Now().String())
	})

	router.POST("/submit", func(c *gin.Context) {
		var body dto
		if err := c.Bind(&body); err != nil {
			log.Printf("Failed to bind POST body: %s\n", err)
			c.Status(http.StatusBadRequest)
			return
		}

		scores.lock.Lock()
		defer scores.lock.Unlock()

		log.Printf("Received: %+v\n", body)

		for k, v := range body.Names {
			scores.names[k] += v
		}

		sendUpdate <- struct{}{}
		c.Status(http.StatusOK)
	})

	scores.lock.Lock()
	scores.names["Tom"] = 5
	scores.names["Jim"] = 3
	scores.names["Mary"] = 0
	scores.lock.Unlock()

	router.GET("/events", func(c *gin.Context) {
		ping := time.NewTicker(15 * time.Second)
		updates := time.NewTicker(10 * time.Second) // Just for local testing
		immediate := time.After(time.Millisecond)

		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovering from panic %v\n", r)
			}

			ping.Stop()
			updates.Stop()
		}()

		c.Stream(func(w io.Writer) bool {
			select {
			case t := <-ping.C:
				c.SSEvent("ping", t.String())
			case <-sendUpdate:
				sendScores(c, scores.names)
			case <-immediate:
				sendScores(c, scores.names)
			case <-updates.C:
				sendScores(c, scores.names)
			}

			return true
		})
	})

	log.Println("Preparing to listen on 8080...")
	router.Run(":8080")
}

func sendScores(c *gin.Context, scores map[string]int) {
	msg, wasSuccessful := getScoresMessage(scores)
	if wasSuccessful {
		c.SSEvent("message", msg)
	}
}

func getScoresMessage(scores map[string]int) (string, bool) {
	str, err := json.Marshal(scores)
	if err != nil {
		return "", false
	}

	return string(str), true
}
