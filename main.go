package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/namsral/flag"
	"github.com/ziutek/syslog"
)

var (
	config   = flag.String("config", "", "Config file to read")
	bind     = flag.String("bind", "0.0.0.0:1514", "Address used to bind the syslog server")
	slackURL = flag.String("slack_url", "", "URL of the Slack Incoming Webhook")
)

type handler struct {
	*syslog.BaseHandler
}

type notification struct {
	Text        string        `json:"text,omitempty"`
	Attachments []*attachment `json:"attachments"`
}

type attachment struct {
	Fallback string   `json:"fallback,omitempty"`
	Pretext  string   `json:"pretext,omitempty"`
	Color    string   `json:"color,omitempty"`
	Fields   []*field `json:"field,omitempty"`
}

type field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func (h *handler) mainLoop() {
	for {
		m := h.Get()
		if m == nil {
			break
		}

		tagsum := sha1.Sum([]byte(m.Tag))
		taghex := hex.EncodeToString(tagsum[:])

		msg, err := json.Marshal(&notification{
			Attachments: []*attachment{
				&attachment{
					Fallback: "[" + m.Tag + "] " + m.Content,
					Color:    "#" + taghex[:6],
					Fields: []*field{
						&field{
							Title: "tag",
							Value: m.Tag,
							Short: true,
						},
						&field{
							Title: "hostname",
							Value: m.Hostname,
							Short: true,
						},
						&field{
							Title: "time",
							Value: m.Timestamp.Format(time.RFC822Z),
							Short: false,
						},
						&field{
							Title: "content",
							Value: m.Content,
							Short: false,
						},
					},
				},
			},
		})
		if err != nil {
			log.Print(err)
		}

		res, err := http.Post(*slackURL, "application/json", bytes.NewReader(msg))
		if err != nil {
			log.Print(err)
		}
		res.Body.Close()
	}

	log.Print("Exit handler")
	h.End()
}

func filter(m *syslog.Message) bool {
	return true
}

func newHandler() *handler {
	h := &handler{syslog.NewBaseHandler(5, filter, false)}
	go h.mainLoop()
	return h
}

func main() {
	flag.Parse()

	s := syslog.NewServer()
	s.AddHandler(newHandler())
	s.Listen(*bind)

	log.Printf("Listening to %s", *bind)

	sc := make(chan os.Signal, 2)
	signal.Notify(sc, syscall.SIGTERM, syscall.SIGINT)
	<-sc

	log.Print("Shutdown the server...")
	s.Shutdown()
	log.Print("Server is down")
}
