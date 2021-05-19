package launchers

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

type FactorioServer struct {
	players       map[User]bool
	shutdownAt    time.Time
	maxSession    time.Time
	shutdownGrace time.Duration
	out           chan ParsedLine
	in            io.WriteCloser
}

const factorioBinaryPath = "game/factorio/bin/x64/factorio"

var factorioRegexpInGame = regexp.MustCompile(`^ *\d+\.\d{3} Info ServerMultiplayerManager\.cpp:.+ changing state .+ to\(InGame\)$`)
var factorioRegexpSaved = regexp.MustCompile(`^ *\d+\.\d{3} Info AppManagerStates\.cpp:\d+: Saving finished$`)
var factorioRegexpMainLog = regexp.MustCompile(`^.{19} \[([A-Z]+)] (.+)$`)
var factorioRegexpChat = regexp.MustCompile(`^(.+?): (.+)$`)
var factorioRegexpJoinLeave = regexp.MustCompile(`^(.+) (joined|left) the game$`)

func (server *FactorioServer) Prepare() {
	if _, err := os.Stat(factorioBinaryPath); !os.IsNotExist(err) {
		return // Already have the executable, skip download
	}

	// Download
	const downloadUrl = "https://factorio.com/get-download/%s/headless/linux64"
	requestUrl := fmt.Sprintf(downloadUrl, os.Getenv("FACTORIO_VERSION"))
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
	err = untar(decompressed, "game")
	if err != nil {
		log.Panic(err)
	}
}

func (server *FactorioServer) Start() {
	server.players = map[User]bool{}
	command := exec.Command(factorioBinaryPath, "--start-server", "game/save.zip")
	stdout, _ := command.StdoutPipe()
	server.in, _ = command.StdinPipe()
	command.Start()
	server.out = make(chan ParsedLine, 100)
	var maxSessionDuration, _ = time.ParseDuration(os.Getenv("MAX_SESSION"))
	var startupGraceDuration, _ = time.ParseDuration(os.Getenv("STARTUP_GRACE"))
	server.shutdownAt = time.Now().Add(startupGraceDuration)
	server.maxSession = time.Now().Add(maxSessionDuration)
	server.shutdownGrace, _ = time.ParseDuration(os.Getenv("SHUTDOWN_GRACE"))
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

	matches := factorioRegexpMainLog.FindStringSubmatch(line)
	if matches == nil {
		return
	}
	event := matches[1]
	line = matches[2]
	switch event {
	case "JOIN":
		parsed.Event = EventJoin
		matches = factorioRegexpJoinLeave.FindStringSubmatch(line)
		parsed.Author = User(matches[1])
		server.players[parsed.Author] = true
	case "LEAVE":
		parsed.Event = EventLeave
		matches = factorioRegexpJoinLeave.FindStringSubmatch(line)
		parsed.Author = User(matches[1])
		delete(server.players, parsed.Author)
		if len(server.players) == 0 {
			server.shutdownAt = time.Now().Add(server.shutdownGrace)
		}
	case "CHAT":
		parsed.Event = EventTalk
		parsed.Message = line
		matches := factorioRegexpChat.FindStringSubmatch(line)
		if matches != nil {
			parsed.Author = User(matches[1])
			parsed.Message = matches[2]
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
