package launchers

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
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
	var waitGroup sync.WaitGroup
	go server.prepareGetGame(waitGroup.Done)
	go server.prepareGetSave(waitGroup.Done)
	go server.prepareGetConfig(waitGroup.Done)
	waitGroup.Add(3)
	waitGroup.Wait()
}

func (server *FactorioServer) prepareGetGame(done func()) {
	defer done()
	var err error

	if _, err = os.Stat(factorioBinaryPath); !errors.Is(err, os.ErrNotExist) {
		return // Already have the game
	}

	reader := s3download("game.tar.xz")
	if reader == nil {
		server.prepareGetGameDownload()
		reader, err = os.Open("/tmp/game.tar.xz")
		if err != nil {
			log.Panic(err)
		}
		defer server.prepareGetGameUpload()
	}

	// Un-xz (why the hell do they use xz!)
	decompressed, err := xz.NewReader(reader)
	if err != nil {
		log.Panic(err)
	}

	// Un-tar
	err = untar(decompressed, "game")
	if err != nil {
		log.Panic(err)
	}
}

func (*FactorioServer) prepareGetGameDownload() {
	version := os.Getenv("FACTORIO_VERSION")
	if version == "" {
		version = "latest"
	}
	// 64 MB... maybe its ok?
	requestUrl := fmt.Sprintf("https://factorio.com/get-download/%s/headless/linux64", version)
	log.Printf("Downloading: %s", requestUrl)
	httpResponse, err := http.Get(requestUrl)
	if err != nil {
		log.Panic(err)
	}
	defer httpResponse.Body.Close()

	file, err := os.Create("/tmp/game.tar.xz")
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	_, err = io.Copy(file, httpResponse.Body)
	if err != nil {
		log.Panic(err)
	}
}

func (*FactorioServer) prepareGetGameUpload() {
	fileRead, err := os.Open("/tmp/game.tar.xz")
	if err != nil {
		return
	}
	s3upload("game.tar.xz", fileRead)
	os.Remove("/tmp/game.tar.xz")
}

func (FactorioServer) prepareGetSave(done func()) {
	defer done()

	if _, err := os.Stat("game/save.zip"); !errors.Is(err, os.ErrNotExist) {
		return // Already have a save
	}

	reader := s3download("save.zip")
	if reader == nil {
		return // Save not found
	}

	file, err := os.Create("game/save.zip")
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		log.Panic(err)
	}
}

func (FactorioServer) prepareGetConfig(done func()) {
	defer done()

	if _, err := os.Stat("game/factorio/mods/mod-list.json"); !errors.Is(err, os.ErrNotExist) {
		return // Already have configuration
	}

	reader := s3download("config.tar.xz")
	if reader == nil {
		return
	}

	decompressed, err := xz.NewReader(reader)
	if err != nil {
		log.Panic(err)
	}

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

	startupGraceDuration, err := time.ParseDuration(os.Getenv("STARTUP_GRACE"))
	if err != nil {
		startupGraceDuration = 5 * time.Minute
	}
	server.shutdownAt = time.Now().Add(startupGraceDuration)

	maxSessionDuration, err := time.ParseDuration(os.Getenv("MAX_SESSION"))
	if err != nil {
		startupGraceDuration = 24 * time.Hour
	}
	server.maxSession = time.Now().Add(maxSessionDuration)

	server.shutdownGrace, err = time.ParseDuration(os.Getenv("SHUTDOWN_GRACE"))
	if err != nil {
		server.shutdownGrace = 1 * time.Minute
	}

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