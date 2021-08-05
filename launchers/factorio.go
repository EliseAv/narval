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

func (server *FactorioServer) Prepare() error {
	worker := ParallelWorker{}
	worker.Add(server.prepareGetGame)
	worker.Add(server.prepareGetState)
	return worker.Join()
}

func (server *FactorioServer) prepareGetGame() error {
	var err error

	if _, err = os.Stat(factorioBinaryPath); !errors.Is(err, os.ErrNotExist) {
		return nil // Already have the game
	}

	reader := s3download("game.tar.xz")
	if reader == nil {
		err := server.prepareGetGameDownload()
		if err != nil {
			return err
		}
		reader, err = os.Open("/tmp/game.tar.xz")
		if err != nil {
			return err
		}
		defer func() { go server.prepareGetGameUpload() }()
	}

	// Un-xz (why the hell do they use xz!)
	decompressed, err := xz.NewReader(reader)
	if err != nil {
		return err
	}

	// Un-tar
	err = unTar(decompressed, "game")
	return err
}

func (*FactorioServer) prepareGetGameDownload() error {
	version := os.Getenv("FACTORIO_VERSION")
	if version == "" {
		version = "latest"
	}
	requestUrl := fmt.Sprintf("https://factorio.com/get-download/%s/headless/linux64", version)
	log.Printf("Downloading: %s", requestUrl)
	httpResponse, err := http.Get(requestUrl)
	if err != nil {
		return err
	}
	defer CloseDontCare(httpResponse.Body)

	file, err := os.Create("/tmp/game.tar.xz")
	if err != nil {
		return err
	}
	defer CloseDontCare(file)

	_, err = io.Copy(file, httpResponse.Body)
	return err
}

func (*FactorioServer) prepareGetGameUpload() {
	fileRead, err := os.Open("/tmp/game.tar.xz")
	if err != nil {
		return
	}

	err = s3upload("game.tar.xz", fileRead)
	if err != nil {
		return
	}

	_ = os.Remove("/tmp/game.tar.xz")
}

func (FactorioServer) prepareGetState() error {
	var worker ParallelWorker
	for name := range s3listRelevantObjects("state") {
		if _, err := os.Stat("game/" + name); !errors.Is(err, os.ErrNotExist) {
			continue // We already have the file
		}

		worker.Add(s3downloadJob{"state/" + name, "game/" + name}.Run)
	}
	return worker.Join()
}

func (FactorioServer) saveState() {
	// folderTargets := []string{"mods", "config"}
	// fileExtensionTargets := []string{"json", "dat", "zip"}
}

func (server *FactorioServer) Start() error {
	server.players = map[User]bool{}
	command := exec.Command(factorioBinaryPath, "--start-server", "game/save.zip")
	stdout, _ := command.StdoutPipe()
	server.in, _ = command.StdinPipe()
	err := command.Start()
	if err != nil {
		return err
	}
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
	return nil
}

func (server *FactorioServer) idleTimeout() {
	// When it comes to polling intervals, I prefer using prime numbers. This is just under 3 seconds.
	const interval time.Duration = 2718281831
	for time.Now().Before(server.shutdownAt) {
		time.Sleep(interval)
	}
	log.Printf("Shutting down!")
	err := server.SendCommand(ParsedLine{Event: EventStop})
	if err != nil {
		panic(err)
	}
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
		go server.saveState()
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

func (server FactorioServer) SendCommand(line ParsedLine) error {
	if line.Event == EventStop {
		_, err := server.in.Write([]byte("/quit\n"))
		return err
	}
	return errInvalidCommand
}
