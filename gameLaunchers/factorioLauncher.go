package gameLaunchers

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

type FactorioServer struct {
	Game       struct{ Version, Save string }
	Settings   ServerSettings
	players    map[User]bool
	shutdownAt time.Time
	maxSession time.Time
	out        chan ParsedLine
	in         io.WriteCloser
}

var factorioRegexpInGame = regexp.MustCompile(`^ *\d+\.\d{3} Info ServerMultiplayerManager\.cpp:.+ changing state .+ to\(InGame\)$`)
var factorioRegexpSaved = regexp.MustCompile(`^ *\d+\.\d{3} Info AppManagerStates\.cpp:\d+: Saving finished$`)
var factorioRegexpChatJoinLeave = regexp.MustCompile(`^.{19} \[([A-Z]+)] (.+)$`)
var factorioRegexpChat = regexp.MustCompile(`^(.+?): (.+)$`)
var factorioRegexpJoinLeave = regexp.MustCompile(`^(.+) (joined|left) the game$`)

func (server *FactorioServer) Prepare() {
	// Download
	requestUrl := fmt.Sprintf("https://factorio.com/get-download/%s/headless/linux64", server.Game.Version)
	log.Printf("Downloading: %s", requestUrl)
	httpResponse, err := http.Get(requestUrl)
	if err != nil {
		log.Panic(err)
	}
	defer httpResponse.Body.Close()

	// Un-xz
	decompressed, err := xz.NewReader(httpResponse.Body)
	if err != nil {
		log.Panic(err)
	}

	// Un-tar
	err = untar(decompressed)
	if err != nil {
		log.Panic(err)
	}
}

func (server *FactorioServer) Start() {
	server.players = map[User]bool{}
	command := exec.Command("factorio/bin/x64/factorio", "--start-server", server.Game.Save)
	stdout, _ := command.StdoutPipe()
	server.in, _ = command.StdinPipe()
	command.Start()
	server.out = make(chan ParsedLine, 100)
	server.shutdownAt = time.Now().Add(server.Settings.StartupGrace)
	server.maxSession = time.Now().Add(server.Settings.MaxSession)
	go server.readStdout(stdout)
	go server.idleTimeout()
	go stdinPassThrough(server.in)
}

func (server *FactorioServer) idleTimeout() {
	// When it comes to polling intervals, I prefer using prime numbers. This is just under 3 seconds.
	const interval time.Duration = 2718281831
	for time.Now().Before(server.shutdownAt) {
		time.Sleep(interval)
	}
	log.Printf("Shutting down!")
	server.SendCommand(ParsedLine{Event: EventStop})
}

func (server *FactorioServer) readStdout(stdout io.ReadCloser) {
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	for err == nil {
		fmt.Print(line)
		line = strings.TrimRight(line, "\n")
		server.out <- server.processLine(line)
		line, err = reader.ReadString('\n')
	}
	close(server.out)
}

func (server *FactorioServer) processLine(line string) (parsed ParsedLine) {
	parsed.Raw = line

	if factorioRegexpInGame.MatchString(line) {
		parsed.Event = EventReady
		return
	}

	if factorioRegexpSaved.MatchString(line) {
		parsed.Event = EventSaved
		return
	}

	matches := factorioRegexpChatJoinLeave.FindStringSubmatch(line)
	if matches == nil {
		return
	}
	event := matches[1]
	if event == "JOIN" {
		parsed.Event = EventJoin
		matches = factorioRegexpJoinLeave.FindStringSubmatch(matches[2])
		parsed.Author = User(matches[1])
		server.players[parsed.Author] = true
	} else if event == "LEAVE" {
		parsed.Event = EventLeave
		matches = factorioRegexpJoinLeave.FindStringSubmatch(matches[2])
		parsed.Author = User(matches[1])
		delete(server.players, parsed.Author)
		if len(server.players) == 0 {
			server.shutdownAt = time.Now().Add(server.Settings.ShutdownGrace)
		}
	} else {
		parsed.Event = EventTalk
		if event == "CHAT" {
			parsed.Message = matches[2]
			matches := factorioRegexpChat.FindStringSubmatch(matches[2])
			if matches != nil {
				parsed.Author = User(matches[1])
				parsed.Message = matches[2]
			}
		} else {
			parsed.Message = matches[0]
		}
	}
	if len(server.players) != 0 || server.shutdownAt.After(server.maxSession) {
		server.shutdownAt = server.maxSession
	}
	return
}

func (server FactorioServer) NumPlayers() int {
	return len(server.players)
}

func (server FactorioServer) GetLinesChannel() chan ParsedLine {
	return server.out
}

func (server FactorioServer) SendCommand(line ParsedLine) {
	if line.Event == EventStop {
		server.in.Write([]byte("/quit\n"))
	}
}
