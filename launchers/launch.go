package launchers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

type jsObj map[string]interface{}

var ipAddress string
var ipAddressKnown = make(chan struct{})

func Launch(what string) error {
	var server Server
	switch what {
	case "factorio":
		server = &FactorioServer{}
	default:
		return errors.New("Server not defined: " + what)
	}
	go fetchIpAddress()

	err := server.Prepare()
	if err != nil {
		return err
	}

	err = server.Start()
	if err != nil {
		return err
	}

	for line := range server.GetLinesChannel() {
		message := toMessage(server, line)
		if message != "" {
			sayInDiscord(message)
		}
	}
	sayInDiscord("Server shut down.")
	return nil
}

func toMessage(server Server, line ParsedLine) string {
	switch line.Event {
	case EventReady:
		<-ipAddressKnown
		return fmt.Sprintf("Server is ready! Ip address is %s", ipAddress)
	case EventJoin:
		return fmt.Sprintf("`[%2d]` :star2: %s", server.NumPlayers(), line.Author)
	case EventLeave:
		return fmt.Sprintf("`[%2d]` :comet: %s", server.NumPlayers(), line.Author)
	case EventTalk:
		return fmt.Sprintf("`<%s>` %s", line.Author, line.Message)
	}
	return ""
}

func sayInDiscord(message string) {
	body := jsObj{
		"content":          message,
		"allowed_mentions": jsObj{"parse": []string{}},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		log.Panic(err)
	}
	reader := bytes.NewReader(payload)
	hookUrl := os.Getenv("WEBHOOK_URL")
	_, err = http.Post(hookUrl, "application/json", reader)
	if err != nil {
		log.Print(err)
	}
}

func fetchIpAddress() {
	const getIpUrl = "http://169.254.169.254/latest/meta-data/public-ipv4"
	httpClient := http.Client{Timeout: 1 * time.Second}
	response, err := httpClient.Get(getIpUrl)
	if err == nil {
		buffer := make([]byte, 20)
		length, _ := response.Body.Read(buffer)
		ipAddress = string(buffer[:length])
		close(ipAddressKnown)
		return
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Panic(err)
	}
	for _, i := range interfaces {
		addresses, err := i.Addrs()
		if err != nil {
			log.Panic(err)
		}
		for _, addr := range addresses {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.To4() != nil && !ip.IsLoopback() {
				ipAddress = ip.String()
				close(ipAddressKnown)
				return
			}
		}
	}

	close(ipAddressKnown)
}
